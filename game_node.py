#!/usr/bin/env python3
"""Backward-compatible alias for the dedicated tcp.py listener."""
from tcp import MAX_CONNECTION_BYTES, TCPHandler, ThreadingTCPServer, main, utc_now, write_event

# Preserve the previous public class name for scripts that imported it.
GameNodeHandler = TCPHandler
ThreadingGameNodeServer = ThreadingTCPServer
MAX_FRAME_BYTES = MAX_CONNECTION_BYTES


if __name__ == "__main__":
    main()
