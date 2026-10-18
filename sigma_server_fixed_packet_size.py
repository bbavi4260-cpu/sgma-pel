import json
import logging
import os
import socket
import sqlite3
import struct
import threading
import time
from dataclasses import dataclass
from typing import Optional, Tuple

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(threadName)s: %(message)s")

# Set SIGMA_HOST=0.0.0.0 to accept connections from other machines.
HOST = os.getenv("SIGMA_HOST", "127.0.0.1")
TCP_PORT = int(os.getenv("SIGMA_TCP_PORT", "10300"))
UDP_PORT = int(os.getenv("SIGMA_UDP_PORT", "10200"))
DB_FILE = os.getenv("SIGMA_DB_FILE", "sigma_server.db")
# The client is sending a framed payload larger than 1 MiB (observed: 1,264,719 bytes).
# Keep a safety ceiling, but allow the larger game-info packet to arrive completely.
MAX_PACKET_SIZE = 8 * 1024 * 1024
DEFAULT_USER_ID = "100000001"
DEFAULT_USERNAME = "LOCAL_SIGMA_TEST"

MAP_ID = 101
MAP_NAME = "PARADISE"
GAME_MODE = "BATTLEROYALE"
MODEL_TYPE = "ModelD"
CMD_USER_LOGIN = 2
MAGIC = 0x5347
HEADER = struct.Struct("!HHI")


class DatabaseManager:
    def __init__(self, db_path: str = DB_FILE):
        self.db_path = db_path
        with self.connect() as conn:
            conn.execute("""CREATE TABLE IF NOT EXISTS accounts (
                user_id TEXT PRIMARY KEY, username TEXT NOT NULL,
                level INTEGER DEFAULT 100, coins INTEGER DEFAULT 999999,
                diamonds INTEGER DEFAULT 5000
            )""")
            # Preserve an existing test account rather than replacing user data at every startup.
            conn.execute("""INSERT OR IGNORE INTO accounts
                (user_id, username, level, coins, diamonds) VALUES (?, ?, 100, 999999, 5000)""",
                (DEFAULT_USER_ID, DEFAULT_USERNAME))
        logging.info("[Database] SQLite DB initialized (%s)", self.db_path)

    def connect(self):
        return sqlite3.connect(self.db_path, timeout=10)

    def fetch_user(self, user_id: str) -> Optional[dict]:
        with self.connect() as conn:
            row = conn.execute(
                "SELECT user_id, username, level, coins, diamonds FROM accounts WHERE user_id=?",
                (user_id,),
            ).fetchone()
        if not row:
            return None
        return dict(user_id=row[0], username=row[1], level=row[2], coins=row[3], diamonds=row[4])


@dataclass
class PlayerSession:
    user_id: str
    username: str
    conn: socket.socket
    addr: Tuple[str, int]


class PacketHandler:
    @staticmethod
    def pack(cmd_id: int, payload: dict) -> bytes:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        if len(body) > MAX_PACKET_SIZE:
            raise ValueError("response exceeds maximum packet size")
        return HEADER.pack(MAGIC, cmd_id, len(body)) + body

    @staticmethod
    def extract(buffer: bytearray):
        """Extract complete framed packets; leave incomplete bytes buffered."""
        packets = []
        while len(buffer) >= HEADER.size:
            magic, cmd_id, length = HEADER.unpack(buffer[:HEADER.size])
            if magic != MAGIC:
                raise ValueError(f"invalid packet magic 0x{magic:04x}")
            if length > MAX_PACKET_SIZE:
                raise ValueError(f"packet too large: {length}")
            total = HEADER.size + length
            if len(buffer) < total:
                break
            body = bytes(buffer[HEADER.size:total])
            del buffer[:total]
            packets.append((cmd_id, body))
        return packets


class TCPServer:
    def __init__(self, host: str, port: int, db: DatabaseManager):
        self.host, self.port, self.db = host, port, db

    def start(self):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server:
            server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            server.bind((self.host, self.port))
            server.listen(32)
            logging.info("[TCP Server] Listening on %s:%s", self.host, self.port)
            while True:
                conn, addr = server.accept()
                threading.Thread(target=self.handle, args=(conn, addr), daemon=True).start()

    def handle(self, conn: socket.socket, addr: Tuple[str, int]):
        logging.info("[TCP] Client connected: %s", addr)
        buffer = bytearray()
        try:
            with conn:
                conn.settimeout(30)
                while True:
                    data = conn.recv(64 * 1024)
                    if not data:
                        if buffer:
                            logging.warning("[TCP] Client closed with %d incomplete bytes from %s", len(buffer), addr)
                        break
                    buffer.extend(data)
                    for cmd_id, raw_body in PacketHandler.extract(buffer):
                        try:
                            request = json.loads(raw_body.decode("utf-8"))
                        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                            logging.warning("[TCP] Invalid JSON request from %s: %s", addr, exc)
                            continue
                        logging.info("[TCP] Received framed command %d from %s (payload=%d bytes)", cmd_id, addr, len(raw_body))
                        # This sample implementation only defines login response behavior.
                        # The game client's actual command IDs/schema must match this framing.
                        if cmd_id != CMD_USER_LOGIN:
                            response = {"ret": 1, "status": 1, "error_code": 1, "message": "unsupported command"}
                        else:
                            requested_id = str(request.get("user_id", DEFAULT_USER_ID))
                            user = self.db.fetch_user(requested_id)
                            if user is None:
                                response = {"ret": 1, "status": 1, "error_code": 1, "message": "unknown user"}
                            else:
                                response = {
                                    "ret": 0, "status": 0, "error_code": 0,
                                    "user_id": int(user["user_id"]), "nickname": user["username"],
                                    "level": user["level"], "coins": user["coins"],
                                    "diamonds": user["diamonds"], "exp": 100000,
                                    "map_id": MAP_ID, "map_name": MAP_NAME,
                                    "game_mode": GAME_MODE, "model_type": MODEL_TYPE,
                                    "sys_time": int(time.time()),
                                }
                        # Exactly one response format: framed packet. Do not append raw ASCII
                        # to a binary protocol stream; it corrupts subsequent framing.
                        conn.sendall(PacketHandler.pack(cmd_id, response))
        except (OSError, ValueError) as exc:
            logging.warning("[TCP] Client %s disconnected/error: %s", addr, exc)
        finally:
            logging.info("[TCP] Connection closed: %s", addr)


class UDPServer:
    def __init__(self, host: str, port: int):
        self.host, self.port = host, port

    def start(self):
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.bind((self.host, self.port))
            logging.info("[UDP Server] Listening on %s:%s", self.host, self.port)
            while True:
                try:
                    data, addr = sock.recvfrom(2048)
                    # Placeholder only. A real game client may require a defined UDP packet format.
                    if data == b"PING":
                        sock.sendto(b"PONG", addr)
                except OSError as exc:
                    logging.error("[UDP] %s", exc)


def main():
    db = DatabaseManager()
    tcp = TCPServer(HOST, TCP_PORT, db)
    udp = UDPServer(HOST, UDP_PORT)
    threading.Thread(target=tcp.start, daemon=True, name="TCP-Thread").start()
    threading.Thread(target=udp.start, daemon=True, name="UDP-Thread").start()
    logging.info("[Server Core] Services started")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        logging.info("[Server Core] Shutting down")


if __name__ == "__main__":
    main()
