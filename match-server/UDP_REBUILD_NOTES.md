# UDP Match Server rebuild notes

## Protocol facts visible in `sigma.cs`

- `UDPMsgPacket` defines msg key `108` (`0x6c`), send options `0..5`, reliable header size `12`, non-reliable header size `8`, and flags encryption=`1`, ZIP=`2`.
- `UDP_Ack_Res` contains `Ack: ushort` and `AckBits: uint` (6 bytes, little-endian under the fixed-width path).
- `S2C_Hello_Res` fields are `SessionKey`, `OrderID`, `RequiredID`, and `EnableFastProto`.
- `NetworkCryptologyUtil` exposes TEA encrypt/decrypt and the decompiled dump lists salt and trailing-zero constants; the dump does not contain method bodies, so it cannot alone prove every crypto implementation detail.

## Changes in this source rebuild

1. UDP ACK handling now reads the full 6-byte ACK + ACK-bitmap payload and clears all acknowledged pending reliable packets, including the previous 32 sequence IDs.
2. Inbound reliable packets are tracked in a 32-packet receive window. Duplicate/stale application packets are ACKed but not dispatched twice; duplicate HELLO/reconnect requests are allowed to receive a fresh handshake response.
3. Packet decoding rejects datagrams whose declared payload length does not exactly consume the datagram.
4. The UDP TEA key is no longer silently defaulted to a placeholder. Set `MATCH_UDP_SECRET_HEX` to the exact 16-byte key shared with the matchmaker. `MATCH_ALLOW_DEV_KEY=1` enables the placeholder only for local protocol tests.
5. Added tests for ACK bitmap semantics, uint16 sequence wrap, duplicate receive suppression, and trailing bytes.

## Compatibility limitations

- The provided dump is Free Fire 1.106; source comments and existing match implementation describe a different client dialect in places. Command IDs and fields must be validated against a packet capture from the exact target client before claiming full compatibility.
- ZIP-flag payload decoding remains intentionally unsupported; the server returns a decode error rather than guessing the compression/encryption order.
- Full integration tests require Go 1.24+ and the module dependencies. In this offline build environment only the dependency-free packet/crypto/message/token packages and an isolated writer/reliability test harness could be run.
