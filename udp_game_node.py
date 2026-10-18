#!/usr/bin/env python3
"""Backward-compatible alias for the dedicated udp2.py listener."""
from udp2 import MAX_DATAGRAM_BYTES, ThreadingUDPServer, UDPHandler, main, utc_now, write_event

# Preserve the previous public class name for scripts that imported it.
UDPGameNodeHandler = UDPHandler
ThreadingUDPGameNodeServer = ThreadingUDPServer


if __name__ == "__main__":
    main()
