#!/usr/bin/env python3
"""Compatibility entrypoint; the maintained UDP implementation is udp.py."""
from udp import MAX_DATAGRAM_BYTES, ThreadingUDPServer, UDPFrame, UDPHandler, main, parse_udp_datagram, utc_now, write_event

if __name__ == "__main__":
    main()
