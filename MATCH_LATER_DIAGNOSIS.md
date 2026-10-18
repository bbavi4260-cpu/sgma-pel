# Match Later / “Reading game info, please wait” — Diagnosis

## Result

The supplied V37 package does **not currently implement a complete Match Later/game-start path**. The observed indefinite wait is consistent with that missing backend functionality, rather than a profile HTTP route regression.

I did **not** add a fabricated TCP success packet: the client must receive valid matchmaking and game-server protocol messages, and guessing them could leave the client in a worse state or corrupt a session.

## Evidence in the supplied files

- `README.md` says the game-node listener does not implement matchmaking or gameplay responses.
- `tcp.py` accepts framed bytes on TCP port `10300`, logs parsed header metadata, and sends no game-protocol response.
- `test_server.py` serves HTTP login/profile compatibility routes; it has no Match Later/matchmaking implementation. Its `LoginRes` advertises the game node using `SIGMA_GAME_NODE`, defaulting to `127.0.0.1:10300`.
- The notification listener is only an opaque presence/auth-heartbeat acknowledgement boundary; it is not a matchmaking server.
- The decompiled client declares `EMatchmaking` TCP commands and a `MatchmakingSussNtf` with `match_id`, `server_addr`, `secret`, `prepare_token`, map and mode fields. But key client method bodies such as `MatchSuccessLoading`, `OnMsgMatchMakingSussNtf`, and `OnMsgHasLatestGameInfo` are empty stubs in the provided decompile. That archive therefore does not reveal enough implementation detail to construct valid server replies.

## Checks run here

- Python syntax compilation passed for `test_server.py`, `game_node.py`, and `notification_gateway.py`.
- `/getlogindata` returned HTTP 200.
- `/getmatchstatshistory` returned HTTP 200 with the deliberately empty declared response.
- `/matchmaking/start` returned HTTP 404; this route is only a probe, not necessarily the client's actual matchmaking transport.
- A raw TCP smoke test confirmed the game listener accepts framed bytes and remains silent. This verifies listener behavior; it does **not** validate game start.

## One configuration check

If the game runs on a different device from the server, make sure `LoginRes` advertises a reachable address rather than loopback. Set `SIGMA_GAME_NODE=<server-LAN-or-public-IP>:10300` when starting the server (and expose/forward that port as appropriate). This may fix a connectivity problem, but **it will not add the missing matchmaking/gameplay protocol**.

## What is needed to implement the actual fix

Please run the server, click Start once, and provide these files from that same attempt:

1. `logs/console.log`
2. `logs/tcp_packets.jsonl` and `logs/game_node.console.log`
3. `logs/notification.console.log`

Also include the client version, selected mode/map, and whether the phone and server are on the same device, same Wi-Fi/LAN, or separate networks. These logs are designed to omit full payloads and auth tokens; still redact any public IP or personal identifier before sharing. The key distinction is whether the attempt reaches TCP `10300`, gets a matchmaking notification, or fails before either event.

A real server-side fix then needs a valid matchmaking service and a protocol-compatible game-server response path (including the game-info loading/ready exchange). The included project currently supplies only listener boundaries, not those services.

## Comparison with `dev-kroux/krouxgg-server`

I inspected the linked repository (`main` branch, commit `0266733`). It does **not** provide a standalone private game server or a local implementation of matchmaking/game-info:

- Its HTTP handler forwards requests to an external official game-service hostname, with some response rewriting.
- Its TCP listener is a byte relay: it forwards client traffic to a fixed remote gateway address on port `39698` and sends the remote bytes back. It does not parse matchmaking messages or synthesize a `MatchmakingSussNtf`/game-info response.
- The source contains no matchmaking/game-info/start-match handler identifiers. Therefore it cannot be merged as the missing local game server. At most, it is a proxy dependent on the configured external upstream continuing to accept the client protocol.

## Security warning about the linked repository

The pasted file and public repository contain a hard-coded value named as an Upstash Redis REST bearer token. I have not used it or repeated it here. Treat it as exposed: revoke/rotate it in Upstash, replace it with an environment-only secret, and remove it from the public Git history if it is a real credential. The proxy also writes raw HTTP request/response bodies and raw TCP packets into `captured_packets`; those can contain login/session data. Do not run it with real accounts until you have reviewed the code, changed exposed credentials, and secured or disabled those packet captures.
