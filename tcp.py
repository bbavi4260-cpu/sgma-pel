#!/usr/bin/env python3
"""Sigma TCP transport diagnostic listener.

The Assembly-CSharp declaration exposes a 5-byte header: Cmd (1 byte) and
Length (4 bytes). TCPMsgPacket.Unserialize accepts BinaryReader, whose standard
ReadInt32 is little-endian. The separate Region field exists in the object, but
the declared wire-header constant is only 5 bytes; this parser therefore does
not consume an extra Region byte. The decompiler omits Serialize/Unserialize
bodies and AES/session setup, so this listener parses/logs frames only and does
not fabricate protocol replies.
"""
from __future__ import annotations

import argparse
import json
import os
import socket
import socketserver
import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

HEADER_SIZE = 5
MAX_FRAME_BYTES = int(os.getenv("SIGMA_MAX_TCP_FRAME", str(8 * 1024 * 1024)))
RECV_BYTES = 64 * 1024
MAX_CONNECTION_BYTES = MAX_FRAME_BYTES + HEADER_SIZE
LOG_LOCK = threading.Lock()


def timestamp() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds")


@dataclass(frozen=True)
class TCPFrame:
    cmd: int
    length: int
    payload: bytes


def parse_tcp_frames(buffer: bytearray, max_frame_bytes: int = MAX_FRAME_BYTES) -> list[TCPFrame]:
    """Consume all complete 5-byte-header frames; retain any partial tail."""
    frames: list[TCPFrame] = []
    while len(buffer) >= HEADER_SIZE:
        cmd = buffer[0]
        length = int.from_bytes(buffer[1:5], "little", signed=True)
        if length < 0:
            raise ValueError(f"negative TCP payload length {length}")
        if length > max_frame_bytes:
            raise ValueError(f"TCP payload length {length} exceeds limit {max_frame_bytes}")
        total = HEADER_SIZE + length
        if len(buffer) < total:
            break
        payload = bytes(buffer[HEADER_SIZE:total])
        del buffer[:total]
        frames.append(TCPFrame(cmd=cmd, length=length, payload=payload))
    return frames


def write_event(log_path: Path, event: str, **fields: object) -> None:
    row = {"timestamp": timestamp(), "event": event, **fields}
    with LOG_LOCK:
        with log_path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, separators=(",", ":")) + "\n")
    print(f"{row['timestamp']} [TCP] {event} " + " ".join(f"{k}={v}" for k, v in fields.items()), flush=True)


def handle_tcp_client(conn: socket.socket, addr, log_path: Path) -> None:
    peer = f"{addr[0]}:{addr[1]}"
    write_event(log_path, "connected", peer=peer)
    buffer = bytearray()
    try:
        with conn:
            conn.settimeout(60)
            while True:
                data = conn.recv(RECV_BYTES)
                if not data:
                    if buffer:
                        write_event(log_path, "incomplete_frame_on_close", peer=peer,
                                    buffered=len(buffer), prefix_hex=bytes(buffer[:16]).hex())
                    break
                buffer.extend(data)
                if len(buffer) > MAX_FRAME_BYTES + HEADER_SIZE:
                    raise ValueError(f"receive buffer exceeded {MAX_FRAME_BYTES + HEADER_SIZE} bytes")
                for frame in parse_tcp_frames(buffer):
                    # Never dump the full body: login/session payloads may contain credentials.
                    write_event(log_path, "frame_received", peer=peer, cmd=frame.cmd,
                                length=frame.length, payload_prefix_hex=frame.payload[:12].hex(),
                                response_sent=False)
    except (OSError, TimeoutError, ValueError) as exc:
        write_event(log_path, "client_error", peer=peer, error=str(exc))
    finally:
        write_event(log_path, "closed", peer=peer)


class TCPHandler(socketserver.BaseRequestHandler):
    """Compatibility handler for callers that used the prior socketserver API."""
    def handle(self) -> None:
        handle_tcp_client(self.request, self.client_address, self.server.game_log)  # type: ignore[attr-defined]


class ThreadingTCPServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


utc_now = timestamp


def main() -> None:
    parser = argparse.ArgumentParser(description="Sigma TCP framed-packet diagnostic listener")
    parser.add_argument("--host", default=os.getenv("SIGMA_HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=int(os.getenv("SIGMA_GAME_NODE_PORT", "10300")))
    parser.add_argument("--log-dir", default="logs")
    args = parser.parse_args()
    log_dir = Path(args.log_dir).resolve()
    log_dir.mkdir(parents=True, exist_ok=True)
    log_path = log_dir / "tcp_packets.jsonl"

    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server:
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind((args.host, args.port))
        server.listen(64)
        server.settimeout(1.0)
        write_event(log_path, "started", host=args.host, port=args.port,
                    framing="cmd:u8,length:i32-le,payload:length", header_bytes=HEADER_SIZE,
                    response_mode="diagnostic_only")
        try:
            while True:
                try:
                    conn, addr = server.accept()
                except socket.timeout:
                    continue
                threading.Thread(target=handle_tcp_client, args=(conn, addr, log_path),
                                 daemon=True, name=f"TCP-{addr[0]}:{addr[1]}").start()
        except KeyboardInterrupt:
            write_event(log_path, "stopping")


if __name__ == "__main__":
    main()
