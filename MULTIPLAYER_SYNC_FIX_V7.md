# v7 — multiplayer visibility + ping fixes (Go changes NOT compiled in the authoring sandbox: run `cd match-server && go build ./... && go test ./...` first)

## Why two accounts could be "in the game" but never see each other
1. **Routing by matchmaker id.** Both tokens carry the same `mid`. `MatchManager.join` now sends a player to an existing match with the SAME `mid` first
   (even if that match already started / bot-filled), then falls back to the old "match still gathering players" rule. Log: `join route: ... SAME match id` or `... NEW match`.
2. **Bad token = same account.** If the prepare_token failed verification, both clients got the same fallback account id (10000001) and the 2nd was treated as a
   *reconnect of the 1st*. Each such connection now gets its own temporary id and a `WARNING prepare_token missing/invalid` log line (the real cause is then a
   `MATCH_JWT_SECRET` mismatch between `transport/tcp_matchmaking.js` and the Go node).
3. A duplicate join post from the same connection is ignored instead of taking a 2nd roster slot.

## Real-time sync
Movement relay (cmd 1001 -> every other player) runs at 20 Hz (was 10 Hz) on the 50 ms ticker.

## Ping 999+
* Ping echo (cmd 3) is written straight to the socket instead of the Writer FIFO.
* DoS guard limit is per source IP; two clients on one IP (same phone / Wi-Fi NAT / hotspot) share it. Default raised 400 -> 1200 weighted pkt/s,
  override with `MATCH_GUARD_MAX_PER_SEC`. A blocked IP shows as `rate-limit: blocking flooding source` in `udp_game_node.console.log`.
* The client must reach the server on a REAL address: set `SIGMA_MATCH_HOST=<server LAN/public IP>` (default 127.0.0.1 only works on the server's own device).

## v8 — loser dropped to lobby after round 1 ("Tie")
A down player's client auto-posts cmd 191 (quit). The server used to answer with cmd 192, which is the client's cue to LEAVE, so the loser went back to
the lobby while the winner kept playing. Quits from a down player or during the between-rounds transition are now ignored (no cmd 192, slot kept, the
round-reset revive brings them back). Log line to look for: `quit while down/between rounds ... IGNORED`.

## v9 — live movement
Each human's cmd-1001 state is relayed the moment it arrives (handleClientPos -> relayMove) as cmd 1001 (all) + cmd 1012 V2 (others, the form FF 1.70.1 applies,
same as the bots), strictly increasing seq. Log: `MOVE first relay slot=.. seq=.. base=..`.

## Round-1 loser leaving: collect these lines (same attempt)
grep -E "ROUND|cmd=107 DEAD|cmd=191|quit while|roster-|HELLO|cmd=103|new session|join route|cmd=388" logs/udp_game_node.console.log | tail -80

## v10 — run button vanishing after a few seconds
The replay "reseed" (cmd 101 self+roster x12, cmd 230 observer-seed, cmd 108 wheel-kick) fired for ~12 s after the first cmd 1001 on EVERY live match.
Now OFF unless MATCH_REPLAY_RESEED=1.
