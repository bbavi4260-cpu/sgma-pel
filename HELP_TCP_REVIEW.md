# `help-tcp.zip` protocol review

## Useful TCP definitions

The archive contains text extracts of client declarations with protobuf field tags. `matchmaking.txt` describes `MatchmakingStartNtf` fields (`map_ids` packed at 1, `game_mode` at 2, `difficulty` at 3, `match_mode` at 4, wait time at 5, group mode at 6) and `MatchmakingSussNtf` handoff fields (`match_id` at 1, `server_addr` at 2, `secret` at 3, `prepare_token` at 4, `sleep_ms` at 5, `map_id` at 6, modes at 7–8, cache flag at 9, plus optional fields). Group and room extracts also provide request/response field tags. These make the expected message shapes clearer.

The TCP session excerpt identifies `TCPSession2`, an AES object, access/session state, `Send(cmd, data, byteCount, region)`, and receive/dispatch methods. But `Encrypt`, `CreateAes`, packet extraction, receive parsing, and message-dispatch bodies are not included; the notes show signatures only. The `packet_summary.txt` file is not a packet capture. The ZIP contains no `.pcap`, `.pcapng`, `.json`, or binary packet payload files, and its listed decoded lengths/command values are inconsistent with several reported packet sizes. Those summaries are not safe evidence for inventing the frame header or command offsets.

## Useful UDP declarations

The file named `udp_handshake.txt` is a large **decompiled C# source dump**, not a recorded UDP handshake. It identifies `C2S_UDP_Ping_Req` with send-tick/ping statistics, `S2C_UDP_Ping_Res` with a send tick, and `S2C_C2S_RUDP_SPLITTED_DATAGRAM` with fields for ID, command, fragment total/number, send option, payload length, and payload. It also shows `UDPClientMessageBase.USER_MESSAGE_ID_START = 100`.

The same dump has 1,464 `Serialize(FastBinaryWriter ...)` declarations and all 1,464 bodies are empty stubs; the ping `UnSerialize` methods are stubs too. It does not provide a usable FastBinary reader/writer implementation, the complete transformed datagram byte layout, a mapping from message IDs to concrete message types, or a real UDP packet capture. Class names and fields alone are not enough to construct a valid ping reply or gameplay datagram.

## What changed in V38

The project now has dedicated runnable `tcp.py` and `udp2.py` entrypoints (`udp2.py` delegates to `udp.py`). TCP defaults to `10300`; UDP defaults to `10200`. The notification TCP presence listener defaults to `10301` so it does not conflict with the game TCP port. All ports are configurable. TCP parses the declaration's 5-byte header as command byte + little-endian signed 32-bit length, including split/coalesced reads; it does not consume a Region byte because that conflicts with the declared 5-byte size. UDP parses the exposed MsgKey/send-option/cmd fields and the declared mini/non-reliable/reliable header sizes (5/8/12 bytes). These are framing diagnostics only. The code intentionally does not fabricate AES/session responses, validate the unknown CRC7 algorithm, decrypt/zip UDP bodies, send ping/ACK responses, or claim matchmaking/gameplay support.

To implement real client traffic, the missing inputs are the actual `TCPSession2` encryption/confirmation method bodies (the header constants are known, but packet methods are stubs), the UDP CRC/transforms and message-ID registry, reliable ACK/retransmit rules, and a sanitized matching-version client/server trace that excludes passwords, auth/session tokens, and matchmaking secrets. Parser unit tests establish only byte-boundary handling for the declared header shapes; they do not prove full game compatibility.
