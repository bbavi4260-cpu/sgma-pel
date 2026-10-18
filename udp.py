#!/usr/bin/env python3
"""Sigma UDP packet diagnostic listener.

The decompiled UDPMsgPacket exposes MsgKey=108, reserved send options, and
header-size constants (mini=5, non-reliable=8, reliable=12). This parser uses
the declared field order and little-endian BinaryReader conventions to report
packet metadata. CRC7 validation, encryption/compression, reliable ACK tracking,
FastBinary message IDs and gameplay replies are intentionally not guessed.
"""
from __future__ import annotations

import argparse
import json
import os
import socketserver
import threading
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

LOG_LOCK = threading.Lock()
MAX_DATAGRAM_BYTES = 65507
UDP_MSG_KEY = 108
SEND_OPTIONS = {0: "NONE", 1: "HELLO", 2: "RELIABLE", 3: "BYEBYE", 4: "VAR", 5: "VAR_RELIABLE"}
RELIABLE_OPTIONS = {2, 5}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds")


@dataclass(frozen=True)
class UDPFrame:
    msg_key: int
    crc7: int
    send_option: int
    cmd: int
    sequence_id: int | None
    order_id: int | None
    flags: int
    length: int
    payload: bytes


def parse_udp_datagram(data: bytes) -> list[UDPFrame]:
    """Decode headers using the class constants; do not decrypt/interpret body."""
    if not data:
        raise ValueError("empty UDP datagram")
    frames: list[UDPFrame] = []
    pos = 0
    while pos < len(data):
        remaining = len(data) - pos
        if remaining < 5:
            raise ValueError(f"truncated UDP mini header: {remaining} bytes")
        key, crc7, option = data[pos], data[pos + 1], data[pos + 2]
        if key != UDP_MSG_KEY:
            raise ValueError(f"unexpected UDP MsgKey 0x{key:02x} (expected 0x{UDP_MSG_KEY:02x})")
        cmd = int.from_bytes(data[pos + 3:pos + 5], "little", signed=False)
        # A datagram consisting of exactly the documented mini header has no
        # length/flags fields. Its body is empty; longer payload-bearing mini
        # variants cannot be distinguished safely without Serialize's body.
        if remaining == 5:
            frames.append(UDPFrame(key, crc7, option, cmd, None, None, 0, 0, b""))
            pos += 5
            continue
        reliable = option in RELIABLE_OPTIONS
        header_size = 12 if reliable else 8
        if remaining < header_size:
            raise ValueError(f"truncated {('reliable' if reliable else 'non-reliable')} UDP header: {remaining} bytes")
        if reliable:
            sequence_id = int.from_bytes(data[pos + 5:pos + 7], "little")
            order_id = int.from_bytes(data[pos + 7:pos + 9], "little")
            flags = data[pos + 9]
            length = int.from_bytes(data[pos + 10:pos + 12], "little")
        else:
            sequence_id = None
            order_id = None
            flags = data[pos + 5]
            length = int.from_bytes(data[pos + 6:pos + 8], "little")
        end = pos + header_size + length
        if end > len(data):
            raise ValueError(f"UDP payload truncated: declared={length}, available={len(data) - pos - header_size}")
        frames.append(UDPFrame(key, crc7, option, cmd, sequence_id, order_id,
                               flags, length, data[pos + header_size:end]))
        pos = end
    return frames


def write_event(log_path: Path, event: str, **fields: object) -> None:
    row = {"timestamp": utc_now(), "event": event, **fields}
    with LOG_LOCK:
        with log_path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, separators=(",", ":")) + "\n")
    print(f"{row['timestamp']} [UDP] {event} " + " ".join(f"{k}={v}" for k, v in fields.items()), flush=True)


class UDPHandler(socketserver.BaseRequestHandler):
    def handle(self) -> None:
        packet, _sock = self.request
        log_path: Path = self.server.game_log  # type: ignore[attr-defined]
        peer = f"{self.client_address[0]}:{self.client_address[1]}"
        if len(packet) > MAX_DATAGRAM_BYTES:
            write_event(log_path, "oversize_datagram", peer=peer, bytes=len(packet))
            return
        try:
            frames = parse_udp_datagram(packet)
        except ValueError as exc:
            write_event(log_path, "unparsed_datagram", peer=peer, bytes=len(packet),
                        reason=str(exc), prefix_hex=packet[:24].hex(), response_sent=False)
            return
        for frame in frames:
            write_event(log_path, "packet_received", peer=peer, msg_key=frame.msg_key,
                        crc7=frame.crc7, send_option=frame.send_option,
                        send_option_name=SEND_OPTIONS.get(frame.send_option, "UNKNOWN"),
                        cmd=frame.cmd, sequence_id=frame.sequence_id, order_id=frame.order_id,
                        flags=frame.flags, payload_length=frame.length,
                        payload_prefix_hex=frame.payload[:12].hex(), response_sent=False)


class ThreadingUDPServer(socketserver.ThreadingUDPServer):
    allow_reuse_address = True
    daemon_threads = True


def main() -> None:
    parser = argparse.ArgumentParser(description="Sigma UDP packet-header diagnostic listener")
    parser.add_argument("--host", default=os.getenv("SIGMA_HOST", "0.0.0.0"))
    parser.add_argument("--port", default=int(os.getenv("SIGMA_UDP_GAME_NODE_PORT", "10200")), type=int)
    parser.add_argument("--log-dir", default="logs")
    args = parser.parse_args()
    log_dir = Path(args.log_dir).resolve()
    log_dir.mkdir(parents=True, exist_ok=True)
    with ThreadingUDPServer((args.host, args.port), UDPHandler) as server:
        server.game_log = log_dir / "udp_game_node.jsonl"  # type: ignore[attr-defined]
        write_event(server.game_log, "started", host=args.host, port=args.port,
                    msg_key=UDP_MSG_KEY, mini_header=5, nonreliable_header=8,
                    reliable_header=12, response_mode="diagnostic_only")
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            write_event(server.game_log, "stopping")


if __name__ == "__main__":
    main()
