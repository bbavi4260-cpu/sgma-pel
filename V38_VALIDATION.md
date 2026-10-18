# V38 validation record

## Delivered changes

- Added `data/sigma.db` (SQLite) with `account_state`, `available_maps`, and `schema_migrations` tables.
- The `account_state` table stores the complete current local account JSON document, preserving nested profile, wallet, selected-items, friends, mail, and other fields. The server continues to serve its existing single canonical local account; this is not a multi-user session/matchmaking database.
- On first launch without a database row, the server imports `data/account_state.json` once and leaves it unchanged as a migration backup. Subsequent account persistence goes to SQLite.
- Seeded the five map codes provided in the new attachment: `PARADISE`, `AZORES`, `EMAPSONORAN`, `PARADISE_NEO`, and `PARADISE_NEOMAX`. `PARADISE` is selected; all five are marked available in `BATTLEROYALE` mode. These are the supplied code strings, not verified numeric client map IDs or map assets.
- Added `GET /available_maps` for local diagnostic inspection and a `select_map()` database method. The game client does not consume this REST endpoint.
- Added standalone `tcp.py` (TCP `10300`) and `udp2.py` (UDP `10200`) entrypoints. The separate notification TCP listener defaults to `10301` to avoid a port collision. TCP parses the declared `Cmd[1]+Length[4]` frame stream; UDP parses the `MsgKey` and declared mini/non-reliable/reliable header variants. `udp2.py` delegates to the actual `udp.py` implementation. The old `game_node.py` and `udp_game_node.py` names remain compatibility aliases. Both game transport listeners remain diagnostic-only and do not send game-protocol replies; UDP is not advertised to the client.
- Database and SQLite sidecar files are restricted to owner read/write permissions where the platform supports it.

## Validation run

- `python3 -m unittest discover -s tests -v`: 11 tests passed (transport header boundaries plus storage and billboard tests).
- Python syntax compilation passed for the server, TCP/UDP parsers, aliases, and tests.
- `bash -n termux_start_server.sh termux_stop_server.sh`: passed.
- Existing full launcher smoke test in a temporary directory: HTTP `/available_maps` returned five maps; TCP accepted a test connection; UDP received a test datagram; launcher stopped cleanly. New unit tests separately cover TCP fragmentation/coalescing and UDP 5/8/12-byte header parsing.
- The packaged database contains 51 source profiles and all five seeded map codes. Original JSON remains byte-for-byte unchanged.
- Lobby billboard support: `/logingetbroadcast` now returns a configurable `LoginBroadcastRes` with a scrolling-marquee entry; regression tests cover configured content and empty-content behavior. `ScrollMarqueeDesc` member order is present in the decompiler dump, but explicit protobuf tag annotations are absent, so sequential tags are a best-supported inference and still need client-run verification.

## Important limitation

The additional `datasigma.zip` confirms client model fields and enum values such as `MATCHMAKING=3`, casual mode `1`, Battle Royale mode `1`, plus request and success-handoff field names. Assembly-CSharp declarations now let the diagnostic transport parser use the TCP 5-byte header constants and UDP MsgKey/send options/header-size constants. The method bodies remain decompiler stubs: TCP AES/session setup, UDP CRC/transforms, FastBinary serialization/message IDs and reliable ACK behavior, numeric map-ID rows, map bundles, and realtime gameplay are still missing. Key client matchmaking callbacks are also empty stubs. Therefore the transport fixes improve framing and diagnostics but do not by themselves make a match playable or guarantee removal of “Reading game info, please wait.” See [HELP_TCP_REVIEW.md](HELP_TCP_REVIEW.md), [ASSEMBLY_PROTO_REVIEW.md](ASSEMBLY_PROTO_REVIEW.md), and [DATASIGMA_SOURCE_REVIEW.md](DATASIGMA_SOURCE_REVIEW.md) for details.
