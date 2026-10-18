# UDP Match Server — protocol audit and validation status

## Scope

This package is a source-level repair of the supplied match-server project, focused on UDP/RUDP framing, acknowledgements, duplicate reliable packets, retransmission, and startup key validation. It is **not a proof of complete game compatibility**.

## Facts checked against `sigma.cs` (Free Fire 1.106 dump)

- `UDPMsgPacket.UDPMESSAGE_MSGKEY = 108` (`0x6c`).
- `UDPMsgPacket` declares non-reliable and reliable header sizes of 8 and 12 bytes.
- Send options are 0 through 5; encryption and ZIP flag bits are 1 and 2.
- `UDP_Ack_Res` contains `ushort Ack` and `uint AckBits`.
- `S2C_Hello_Res` contains `SessionKey`, `OrderID`, `RequiredID`, and `EnableFastProto`.

The supplied dump is a decompilation: method bodies are empty, so it does not prove all serializer details, TEA padding/salt behavior, the live `SECRET_KEY`, ZIP ordering, or every command payload layout.

## Source changes in this package

- Parse the six-byte ACK body (`Ack` + `AckBits`) and remove acknowledged reliable packets from the retransmit queue.
- Track the newest inbound reliable sequence plus a 32-packet bitmap; suppress duplicate application dispatch while still answering repeated HELLO/reconnect requests.
- Reject datagrams with trailing bytes or declared payload lengths that do not match the datagram.
- Require `MATCH_UDP_SECRET_HEX` for normal startup; the built-in placeholder is only permitted with explicit `MATCH_ALLOW_DEV_KEY=1`.
- Add focused tests for packet framing, CRC rejection, ACK bitmap/wrap behavior, duplicate receive handling, and retransmission.

## Validation actually completed in the available build environment

- `go test ./packet ./crypto ./message ./token`: PASS using the installed Go 1.23 toolchain with a temporary local `go.mod` directive adjustment; the delivered `go.mod` remains unchanged.
- Isolated RUDP/packet test harness: `go test -count=1 ./...`: PASS.
- Isolated RUDP/packet test harness: `go test -race -count=1 .`: PASS.
- Full `go test ./...`, full `go vet ./...`, and final target build: NOT RUN successfully. The source declares Go 1.24 and needs external modules (`go-redis` and `protobuf`) that were unavailable in the offline environment.
- No live game packet capture or in-game end-to-end session was available, so game compatibility is **not yet verified**.

## Required final checks on Termux

1. Use Go 1.24 or newer and install module dependencies (`go mod download`).
2. Set the exact 16-byte key shared with the matchmaker in `MATCH_UDP_SECRET_HEX`; do not use the development key for a real client.
3. Run `./verify-termux.sh` and retain the complete output.
4. Test with the exact target game version and capture client/server UDP packets. Verify HELLO, ACKs, reconnect, join, spawn, movement, damage, and disconnect before calling it compatible.

## Important limitation

A source dump alone cannot establish that the live secret key or all packet crypto details are correct. Do not label this build “100% fixed” until the full build, automated suite, and target-client integration test all pass.

## Follow-up source audit (2026-10-16)

The supplied `sigma.cs` declares the UDP packet constants used here: message key `108`, reliable header size `12`, non-reliable header size `8`, and send-option/flag values. The decompiled method bodies are empty, so this confirms declarations but not the full live serializer or encryption behavior.

A gameplay-flow issue was found in the CS shop path and corrected in `match.go`:

- Previously, cmd 407 (shop catalogue) was sent during `phSetupWait`, before the client enters the active Prepare/buy phase. The client-side handler is documented in this source as accepting the shop only while the match is Running, so that early packet can be ignored.
- Previously, the setup path sent the catalogue only to the first session and did not re-send it after later round transitions.
- The catalogue is now sent to each human after entering `phPrepare`, including subsequent buy phases. The `MATCH_HOLD_PREPARE=1` diagnostic path also sends it after entering Prepare. BR is explicitly excluded from this CS-only shop path.

Validation performed after this change:

- `gofmt` applied to `match.go`.
- `GOTOOLCHAIN=local go test ./packet ./crypto ./message ./token`: PASS using installed Go 1.23 after temporarily lowering only the local test copy's `go.mod` directive; the delivered `go.mod` remains at Go 1.24.
- Full root-package tests/build remain NOT RUN successfully here: Go 1.24 and external dependencies are unavailable in the offline environment.
- The shop flow fix has not yet been verified in a live client. It is a source-level correction, not a claim of zero errors or complete compatibility.

No real `MATCH_UDP_SECRET_HEX` was recovered from the dump. Do not replace the shared live key with the placeholder; `sigma.cs` field declarations alone cannot prove the live key, TEA salt/padding details, or all payload layouts.


## Target-version gameplay command-ID comparison (sigma.cs, Free Fire 1.106)

A direct comparison of the `message.OPNCEEBBMJH` enum and the server's constants found a second, separate compatibility issue: several Contra Squad/revive command IDs were from an older/different mapping, even though the payload classes were named correctly. Updated the constants and code comments to the IDs declared by the supplied 1.106 dump:

- `RUDP_NOTIFYREVIVE`: **394**, not 388. In the supplied enum, 388 is `RUDP_PLAYER_INROOM`.
- `RUDP_SYNC_CSSHOPITEMS_LIST`: **413**, not 407. In the supplied enum, 407 is `RUDP_DISMISS_HOTZONES`.
- `RUDP_PURCHASE_CSITEMS`: **414**, not 408. In the supplied enum, 408 is `RUDP_SHOW_MATCH_RESULT`.
- `RUDP_CS_CURROUND_RESULT`: **415**, not 409. In the supplied enum, 409 is `RUDP_PET_FINDMUSHROOM_MARK`.
- Limited purchase-count sync was changed from 533 to **547** (`RUDP_SYNC_LIMITED_PURCHASE_ITEMS`) based on the supplied enum; its payload still needs live-client verification.
- `RUDP_SAFE_ZONE_CHANGE` remains **117**, and `RUDP_JOIN_MATCH_PREPARE` / `RUDP_JOIN_MATCH_POST` remain **447/448**, matching the dump.

These mismatches plausibly explain why the shop and revive/round-result UI did not respond as expected: the client would dispatch the numeric command as a different message type. This is a strong source-level finding, but it is not proof that all other command IDs or payloads are correct. The command-ID changes do not establish that the join request parser, purchase payload, revival state, zone timing, or round transition is fully compatible.
