# Sigma V38 SQLite, Maps, and Transport Compatibility Test

This package is an explicit nearby-version source-default revision. It imports
the uploaded FreeFireServer's literal default profile roster, selected Adam
clothes/skin state, source `items.json` ownership corpus, and source free store
sample. The matched Sigma 1.0.113 client/source proto supplies every emitted
response envelope and field.

## V38 storage and map catalog

V38 stores the complete local account document in `data/sigma.db` (SQLite,
`account_state` table) and stores map availability in the `available_maps`
table. On first launch it imports the existing `data/account_state.json`; the
server leaves that file untouched as a migration backup. Profile, wallet,
selected-items, friends, mails, and unknown account fields are preserved in the
SQLite JSON document. The map catalog is seeded from
`resources/available_maps.json`, using only the five map codes in the supplied
config. `GET /available_maps` provides a diagnostic JSON view; it is not a
client matchmaking protocol endpoint.

The dedicated `tcp.py` listener runs on TCP port `10300`; `udp2.py` runs on
UDP port `10200` by default (`SIGMA_UDP_GAME_NODE_PORT` changes the launcher
port). The notification/presence listener uses TCP port `10301` to avoid
colliding with the game TCP listener. UDP is not advertised to the client. TCP now parses
the declared 5-byte frame header (1-byte command + little-endian 4-byte length)
across split/coalesced stream reads. UDP parses the declared MsgKey, send option,
command, and 5/8/12-byte header variants. Both remain diagnostic listeners: they
do not decrypt payloads, confirm sessions, maintain RUDP ACK/retransmit state,
or implement matchmaking/gameplay, so they are not a playable game server.

It preserves V29's working lobby and Vault source-default state, the verified
V25 non-empty hide-avatar response, V30's skill projection, V30's matching
51-profile cardinality, and V31's non-null `GetPlayerTCStats` plus declared
empty `GetBattleTag` response.

V32 added only the two 404 routes proved by the V31 screen recording and same-run
log: `GetMatchStatsHistory` returns an empty declared `CSGetMatchStatsListRes`,
and `GetPlayerStats` returns non-null zeroed solo, duo, and quad statistics for
the canonical local account. It does not fabricate match records.

V33 adds one confirmed structural field to the retained `GetPlayerTCStats`
response. Sigma declares `AccountInfoWithTCStats.detailed_stats` at nested field
5. V33 makes that `DetailedTCStats` message present but empty (`2a00`), keeping
all ten Clash-Squad counters at protobuf zero. The complete response is exactly
`0a070881c2d72f2a00`. It does not add match records, wins, kills, ranks, ratings,
or any fabricated resource data.

V34 addresses the exact same-run re-login boundary recorded on the phone. After
all lobby/Profile routes returned HTTP 200, `SetPregameShowChoices` and
`UpdateSocialBasicInfo` returned 404 immediately before OAuth inspect and the
client’s “please re-login” message. V34 adds only their declared contracts:
`SetPregameShowChoices` receives an empty protobuf acknowledgement because no
response type is declared, and `UpdateSocialBasicInfo` returns
`CSSocialBasicInfoRes.info` containing only the canonical account ID
(`0a050881c2d72f`). It does not persist or invent social settings, tags,
signature, preferences, or pregame choices.

V35 addresses the subsequent no-HTTP-error re-login boundary. `LoginRes`
advertises a persistent notification channel at `127.0.0.1:10301`, but V34
started no listener there. V35 starts a local TCP presence listener alongside
HTTP. It accepts a complete opaque Cmd=1 auth frame and returns only bare
`0x02`; it returns the same bare `0x02` for Cmd=2 heartbeats. The listener logs
only command metadata and encrypted length. It does **not** decrypt, store, or
log session tokens or payloads, and it does not implement notification pushes,
matchmaking, the port-10300 realtime game node, or any fabricated packet.

V36 restores the omitted **source account lifecycle state** that feeds the
Profile screen and visible selected-avatar state. The previous V35 package kept
the source profile roster and inventory, but it projected a different hardcoded
LoginData snapshot, emitted only avatar/skin/clothes from SelectedItems, and
returned only an empty AccountPrefers placeholder for Personal Show basic info.
That is why the Profile screen displayed UID 0, level 0, and empty-looking
presentation fields.

V36 projects the literal FreeFireServer default account values through matched
Sigma tags: level 100, 250000 exp, rank 1, 1000 ranking points, 999999 wallet
currency, banner 900000014, head picture 902000003, two source loadouts, and
the source skill-slot layout. It keeps selected profile and SelectedItems in
sync after selection or clothes changes. `GetBackpack` now returns the supported
full SelectedItems state, while `GetPlayerPersonalShow` returns full
AccountInfoBasic plus the selected AvatarProfile and source default EP-history
entries. No match records, social tags, battle tags, or resource configuration
data is invented.

V37 keeps the V36 source-default LoginData and complete Backpack SelectedItems
state, but reduces only `GetPlayerPersonalShow` after V36 blanked the Profile
scene. It retains source `AccountInfoBasic` and the selected AvatarProfile, then
restores V35's present empty `DiamondCostRes` at top-level field 10. It omits
only V36's added top-level ranking-position and repeated EP-history fields. This
is a bounded Sigma-version response-shape compatibility test, not a reversal of
the source-first lifecycle.

Character is an independent unresolved client-side configuration dependency:
the source provides owned skill IDs, while the matched Sigma Character cards
also require separate avatar-skill definition data not present in the preserved
local resources or nearby source archive.

The original Sigma `fileinfo` and `config/avatar` payloads remain byte-for-byte
local preserved artifacts with passive routes. The new `available_maps.json`
file is only a metadata catalog; it does not contain map assets, collision data,
or a resource descriptor, and it does not make a map playable by itself.

## Termux live logs

The bundled `termux_start_server.sh` runs the HTTP server, notification listener, TCP game node, and UDP game node while keeping the terminal attached to live, prefixed logs. Outputs are persisted under `logs/console.log`, `logs/notification.console.log`, `logs/game_node.console.log`, and `logs/udp_game_node.console.log`; transport logs contain metadata, not packet bytes.

From Termux, run:

```bash
cd sigma_v38_sqlite_udp
chmod +x termux_start_server.sh termux_stop_server.sh
bash termux_start_server.sh
```

Keep that terminal open to watch live logs. Press `Ctrl-C` to stop all four listeners cleanly. From another Termux session, they can also be stopped with:

```bash
cd sigma_v38_sqlite_udp
bash termux_stop_server.sh
```

To follow an individual log without starting services again, use `tail -f logs/console.log`, `logs/notification.console.log`, `logs/game_node.console.log`, or `logs/udp_game_node.console.log`.

### Optional ports

The default HTTP port is `3000`, TCP matchmaking port is `10300`, legacy notification TCP port is `10301`, and UDP match port is `10200`. Override them before starting if needed:

```bash
SIGMA_TEST_PORT=3000 SIGMA_NOTIFICATION_PORT=10301 SIGMA_GAME_NODE_PORT=10300 SIGMA_UDP_GAME_NODE_PORT=10200 SIGMA_MATCH_HOST=127.0.0.1 bash termux_start_server.sh
```

The server binds to `0.0.0.0` by default. The merged launcher keeps the old HTTP/SQLite and notification code, advertises the new matcher on TCP `10300`, and uses the included Go match node on UDP `10200`. For Termux install/build steps and host configuration, see [TCP_MATCHMAKING_TERMUX.md](TCP_MATCHMAKING_TERMUX.md). The old `tcp.py` and `udp2.py` files remain as diagnostic references but are not started by the merged launcher.

The TCP matcher uses AES-CBC client payloads and the Sigma matchmaking message IDs recovered from the supplied client/source resources. LoginRes points the client's `notification_channel` at TCP `10300`; the old Python notification listener remains on TCP `10301` but is no longer advertised. The queue is in-memory and intended for one local server process. See [INTEGRATION_NOTES.md](INTEGRATION_NOTES.md) for exact scope and limitations.

The solo queue fallback now waits 3 seconds by default (it was 15 s, which made every start feel frozen); `MatchmakingStartNtf` still advertises the 15 s estimate to the client UI. Override with `SIGMA_MM_TIMEOUT_MS` if needed.

**Fight Out loading fix:** Fight Out (`game_mode=36`) stays selectable and is echoed in `MatchmakingStartNtf`, but the `MatchmakingSussNtf` loading handoff always carries the mode the Go UDP node really serves (Contra Squad, mode 15, map 1). Handing off 36 made the client load a Fight Out scene the server never initialises and hang on the loading screen. After tapping the map star, `logs/game_node.console.log` shows `SUSS ... handoff_mode=15`, then `logs/udp_game_node.console.log` should show `new session`, `HELLO`, `cmd=100/101/130` and `starting CS match loop`. If `new session` never appears, the phone cannot reach `SIGMA_MATCH_HOST:10200/udp`.

For solo Clash Squad testing, set `MATCH_BOT=1`. When the waiting period expires,
the Go match node adds three visible filler bots by default (`BOT`, `BOT-2`,
`BOT-3`) with unique entities, names, `cmd=101` joins, BindPRI entries and PRI
health blocks. Change the count with `MATCH_BOT_COUNT=1..7`. These are currently
static roster bots; the primary `BOT` remains the combat/HP target, while the
additional bots are visual roster fillers. When a real second human joins, all
filler bots are retired so the human-vs-human path remains unchanged.

The TCP matcher also answers matchmaking subcommand `GAMEOPENINGINFO` (`cmd=7`), which the client requests from the mode-selection screen before starting a match. It advertises only the implemented Contra Squad mode (`map_id=1`, `game_mode=15`). If a client sends an unsupported Battle Royale request (`game_mode=41`), the matcher normalizes it to this supported CS handoff instead of sending CS-only UDP packets under a BR identity and leaving the client stuck on loading.

## Lobby scrolling billboard

The video shows a scrolling marquee below the profile area. V38 now returns a
`LoginBroadcastRes` from `/logingetbroadcast` containing a `ScrollMarqueeDesc`
entry, so the client can render the text using its own lobby marquee UI. The
same ticker is also returned directly by `/getscrollmarquee` on subsequent lobby
requests, so reopening the lobby does not depend on the login-only response. The
default is `WELCOME TO SIGMA PRIVATE SERVER!`; configure it before starting:

```bash
SIGMA_BILLBOARD_TEXT="Your lobby message here" \
SIGMA_BILLBOARD_LANGUAGE="en" \
bash termux_start_server.sh
```

Optional `SIGMA_BILLBOARD_DURATION_SECONDS` controls how long the message stays
valid (default: one year). `SIGMA_BILLBOARD_START_GRACE_SECONDS` (default: 30 days)
backs up the start timestamp so a small client/server clock difference does not
make the ticker appear expired or not-yet-active. A blank `SIGMA_BILLBOARD_TEXT`
disables the marquee. The protobuf field numbers are verified against the
supplied `protocol/protos/proto.proto`. Every `/logingetbroadcast` and
`/getscrollmarquee` request generates a fresh active response; the server cannot
force the game client to re-request or redraw a marquee if the client caches it.


### Mode compatibility

The included Go UDP match node implements Contra Squad (game mode 15), not the Battle Royale world/session flow. The matcher now prevents a false BR success handoff: it advertises CS only and normalizes unsupported mode requests to CS. A genuine BR implementation would still require BR-specific join and world replication; until then, select Contra Squad for the playable path. For diagnostics, retain the TCP matcher and UDP match-node logs, especially `[tcp-mm] START ...`, any `unsupported game_mode ... falling back` line, and all `[mm-udp]` lines around `JOIN`, `prepare_token`, `cmd=100`, `cmd=101`, `cmd=130`, and `starting CS match loop`.

## Loading stuck at ~90% (cmd 448 join fix)

Logs showed the client reaching UDP (`new session`, `HELLO`, ACK) and then sending one `cmd=448` packet of ~646 bytes (the size of the prepare_token). The Go node only logged it as "no response required", so no join reply (`cmd=100/101/130`) was ever sent and the client waited forever on the loading screen. `handlers.go` now treats a `cmd=448` that contains the prepare_token as the join request (same path as `cmd=440`), and ignores a later duplicate 439/440. The log line `cmd=448 carries prepare_token ... before=... after=...` shows the non-secret bytes around the token if the layout needs further tuning. **Rebuild is required** (`bash termux_start_server.sh` rebuilds the Go node).

## Loading error `ArgumentOutOfRangeException ... Buffer.BlockCopy ... count`

After the cmd-448 fix the join completed (`cmd=100/101/130/176/117/457/118` all ACKed) and the client then died with this error. The only join-time packet that was sent without a log label (`seq=5`) is `cmd=1030 InGameChat` ("Apenas testando umas coisas aqui"), a leftover debug packet whose own comment says "apparently sends a chat message? unsure". It is now **off by default** (`MATCH_DEBUG_CHAT=1` re-enables it).

If the error persists, bisect with `MATCH_SKIP_CMDS`, e.g.

```bash
MATCH_SKIP_CMDS=176,457 bash termux_start_server.sh      # never send cmd 176 and 457
MATCH_SKIP_CMDS=101 bash termux_start_server.sh          # (client will not get a player; only to test)
```

Skipped packets are logged as `SKIPPED cmd=N`. Skipping happens before seq/order are assigned, so reliable ordering stays intact.

## Solo wait shortened

With `MATCH_BOT=1` (default in `termux_start_server.sh`) the in-match "Waiting for players" hold is now 2 s instead of 30 s (bots appear almost immediately; `MATCH_BOT_COUNT=1..7` sets how many), since bots fill the match anyway.

## Inbound diagnostics

`udp_game_node.console.log` now prints `inbound last 5s: cmdN=xK ...` (every command the client sends, including ping 3 / clock 1000 / position 1001 which the per-packet log hides) and `first-samples cmd=... head=...` for those hidden commands. Use it to see whether the client ever sends cmd 1001 (position) after the join.

## How to confirm you run the fixed build

`logs/udp_game_node.console.log` must contain `[mm-udp] BUILD fixed-r6 ...` right after start. If it is missing you are still running an older extraction. Also `cat VERSION` in the project folder prints `fixed-r6`.

## Experiment: MATCH_SEND_110=1

With the r6 build the client joins, stays connected ~70 s, sends ping + cmd 1000 but never cmd 1001 (position), i.e. it never leaves its pre-match state. Reference servers send `cmd 110 BattleStart` right after `cmd 130`. It was disabled for Contra Squad on the older 1.70 client (it caused a reconnect), but the 1.0.113 client differs (join arrives as cmd 448). Start with `MATCH_SEND_110=1` to send it (logged as `cmd=110 BattleStart (MATCH_SEND_110 experiment)`). If the client then reconnects (`HELLO(cmd=5)`), send the log.

## r8: test defaults in the launcher

`termux_start_server.sh` now exports `MATCH_SEND_110=1`, `MATCH_DEBUG_POS=1`, `MATCH_NO_ZONE=1`, `MATCH_BOT_COUNT=5` unless you set them yourself, so a plain `bash termux_start_server.sh` runs the cmd-110 experiment. Check `logs/udp_game_node.console.log` for `BUILD fixed-r8` and `cmd=110 BattleStart (MATCH_SEND_110 experiment)`.
