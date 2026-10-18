# Sigma v38: TCP matchmaking add-on (Termux)

This is a **copy of the old `sigma_v38_sqlite_udp` source** with a selective PrjShadow TCP/matchmaking add-on. The old Python HTTP routes, SQLite account/catalog data, billboard responses, and notification listener code are retained. One TCP-related LoginRes default now advertises the new matcher at `10300`; no other HTTP behavior was changed. The old `tcp.py` and `udp2.py` diagnostic files remain in the tree, but the launcher uses the new TCP gateway and the Go match node instead.

## Ports

- HTTP/SQLite compatibility service: `3000`
- TCP matchmaking gateway (also advertised by LoginRes `notification_channel`): `10300`
- Legacy Python notification listener (retained, not advertised to the game client): `10301`
- UDP match/game node: `10200`

The values are configurable with `SIGMA_TEST_PORT`, `SIGMA_GAME_NODE_PORT`, `SIGMA_NOTIFICATION_PORT`, and `SIGMA_UDP_GAME_NODE_PORT`.

## Install and start

From Termux, in the extracted project directory:

```sh
pkg update
pkg install python nodejs-lts golang unzip
python -m py_compile test_server.py
bash termux_start_server.sh
```

The first start compiles the included Go match server and may take several minutes while Go downloads/builds modules. The bundled `match-server/go.mod` currently requires Go 1.25 or newer. No npm packages, Redis, or `better-sqlite3` are required by the new TCP sidecar.

Stop with **Ctrl-C**, or from another Termux session:

```sh
bash termux_stop_server.sh
```

The launcher reads `.env` only when that file exists. For a client on the **same phone**, the match address defaults to loopback. For a client on another device, set the phone's reachable LAN/VPN address before starting:

```sh
export SIGMA_MATCH_HOST=192.168.1.25
bash termux_start_server.sh
```

The launcher uses that host for both the client-advertised TCP matchmaking address and the UDP match handoff. Do not use `127.0.0.1` for a client on a different device. TCP `10300` and UDP `10200` must be reachable from the client.

## What the TCP add-on does

- Parses client frames as `Cmd + Region + u32 big-endian length + AES-CBC payload`; heartbeat is the two-byte client frame and receives the one-byte `0x02` acknowledgement.
- Reads the canonical session token/account JSON from the old server's SQLite `account_state` table via Python's standard-library `sqlite3` (with the old JSON seed as fallback). It does not log or expose that token.
- Handles Sigma matchmaking protocol `3`, sends `START_NTF` (`cmd=11`), queues requests in-process, and returns `MatchmakingSussNtf` (`cmd=5`) when enough requests arrive or the timeout expires.
- Signs the local prepare token using `MATCH_JWT_SECRET`; the UDP node uses the same secret. `MATCH_BOT=1` is enabled by default for a solo local test.

The queue is **single-process/in-memory**, intended for a local Termux test server, not a multi-node deployment. Tune with `SIGMA_MM_MIN_PLAYERS` and `SIGMA_MM_TIMEOUT_MS`.

## Troubleshooting

- `EADDRINUSE`: stop the previous server or change the corresponding port environment variable.
- Client connects but matchmaking doesn't start: inspect `logs/game_node.console.log`; verify the client points to this phone on TCP `10300` and that LoginRes returns the same local `session_token` stored in SQLite.
- Match starts but client cannot join: verify `SIGMA_MATCH_HOST` is reachable and UDP `10200` is allowed.
- Go build errors: run `cd match-server && go version && go build .` and review Go's reported toolchain/dependency error.

This is a local compatibility implementation using the supplied PrjShadow transport/match resources and the old Sigma message declarations. It does not claim that every gameplay, room, or custom protocol path is implemented.


### TCP auth compatibility

The gateway ACKs an empty `Cmd=1` control frame but does **not** mark it authenticated. For nonempty frames it tries AES-CBC first; as a compatibility fallback, it accepts plaintext only if the complete payload exactly matches the session token in SQLite. Any other value is rejected. Auth logs never print the token or ciphertext. If the client still reconnects, send the new `[tcp-mm] auth rejected ... payload_bytes=...` line; it tells us whether the remaining issue is padding/length or a token mismatch.


For a client build that authenticates with the exact local token but sends unencrypted matchmaking messages, the gateway now validates a plaintext `ProtoReq` fallback after authentication. Only supported matchmaking START/CANCEL commands are handled. If the new log still reports `app request rejected`, send that line; it records parser reasons and payload length, not packet contents.
