from __future__ import annotations

import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
import tcp  # noqa: E402
import udp  # noqa: E402


class TCPFrameParserTests(unittest.TestCase):
    def test_partial_then_complete_frame(self) -> None:
        buf = bytearray(b"\x01\x03\x00")
        self.assertEqual(tcp.parse_tcp_frames(buf), [])
        buf.extend(b"\x00\x00abc")
        frames = tcp.parse_tcp_frames(buf)
        self.assertEqual([(f.cmd, f.length, f.payload) for f in frames], [(1, 3, b"abc")])
        self.assertEqual(buf, b"")

    def test_multiple_frames_and_empty_payload(self) -> None:
        buf = bytearray(b"\x02\x00\x00\x00\x00\x07\x02\x00\x00\x00ok")
        frames = tcp.parse_tcp_frames(buf)
        self.assertEqual([(f.cmd, f.payload) for f in frames], [(2, b""), (7, b"ok")])

    def test_rejects_negative_or_excessive_length(self) -> None:
        with self.assertRaises(ValueError):
            tcp.parse_tcp_frames(bytearray(b"\x01\xff\xff\xff\xff"))
        with self.assertRaises(ValueError):
            tcp.parse_tcp_frames(bytearray(b"\x01\x11\x00\x00\x00"), max_frame_bytes=16)


class UDPHeaderParserTests(unittest.TestCase):
    def test_exact_mini_header_datagram(self) -> None:
        frame = udp.parse_udp_datagram(bytes([108, 0x7F, 1, 1, 0]))[0]
        self.assertEqual((frame.msg_key, frame.send_option, frame.cmd), (108, 1, 1))
        self.assertEqual((frame.length, frame.payload), (0, b""))

    def test_nonreliable_packet(self) -> None:
        packet = bytes([108, 0x2A, 0, 0x34, 0x12, 1, 3, 0]) + b"xyz"
        frames = udp.parse_udp_datagram(packet)
        self.assertEqual(len(frames), 1)
        frame = frames[0]
        self.assertEqual((frame.msg_key, frame.crc7, frame.send_option), (108, 0x2A, 0))
        self.assertEqual((frame.cmd, frame.flags, frame.length, frame.payload), (0x1234, 1, 3, b"xyz"))
        self.assertIsNone(frame.sequence_id)

    def test_reliable_packet(self) -> None:
        packet = bytes([108, 0, 2, 9, 0, 0x34, 0x12, 0x78, 0x56, 1, 2, 0]) + b"ok"
        frame = udp.parse_udp_datagram(packet)[0]
        self.assertEqual((frame.cmd, frame.sequence_id, frame.order_id), (9, 0x1234, 0x5678))
        self.assertEqual((frame.flags, frame.length, frame.payload), (1, 2, b"ok"))

    def test_rejects_unknown_key_and_truncated_payload(self) -> None:
        with self.assertRaises(ValueError):
            udp.parse_udp_datagram(b"\x00\x00\x00\x00\x00")
        with self.assertRaises(ValueError):
            udp.parse_udp_datagram(bytes([108, 0, 0, 1, 0, 0, 3, 0]) + b"x")


if __name__ == "__main__":
    unittest.main()
