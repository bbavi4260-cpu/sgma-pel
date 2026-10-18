#!/usr/bin/env python3
"""Bounded local Sigma notification-channel presence service.

This listener implements only the confirmed lobby-residency transport boundary:
client Cmd=1 opaque auth frame -> server bare 0x02 acknowledgement, then client
Cmd=2 heartbeat -> server bare 0x02 echo. It never decrypts, stores, or logs
tokens/payloads, and it never sends application, matchmaking, or game-node data.
"""
from __future__ import annotations

import argparse
import json
import socket
import socketserver
import threading
from datetime import datetime, timezone
from pathlib import Path


CMD_AUTH = 1
CMD_HEARTBEAT = 2
INIT_ACK = b"\x02"
MAX_FRAME_BYTES = 8192  # Matched Sigma TCP_MTU.
MAX_BUFFER_BYTES = MAX_FRAME_BYTES * 2 + 6
LOG_LOCK = threading.Lock()


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds")


def write_event(log_path: Path, event: str, **fields: object) -> None:
    row = {"timestamp": utc_now(), "event": event, **fields}
    with LOG_LOCK:
        with log_path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, separators=(",", ":")) + "\n")
        safe_fields = " ".join(
            f"{key}={value}" for key, value in fields.items()
        )
        print(f"{row['timestamp']} [NOTIFY] {event} {safe_fields}".rstrip(), flush=True)


class NotificationHandler(socketserver.BaseRequestHandler):
    def handle(self) -> None:
        log_path: Path = self.server.transport_log  # type: ignore[attr-defined]
        peer = f"{self.client_address[0]}:{self.client_address[1]}"
        confirmed = False
        pending = bytearray()
        self.request.settimeout(30)
        write_event(log_path, "connected", peer=peer)
        try:
            while True:
                chunk = self.request.recv(4096)
                if not chunk:
                    break
                pending.extend(chunk)
                if len(pending) > MAX_BUFFER_BYTES:
                    write_event(log_path, "closed_oversize_buffer", peer=peer, buffered=len(pending))
                    return

                offset = 0
                while len(pending) - offset >= 2:
                    command = pending[offset]
                    region = pending[offset + 1]
                    if command == CMD_HEARTBEAT:
                        self.request.sendall(INIT_ACK)
                        write_event(log_path, "heartbeat_ack", peer=peer, region=region)
                        offset += 2
                        continue

                    if len(pending) - offset < 6:
                        break
                    payload_length = int.from_bytes(pending[offset + 2:offset + 6], "big", signed=False)
                    if payload_length > MAX_FRAME_BYTES:
                        write_event(log_path, "closed_oversize_frame", peer=peer, command=command, region=region, length=payload_length)
                        return
                    frame_end = offset + 6 + payload_length
                    if len(pending) < frame_end:
                        break

                    if command == CMD_AUTH and not confirmed:
                        confirmed = True
                        self.request.sendall(INIT_ACK)
                        write_event(log_path, "auth_ack", peer=peer, region=region, encrypted_length=payload_length)
                    else:
                        write_event(log_path, "application_frame_ignored", peer=peer, command=command, region=region, encrypted_length=payload_length)
                    offset = frame_end

                if offset:
                    del pending[:offset]
        except (ConnectionError, OSError, socket.timeout) as error:
            write_event(log_path, "connection_error", peer=peer, confirmed=confirmed, error=type(error).__name__)
        finally:
            write_event(log_path, "disconnected", peer=peer, confirmed=confirmed)


class ThreadingNotificationServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main() -> None:
    parser = argparse.ArgumentParser(description="Bounded local Sigma notification-channel presence service")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", default=10301, type=int)
    parser.add_argument("--log-dir", required=True)
    args = parser.parse_args()

    log_dir = Path(args.log_dir).resolve()
    log_dir.mkdir(parents=True, exist_ok=True)
    transport_log = log_dir / "notification_transport.jsonl"
    with ThreadingNotificationServer((args.host, args.port), NotificationHandler) as server:
        server.transport_log = transport_log  # type: ignore[attr-defined]
        write_event(transport_log, "started", host=args.host, port=args.port, scope="opaque auth/heartbeat acknowledgements only")
        print(f"Sigma notification presence listener on {args.host}:{args.port}", flush=True)
        server.serve_forever()


if __name__ == "__main__":
    main()
