

## Current loading-screen limitation

The observed client screenshot has Battle Royale selected. The bundled Go UDP node's recovered game-state and round implementation is explicitly Contra Squad (game mode 15), including CS round, shop, and GRI state. The TCP matcher forwards the requested game mode, but the UDP node is not a Battle Royale world server. This mode mismatch can explain the transition to loading followed by a stuck map. Do not treat the billboard or wait-time changes as a Battle Royale gameplay fix; a true BR implementation needs the BR-specific UDP join/world-start/replication sequence and matching client captures. For diagnostics, collect the `[tcp-mm] START ...` line plus the full `[mm-udp]` join logs from the same attempt.
