# Battle Royale isolation (game_mode=1, map_id=1)

Battle Royale is a separate match path from Clash Squad.

- Matchmaking forms BR queues separately and caps a BR match at 48 players (`MM_BR_MATCH_SIZE`).
- The match manager only admits a player when map/game mode match the existing match.
- BR has no CS shop, CS economy, CS rounds, team wipe, teammate rescue, round respawn, or CS bot fill.
- Players use BR spawn points, enter with fists + medkits, and collect ground loot.
- BR uses single-life elimination; kills are counted player-vs-player.
- Eliminated players drop their carried loot and spectate remaining survivors.
- A shrinking safe zone is authoritative on the BR match loop.
- BR does not emit Clash Squad GRI phase/round/draw/shop state. Only the low-level UDP/PRI transport is shared because the 1.70.1 client uses the same transport for multiple game modes.
- A one-player/private BR test is allowed to remain active; the server does not auto-finish it just because one player is alive.

The BR path is selected by `map_id=1` + `game_mode=1`. CS remains `game_mode=15`.

- Safe zone has 4 stages: 1350m → 900m → 550m → 250m. Each stage lasts 3 minutes before the next shrink; each shrink transition takes 30 seconds.
- Map loot is seeded at 144 randomized pickup locations (weapons, ammo, medkits, armor/helmets, and gloo walls), based on the known BR world spawn locations.

## How the mode is selected (do not trust the client)

The gameplay path comes from the **signed** `prepare_token`, never from the client's own cmd-440
`GameMode` byte, which a modified client controls.

1. `src/tcp/matchmaker.js` already knows the mode when it forms a match, so `buildPrepareToken`
   writes it into the JWT as the `map` + `mode` claims. The custom-room start
   (`src/tcp/handlers/RoomStart.js`) passes the room's `map_id` / `game_mode` the same way.
2. `token.Claims.MapID` / `GameMode` carry them into the match server; `resolvePlayer` returns them
   as a `matchMode` and the session stores it.
3. `newMatch` fixes `m.mode` / `m.battleRoyale` from the FIRST player's claims, and `canAdmit`
   requires an exact map+mode match, so a match can never mix BR and CS players.

An absent or zero claim means Clash Squad, so an old token — or a manual test join with no token —
keeps the previous behaviour exactly.

## Teams in BR

Solo BR is free-for-all, so **each player gets its own team**: `allocSlot` sets the entity-id hibyte
to the player's slot, making `teamOf()` differ for any two players. Nothing else in the combat code
needs a BR special case, because the existing rules then do the right thing:

- `applyDamage` credits damage and kills across teams,
- `downOrKill` sees `teamSize() == 1` and kills outright — single-life elimination, no DBNO revive,
- a team wipe / round end can never fire, because a team is one player.

Only the spectate target needed a BR branch (`spectateFor`): with no teammates, an eliminated player
follows any remaining survivor instead of returning 0, which the client reads as "no camera target".

Squad BR is the natural extension of this: map `team` to the squad id in `allocSlot`, and the
knockdown/revive path comes back on its own because `teamSize()` is then > 1.

## Zone damage and the client's circle

The client lerps its own circle off a local sim clock that trails the server's by the network
latency plus sim jitter, so the circle it SHOWS is always a little larger than a same-instant lerp.
`damageRadius` therefore evaluates the shrinking radius `zoneClientLag` (300 ms) in the past, exactly
as the Clash Squad zone does. Without it, players take zone damage while standing visibly inside the
circle.

## Testing knobs

| Env | Effect |
| --- | --- |
| `MM_BR_MATCH_SIZE` | BR lobby size on the matchmaker (default 48; keep it in step with `maxBRPlayers`) |
| `MATCH_BR_CS_GRI=1` | Also stream the Clash Squad GRI in a BR match — an A/B escape hatch if a client build turns out to need the GRI to unlock movement |

## Still missing (deliberately)

- No aircraft / parachute drop: players are force-teleported to their ground spawn, because the
  flight packet payload is not verified for 1.70.1 and an invented one crashes the client.
- No squads, no revive, no vehicles.
- Loot has no rarity tiers and no airdrops.
