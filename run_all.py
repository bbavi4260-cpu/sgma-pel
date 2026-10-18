#!/usr/bin/env python3
"""Run every match-related service of the project from one command.

    python runall.py

Starts, in order:
  1. test_server.py            HTTP + SQLite      (port $PORT, default 3000)
  2. Go UDP match server       (port 10200)       - built automatically if missing
  3. notification_gateway.py   presence listener  (port 10301)
  4. transport/tcp_matchmaking.js  TCP matchmaking (port 10300)

If any service exits, all the others are stopped and runall.py exits with code 1
(so Render / Docker / systemd can restart it). Ctrl-C stops everything.
"""
import os
import shutil
import signal
import sqlite3
import subprocess
import sys
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
os.chdir(ROOT)

HOST = "0.0.0.0"
HTTP_PORT = os.environ.get("PORT") or os.environ.get("SIGMA_TEST_PORT", "3000")
NOTIFICATION_PORT = os.environ.get("SIGMA_NOTIFICATION_PORT", "10301")
GAME_NODE_PORT = os.environ.get("SIGMA_GAME_NODE_PORT", "10300")
UDP_PORT = os.environ.get("SIGMA_UDP_GAME_NODE_PORT", "10200")
DB_PATH = ROOT / "data" / "sigma.db"
GO_BIN = ROOT / "logs" / ("sigma_match_server.exe" if os.name == "nt" else "sigma_match_server")

match_host = (
    os.environ.get("SIGMA_MATCH_HOST")
    or os.environ.get("SIGMA_ADVERTISED_HOST")
    or os.environ.get("RENDER_EXTERNAL_HOSTNAME")
    or "127.0.0.1"
)

ENV = os.environ.copy()
defaults = {
    "SIGMA_MATCH_HOST": match_host,
    "SIGMA_GAME_NODE": f"{match_host}:{GAME_NODE_PORT}",
    "SIGMA_NOTIFICATION_CHANNEL": f"{match_host}:{GAME_NODE_PORT}",
    "MATCH_JWT_SECRET": "dev-match-secret-change-me",
    "MATCH_UDP_SECRET_HEX": "00112233445566778899aabbccddeeff",
    "MATCH_BOT": "1",
    "MATCH_SEND_110": "0",
    "MATCH_DEBUG_POS": "0",
    "MATCH_NO_ZONE": "1",
    "MATCH_BOT_COUNT": "5",
    "MATCH_BOT_ALLIES": "0",
    "MATCH_BOT_ENEMIES": "0",
}
for key, value in defaults.items():
    ENV.setdefault(key, value)

(ROOT / "logs").mkdir(exist_ok=True)
(ROOT / "data").mkdir(exist_ok=True)
(ROOT / "resources").mkdir(exist_ok=True)

procs = []  # (name, Popen)


def log(msg):
    print(f"[runall] {msg}", flush=True)


def require(tool, hint):
    if shutil.which(tool) is None:
        log(f"'{tool}' not found. {hint}")
        sys.exit(1)


def pump(name, proc):
    """Forward a service's output to stdout with a name prefix."""
    for line in iter(proc.stdout.readline, b""):
        sys.stdout.write(f"[{name}] {line.decode('utf-8', 'replace')}")
        sys.stdout.flush()


def start(name, cmd):
    log(f"starting {name}: {' '.join(cmd)}")
    proc = subprocess.Popen(
        cmd, env=ENV, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    )
    threading.Thread(target=pump, args=(name, proc), daemon=True).start()
    procs.append((name, proc))
    return proc


def stop_all():
    for name, proc in procs:
        if proc.poll() is None:
            proc.terminate()
    deadline = time.time() + 5
    for name, proc in procs:
        try:
            proc.wait(timeout=max(0.1, deadline - time.time()))
        except subprocess.TimeoutExpired:
            proc.kill()


def go_binary_runs():
    """True if the prebuilt Go binary can execute on this machine (right OS/CPU)."""
    if not GO_BIN.exists():
        return False
    try:
        subprocess.run([str(GO_BIN), "-h"], cwd=ROOT, env=ENV, timeout=5,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return True
    except subprocess.TimeoutExpired:
        return True  # it started and kept running, so the format is fine
    except OSError as exc:  # e.g. [Errno 8] Exec format error (binary built for ARM/Termux)
        log(f"existing Go binary cannot run here: {exc}")
        return False


def db_ready():
    try:
        conn = sqlite3.connect(DB_PATH)
        conn.execute("SELECT state_json FROM account_state LIMIT 1").fetchone()
        conn.close()
        return True
    except Exception:
        return False


def main():
    node_available = shutil.which("node") is not None
    if not node_available:
        log("Node.js not found: TCP matchmaking will be SKIPPED (install Node.js, or use the Docker deploy).")
    udp_available = True
    if not go_binary_runs():
        if shutil.which("go") is None:
            log("Go is not installed and no compatible match-server binary exists.")
            log("UDP match server will be SKIPPED. Use the Docker deploy (it installs Go) for matches.")
            udp_available = False
        else:
            log("building Go match server for this machine (first build can take a while)...")
            GO_BIN.unlink(missing_ok=True)
            build = subprocess.run(
                ["go", "build", "-o", str(GO_BIN), "."], cwd=ROOT / "match-server"
            )
            if build.returncode != 0:
                log("Go build failed; see error above. UDP match server will be SKIPPED.")
                udp_available = False

    py = sys.executable
    # 1) HTTP + SQLite first: it owns the account/session state.
    start("http", [py, "-u", "test_server.py", "--host", HOST, "--port", str(HTTP_PORT), "--root", str(ROOT)])
    for _ in range(40):
        if db_ready():
            break
        time.sleep(0.25)
    else:
        log("SQLite account state did not initialise in time (continuing anyway).")

    # 2) Go UDP match server
    if udp_available:
        start("udp-match", [str(GO_BIN), "-addr", f":{UDP_PORT}"])
    # 3) Notification / presence gateway
    start("notify", [py, "-u", "notification_gateway.py", "--host", HOST,
                     "--port", str(NOTIFICATION_PORT), "--log-dir", str(ROOT / "logs")])
    # 4) Node TCP matchmaking
    if node_available:
        start("tcp-match", ["node", "transport/tcp_matchmaking.js", "--host", HOST,
                        "--port", str(GAME_NODE_PORT), "--udp-port", str(UDP_PORT),
                        "--state", str(ROOT / "data" / "account_state.json"),
                        "--db", str(DB_PATH)])

    log(f"HTTP :{HTTP_PORT} | notify :{NOTIFICATION_PORT} | TCP match :{GAME_NODE_PORT} | UDP match :{UDP_PORT}")
    log("all services started. Ctrl-C to stop.")

    def handle_signal(signum, frame):
        log("stopping all services...")
        stop_all()
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    # Watchdog: if any service dies, shut everything down.
    while True:
        for name, proc in procs:
            code = proc.poll()
            if code is not None:
                log(f"service '{name}' exited with code {code}; stopping the rest.")
                stop_all()
                sys.exit(1)
        time.sleep(1)


if __name__ == "__main__":
    main()
