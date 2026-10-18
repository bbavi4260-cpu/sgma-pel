package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

// Battle Royale is intentionally a separate gameplay path from Clash Squad.
// It does not use CS teams, rounds, shop/economy, arena rotation, CS bot filling,
// teammate revive, or round transitions. The only shared code is the low-level
// packet/replication transport that the 1.70.1 client itself also shares.
// matchMode is the gameplay path a joining player was matched into. It comes from the
// signed prepare_token (token.Claims.MapID / GameMode), which the matchmaker fills from the
// queue entry — never from the client's own cmd-440 bytes, which a modified client controls.
// The zero value is Clash Squad, so an old token (or a manual test join with no token) keeps
// the previous behaviour exactly.
type matchMode struct {
	mapID  uint32
	game   uint8
	rounds uint8 // match-length override from the signed token (Fight Out = 7); 0 = default
}

// Game-mode ids as the lobby/matchmaker uses them. CS is 15; BR is 1 on map 1 (Bermuda).
const (
	gmBattleRoyale uint8  = 1
	gmClashSquad   uint8  = 15
	brMapID        uint32 = 1
)

// isBattleRoyale reports whether this mode selects the BR path. Requiring BOTH the map and the
// game mode keeps an unexpected/garbage token on the CS path rather than half-starting BR.
func (mm matchMode) isBattleRoyale() bool { return mm.game == gmBattleRoyale && mm.mapID == brMapID }

// sameAs gates match routing: a joining player may only be admitted into a match running the
// SAME map + game mode, so a BR player can never land in a Clash Squad match (and vice versa).
func (mm matchMode) sameAs(o matchMode) bool {
	return mm.mapID == o.mapID && mm.game == o.game && mm.rounds == o.rounds // a 7-round Fight Out never mixes with a default-length match
}

// brStage is the BR match state machine. It is deliberately separate from csPhase: BR has no
// buy phase, no rounds and no round transitions, so reusing csPhase would mean gating every CS
// transition on a mode flag. The csPhase the client is told about is set by enter() as usual.
type brStage int

const (
	brWaiting  brStage = iota // gathering players in the lobby area (the match is still admit-able)
	brFight                   // dropped, zone running, single-life elimination
	brMatchEnd                // a winner (or nobody) is left: results, stats hold, teardown
)

// maxBRPlayers caps a BR match's roster. 48 mirrors the live BR lobby size and is what
// MM_BR_MATCH_SIZE on the matchmaker side forms queues against.
const maxBRPlayers = 48

const (
	brCenterX = 180.0
	brCenterZ = 120.0
	// The initial circle covers the whole legal BR play area.
	brOuterRadius  = 1350.0
	brInnerRadius  = 80.0
	brWaitDuration = 25 * time.Second
	// BR zone: four stages. Players get 3 full minutes between stage transitions;
	// each transition itself takes 30 seconds to close.
	brZoneStageWait  = 3 * time.Minute
	brShrinkDuration = 30 * time.Second
	brDamage         = 12
	brDamageEvery    = 2 * time.Second
)

var brRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// brHoldsOnIsland reports whether BR should keep joining players on the waiting island until the
// gathering countdown ends, instead of dropping them straight onto the battle map. Set
// MATCH_BR_ISLAND=0 to go back to the old behaviour (cmd 110 at join time).
func brHoldsOnIsland() bool { return os.Getenv("MATCH_BR_ISLAND") != "0" }

// These are BR world spawn points, not CS arenas. They are spread over the Bermuda
// play area so a 48-player match does not stack everybody at one CS gate.
var brSpawnPoints = []message.Vec3{
	{X: 366.6, Y: 16.2, Z: -261.5}, {X: 158.8, Y: 0.8, Z: -148.4}, {X: -146.4, Y: 30.5, Z: 320.5},
	{X: 131.0, Y: 2.6, Z: 415.6}, {X: 306.0, Y: 10.2, Z: 769.7}, {X: 565.5, Y: 25.2, Z: -560.1},
	{X: 659.5, Y: 25.6, Z: -227.0}, {X: 909.9, Y: 49.6, Z: 450.5}, {X: 1010, Y: 12, Z: -120},
	{X: 760, Y: 18, Z: -720}, {X: 420, Y: 8, Z: -780}, {X: 50, Y: 4, Z: -650},
	{X: -250, Y: 8, Z: -520}, {X: -420, Y: 15, Z: -250}, {X: -520, Y: 10, Z: 40},
	{X: -460, Y: 12, Z: 330}, {X: -300, Y: 18, Z: 610}, {X: -20, Y: 10, Z: 760},
	{X: 260, Y: 12, Z: 620}, {X: 520, Y: 18, Z: 520}, {X: 800, Y: 30, Z: 650},
	{X: 1020, Y: 18, Z: 300}, {X: 1080, Y: 0, Z: 20}, {X: 980, Y: 20, Z: -350},
	{X: 780, Y: 8, Z: -500}, {X: 500, Y: 15, Z: -300}, {X: 260, Y: 8, Z: -120},
	{X: 20, Y: 5, Z: 40}, {X: -180, Y: 9, Z: 80}, {X: 60, Y: 15, Z: 260},
	{X: 360, Y: 10, Z: 280}, {X: 600, Y: 12, Z: 260}, {X: 760, Y: 10, Z: 180},
	{X: 440, Y: 20, Z: 470}, {X: 180, Y: 12, Z: 560}, {X: -80, Y: 20, Z: 520},
	{X: -260, Y: 14, Z: 400}, {X: -120, Y: 5, Z: -180}, {X: 40, Y: 8, Z: -360},
	{X: 300, Y: 10, Z: -500}, {X: 620, Y: 16, Z: -420}, {X: 880, Y: 12, Z: -620},
	{X: -600, Y: 15, Z: 620}, {X: -700, Y: 10, Z: 200}, {X: -680, Y: 8, Z: -180},
	{X: -500, Y: 6, Z: -500}, {X: 950, Y: 6, Z: 700}, {X: 200, Y: 4, Z: 900},
}

// --- admission ---------------------------------------------------------------
//
// BR reuses the join HANDSHAKE (cmd 100 / 101 / 130 / 176) but none of the Clash Squad world
// setup: no arena city, no filler bot, no shop, no round. Players gather in a small lobby area
// during brWaiting and are teleported to their drop point when the match starts.

// setupBattleRoyale initialises a fresh BR match's world for its first player.
//
// The waiting area is a REAL CS arena, not the invented BR lobby coordinates. Comparing a working
// Clash Squad join against a stuck BR join, the only world setup BR was missing was the surveyed
// gate spawn plus the cmd 457 zone index — and without them the client streamed in no region, so
// the player got an overhead camera and no pawn. The arena only decides where everyone WAITS; the
// drop still scatters the roster over brSpawnPoints when the match starts.
func (m *Match) setupBattleRoyale(s *session) {
	if m.arenasUsed == nil {
		m.arenasUsed = make(map[csArena]bool)
	}
	m.arena = choosePlayerSpawn(&s.player, m.arenasUsed)
	m.arenasUsed[m.arena] = true
	m.round = 1
	m.botEntity = 0
	m.addPlayer(s)
	if cfg.spawn == nil { // re-anchor to the gate for the faction addPlayer just assigned
		s.player.SpawnPos, s.player.SpawnFace = m.arena.spawnFor(s.faction)
	}
	s.playerPos = s.player.SpawnPos
	log.Printf("[mm-udp] BR waiting area: city=%q zone=%d pos=(%.1f,%.1f,%.1f)",
		m.arena.City, m.arena.ZoneID, s.playerPos.X, s.playerPos.Y, s.playerPos.Z)
}

// admitFirstBattleRoyale is admitFirst's BR path: join handshake + lobby spawn, then run() starts
// the BR lifecycle (startBattleRoyaleLifecycle) instead of the CS phase machine.
func (m *Match) admitFirstBattleRoyale(s *session) {
	m.loadMatchSettings(s.player.MatchID)
	m.settings.noLoadout = true // BR: drop in with fists + medkits; every weapon comes from ground loot
	m.setupBattleRoyale(s)

	s.sendDataLog(packet.CmdJoinMatchRes, message.JoinMatchRes(0, m.settings.roomSetting, m.settings.roomSetting2), "cmd=100 join res (BR)")
	s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(s.player, m.serverTick()),
		fmt.Sprintf("cmd=101 BR self acc=%d ent=%#x %q", s.player.AccountID, s.player.EntityID, s.player.Name))
	s.holdOnIsland = brHoldsOnIsland() // stay on the waiting island until the countdown ends
	s.sendJoinTail("(BR)")
	s.sendDataLog(packet.CmdSyncRegionActivities, message.SyncRegionActivitiesDefault(),
		"cmd=176 SYNC_REGION_ACTIVITIES (DEFAULT) — dynamic-prefab visibility")

	// Everything below this line is what Clash Squad's admitFirst does and BR used to skip. Each of
	// these is part of what makes the client materialise the local pawn instead of parking the
	// camera over the map: the zone circle, the region index that names the city, the PRI binding
	// that maps RepIDs to entities, and the starting inventory the pawn renders holding.
	s.broadcastZone()      // cmd 117 — a circle to stand in
	m.broadcastZoneIndex() // cmd 457 — tells the client WHICH region to stream in (default 99 = miss)
	m.bindAll()            // cmd 118 — without it the cmd 900/901 PRI stream has no pawn to land on
	s.giveLoadout(true)    // cmd 174 + 124 — noLoadout is set for BR, so fists + medkits only

	s.startCSMatch() // starts run(), which branches to the BR lifecycle
}

// admitLaterBattleRoyale admits a 2nd+ player onto a BR match that is still gathering. It runs on
// run()'s goroutine (enqueued by admit), so mutating the roster here is safe.
func (m *Match) admitLaterBattleRoyale(s *session) {
	m.addPlayer(s)
	s.player.SpawnPos, s.player.SpawnFace = battleRoyaleLobbySpawn(s.slot)
	s.playerPos = s.player.SpawnPos
	m.hp[s.entityID] = m.settings.maxHP
	m.life[s.entityID] = lifeAlive

	s.sendDataLog(packet.CmdJoinMatchRes, message.JoinMatchRes(0, m.settings.roomSetting, m.settings.roomSetting2), "cmd=100 join res (BR, later)")
	s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(s.player, m.serverTick()), // own entity FIRST — the client adopts it as the local player
		fmt.Sprintf("cmd=101 BR self ent=%#x %q", s.player.EntityID, s.player.Name))
	for _, p := range m.players {
		if p != s {
			s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(p.player, m.serverTick()),
				fmt.Sprintf("cmd=101 BR other ent=%#x %q", p.player.EntityID, p.player.Name))
		}
	}
	for _, p := range m.players { // learn everyone's current inventory / mounted weapons
		if p != s {
			s.sendDataLog(packet.CmdSyncInventory, p.currentInventorySync(),
				fmt.Sprintf("cmd=174 existing ent=%#x inventory -> BR joiner", p.entityID))
			p.resendEquipTo(s)
			p.resendAttachTo(s)
		}
	}
	s.holdOnIsland = brHoldsOnIsland() && m.brPhase == brWaiting
	s.sendJoinTail("(BR later)")
	s.sendDataLog(packet.CmdSyncRegionActivities, message.SyncRegionActivitiesDefault(), "cmd=176 SYNC_REGION_ACTIVITIES (DEFAULT)")

	for _, p := range m.players { // everyone already in learns the joiner
		if p != s {
			p.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(s.player, m.serverTick()),
				fmt.Sprintf("cmd=101 BR new player ent=%#x %q -> %v", s.player.EntityID, s.player.Name, p.remote))
		}
	}

	m.bindAll()
	s.giveLoadout(true) // noLoadout is set for BR -> fists + medkits only
	// A player admitted after the drop (rejoin path) must still see the world that already exists:
	// the ground loot placed at match start and the current safe-zone circle. Without these the
	// joiner sees an empty map and no zone, then takes "invisible" out-of-zone damage.
	if m.brLootSeeded {
		m.sendBattleRoyaleLootTo(s)
	}
	if m.zoneOn {
		m.broadcastBattleRoyaleZoneTo(s)
	}
	log.Printf("[mm-udp] BR admitted player slot=%d ent=%#08x (roster=%d/%d) %v", s.slot, s.entityID, len(m.players), maxBRPlayers, s.remote)
}

// dropBattleRoyaleLoot spills an eliminated player's carried items onto the ground where they
// fell, so the kill is worth taking (BATTLE_ROYALE.md: "Eliminated players drop their carried
// loot"). It reuses the ordinary drop path, so the items merge into a nearby box and every client
// is told about them exactly as a manual cmd-112 drop would. The player's own inventory is then
// cleared, which also stops a spectating corpse from appearing to still carry a gun.
func (m *Match) dropBattleRoyaleLoot(victim uint32) {
	s := m.sessionByEntity(victim)
	if s == nil || len(s.clientUIDs) == 0 {
		return
	}
	pos := s.snapPlayerPos()
	if pos == (message.Vec3{}) {
		pos = s.player.SpawnPos
	}
	// Iterate in a stable order (map iteration is randomised) so the emitted packet order is
	// reproducible — it makes a desync in a capture readable.
	uniques := make([]uint32, 0, len(s.clientUIDs))
	for uid := range s.clientUIDs {
		uniques = append(uniques, uid)
	}
	sort.Slice(uniques, func(i, j int) bool { return uniques[i] < uniques[j] })

	var out []outPkt
	for _, uid := range uniques {
		it := s.clientUIDs[uid]
		if it.data == 0 { // the fist is fixed to the melee slot and is never dropped
			continue
		}
		out = s.dropToGround(it, pos, out)
	}
	s.clientUIDs = map[uint32]lootItem{}
	s.weapons = map[byte]csWeapon{}
	s.equipment = []message.Equipment{{Slot: message.SlotMelee, Data: 0, Unique: 0}}
	s.itemOnHand = 0
	s.flush(out)
	log.Printf("[mm-udp] BR death drop ent=%#x items=%d at (%.1f,%.1f,%.1f)", victim, len(uniques), pos.X, pos.Y, pos.Z)
}

// battleRoyaleResults emits the per-player cmd 103 for a finished BR match. It cannot use the CS
// matchEnd(), which ranks by m.winnerTeam: in BR every player is its own team, so a team compare
// would hand the win to whoever holds that team id rather than to the last survivor.
func (m *Match) battleRoyaleResults() {
	for _, p := range m.players {
		rank, result := uint16(2), "LOSE"
		if p.entityID == m.brWinner {
			rank, result = 1, "WIN"
		}
		p.sendDataLog(packet.CmdMatchEnd, message.MatchEnd(rank),
			fmt.Sprintf("cmd=103 BR MatchEnd rank=%d %s ent=%#x kills=%d roster=%d",
				rank, result, p.entityID, m.kills[p.entityID], len(m.players)))
	}
	m.publishResult() // durable settlement to the bus, if enabled
}

// battleRoyaleLobbySpawn keeps players together in a harmless pre-match area.
//
// The point MUST be one the game's own geometry vouches for. An invented coordinate
// lands the pawn inside terrain: the world renders, but the player is stuck inside a
// hill with a fixed camera and cannot move — indistinguishable from "the match never
// started". brSpawnPoints[0] is the factory gate, the same position the verified Clash
// Squad arena uses (csArenas[0] in spawns.go), so it is known-good ground. Players are
// ringed around it and lifted slightly so no two pawns start inside each other.
func battleRoyaleLobbySpawn(slot byte) (message.Vec3, message.Vec3) {
	base := brSpawnPoints[0]
	i := float64(slot - 1)
	ang := i * 2.3999632297 // golden angle — spreads slots evenly around the ring
	r := 4.0 + math.Mod(i, 6)*2.5
	return message.Vec3{
		X: base.X + math.Cos(ang)*r,
		Y: base.Y + 1.0,
		Z: base.Z + math.Sin(ang)*r,
	}, message.Vec3{X: 0, Y: 0, Z: 1}
}

// battleRoyaleSpawn is the actual ground drop position.
func battleRoyaleSpawn(slot byte) (message.Vec3, message.Vec3) {
	// Always a LISTED point (wrapping when there are more players than points). The
	// previous overflow branch invented a coordinate on a ring around the zone centre,
	// which can land inside terrain or water — the pawn then spawns stuck with a fixed
	// camera. Wrapping puts two players near each other at worst, which BR tolerates.
	base := brSpawnPoints[int(slot-1)%len(brSpawnPoints)]
	ang := brRand.Float64() * 2 * math.Pi
	r := 4.0 + brRand.Float64()*10.0
	pos := message.Vec3{X: base.X + math.Cos(ang)*r, Y: base.Y + 1.5, Z: base.Z + math.Sin(ang)*r}
	face := message.Vec3{X: 0, Y: 0, Z: 1}
	return pos, face
}

func (m *Match) battleRoyaleZoneGeometry() (message.Vec3, float64) {
	return message.Vec3{X: brCenterX, Y: 0, Z: brCenterZ}, brOuterRadius
}

// startBattleRoyaleLifecycle is the BR-only match state machine bootstrap.
// The match remains admit-able during the gathering countdown; once it flips to
// Fight, the manager locks it and no CS waiting/buy/round code is entered.
func (m *Match) startBattleRoyaleLifecycle(now time.Time) {
	m.brPhase = brWaiting
	// The CS-facing phase matters even though BR has no rounds: the GRI phase is what decides whether
	// the client leaves the pawn FROZEN. phSetupWait maps to CSPhaseWaiting ("waiting for players"),
	// which holds the camera and never hands over control — that was the overhead-map symptom. phPrepare
	// is the unfrozen-at-the-gate phase, and m.started=true keeps matchState from raising the waiting
	// overlay on top of it. BR's own gathering countdown still runs off m.deadline exactly as before.
	m.started = true
	wait := brWaitDuration
	if v := os.Getenv("MATCH_BR_WAIT"); v != "" { // seconds; lets a solo tester linger in the waiting area
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			wait = time.Duration(secs) * time.Second
		}
	}
	m.enter(now, phPrepare, wait)
	m.brLootSeeded = false
	log.Printf("[mm-udp] BR lifecycle started: gathering up to %d players for %v", maxBRPlayers, wait)
}

func (m *Match) startBattleRoyaleFight(now time.Time) {
	m.started = true
	m.brPhase = brFight
	m.brRoster = len(m.players) // remember the size at the drop — see checkBattleRoyaleEnd
	m.enter(now, phFight, holdDur)

	// Record the configured flight path independently from landing and loot points.
	// The 1.70.1 aircraft wire payload is not verified in this project, so this route
	// calculation is diagnostic/server-side only; the verified teleport remains the
	// authoritative landing operation until a real client flight packet is confirmed.
	log.Printf("[mm-udp] BR airplane route configured start=(%.1f,%.1f,%.1f) end=(%.1f,%.1f,%.1f) duration=%s yaw=%.3f",
		brAirplaneRoute.Start.X, brAirplaneRoute.Start.Y, brAirplaneRoute.Start.Z,
		brAirplaneRoute.End.X, brAirplaneRoute.End.Y, brAirplaneRoute.End.Z,
		brAirplaneRoute.Duration, brAirplaneRoute.headingRadians())

	// Leave the BR waiting area and perform the authoritative drop. The verified
	// teleport is retained; do not invent a flight packet that could crash the client.
	for _, p := range m.players {
		pos, face := battleRoyaleSpawn(p.slot)
		p.player.SpawnPos, p.player.SpawnFace = pos, face
		p.playerPos = pos
		// Anyone still held on the waiting island leaves it NOW: cmd 110 makes the client load the
		// battle map, so the spawn transform above is what it loads into.
		p.sendBattleStart("(BR drop)")
		m.tpSeq++
		p.sendData(packet.CmdTeleport, message.ForceTeleport(p.entityID, m.tpSeq, pos, face, 0))
	}

	if !m.brLootSeeded {
		m.broadcastBattleRoyaleLoot()
		m.brLootSeeded = true
	}
	m.startBattleRoyaleZone()
	log.Printf("[mm-udp] BR started: players=%d (waiting area -> random drop)", len(m.players))
}

func (m *Match) advanceBattleRoyale(now time.Time) {
	switch m.brPhase {
	case brWaiting:
		// Start early once the lobby is full — otherwise 48 players sit through the rest of the
		// gathering countdown while nobody else can possibly join (canAdmit is already false).
		if len(m.players) < maxBRPlayers && now.Before(m.deadline) {
			return
		}
		// BR never falls back to the CS bot. One player is allowed for private testing,
		// but the participant is still a BR player and all BR rules remain active.
		m.startBattleRoyaleFight(now)
	case brFight:
		m.stepBattleRoyaleZone(now)
		m.checkBattleRoyaleEnd()
	case brMatchEnd:
		if !m.brResultsSent {
			if m.deadline.IsZero() || now.Before(m.deadline) {
				return
			}
			m.battleRoyaleResults()
			m.ended = true
			m.brResultsSent = true
			m.enter(now, phMatchEnd, matchStatsHold)
			m.deadline = now.Add(matchStatsHold)
			return
		}
		if now.Before(m.deadline) {
			return
		}
		if !m.tearingDown {
			m.tearingDown = true
			close(m.done)
		}
	}
}

func (m *Match) startBattleRoyaleZone() {
	center, outer := m.battleRoyaleZoneGeometry()
	m.zoneCenter = center
	m.zoneOuterR = outer
	m.zoneInnerR = outer
	m.zoneStage = 1
	m.zoneWaitEnd = time.Now().Add(brZoneStageWait)
	m.zoneShrink = false
	m.zoneShrinkStart = time.Time{}
	m.zoneLastDmg = time.Now()
	m.zoneOn = true
	m.broadcastBattleRoyaleZone()
}

func (m *Match) broadcastBattleRoyaleZone() {
	for _, p := range m.players {
		m.broadcastBattleRoyaleZoneTo(p)
	}
}

func (m *Match) broadcastBattleRoyaleZoneTo(p *session) {
	if p == nil || p.out == nil {
		return
	}
	nowMs := uint32(time.Since(m.matchStart).Milliseconds())
	if m.zoneShrink {
		endMs := nowMs + uint32(brShrinkDuration.Milliseconds())
		body := message.SafeZoneChange(byte(m.zoneStage), m.zoneCenter, m.zoneOuterR, m.zoneCenter, m.zoneInnerR, message.ZoneShrinking, nowMs, endMs)
		p.sendData(packet.CmdSafeZoneChange, body)
		return
	}
	endMs := nowMs + uint32(time.Until(m.zoneWaitEnd).Milliseconds())
	if int32(endMs-nowMs) < 0 {
		endMs = nowMs
	}
	body := message.SafeZoneChange(byte(m.zoneStage), m.zoneCenter, m.zoneOuterR, m.zoneCenter, m.zoneOuterR, message.ZoneWaiting, nowMs, endMs)
	p.sendData(packet.CmdSafeZoneChange, body)
}

func (m *Match) stepBattleRoyaleZone(now time.Time) {
	if !m.zoneOn || m.brPhase != brFight {
		return
	}

	// Four BR stages: 1=1350m, 2=900m, 3=550m, 4=250m, then the final
	// radius is held. The 3-minute timer is the time spent at each stage.
	stageRadii := [...]float64{1350, 900, 550, 250}

	if !m.zoneShrink && !now.Before(m.zoneWaitEnd) && m.zoneStage < 4 {
		nextRadius := stageRadii[m.zoneStage] // zoneStage is 1-based here
		nowMs := uint32(time.Since(m.matchStart).Milliseconds())
		endMs := nowMs + uint32(brShrinkDuration.Milliseconds())
		body := message.SafeZoneChange(byte(m.zoneStage+1), m.zoneCenter, m.zoneOuterR, m.zoneCenter, nextRadius, message.ZoneShrinking, nowMs, endMs)
		m.broadcastData(packet.CmdSafeZoneChange, body, fmt.Sprintf("BR SafeZone stage %d->%d r=%.0f->%.0f", m.zoneStage, m.zoneStage+1, m.zoneOuterR, nextRadius))
		m.zoneShrink = true
		m.zoneShrinkStart = now
		m.zoneInnerR = nextRadius
	}

	if m.zoneShrink {
		elapsed := now.Sub(m.zoneShrinkStart)
		t := elapsed.Seconds() / brShrinkDuration.Seconds()
		if t >= 1 {
			if m.zoneStage < 4 {
				m.zoneStage++
			}
			m.zoneOuterR = m.zoneInnerR
			m.zoneShrink = false
			m.zoneShrinkStart = time.Time{}
			m.zoneWaitEnd = now.Add(brZoneStageWait)
			m.broadcastBattleRoyaleZone()
		} else {
			radius := m.zoneOuterR + (m.zoneInnerR-m.zoneOuterR)*t
			m.zoneCurrentR = radius
		}
	} else {
		m.zoneCurrentR = m.zoneOuterR
	}

	if now.Sub(m.zoneLastDmg) < brDamageEvery {
		return
	}
	m.zoneLastDmg = now
	radius := m.damageRadius(now)
	for _, p := range m.players {
		if m.lifeOf(p.entityID) != lifeAlive {
			continue
		}
		if dist2D(p.playerPos, m.zoneCenter) > radius {
			m.applyDamage(p.entityID, 0, brDamage, 0)
		}
	}
}

// damageRadius is the radius the out-of-zone check uses this tick. While the circle is closing it
// is evaluated zoneClientLag in the PAST, exactly like the Clash Squad zone does: the client lerps
// its own circle off a local sim clock that trails ours by the network latency plus sim jitter, so
// its VISIBLE circle is always a little larger than a same-instant lerp. Without this the BR zone
// damages players who are still standing inside the circle they can see — the single most common
// "the zone is killing me for no reason" report. Never smaller than the final radius.
func (m *Match) damageRadius(now time.Time) float64 {
	if !m.zoneShrink {
		if m.zoneOuterR > 0 {
			return m.zoneOuterR
		}
		return brOuterRadius
	}
	elapsed := now.Sub(m.zoneShrinkStart) - zoneClientLag
	t := elapsed.Seconds() / brShrinkDuration.Seconds()
	if t <= 0 {
		return m.zoneOuterR
	}
	if t >= 1 {
		return m.zoneInnerR
	}
	return m.zoneOuterR + (m.zoneInnerR-m.zoneOuterR)*t
}

func (m *Match) checkBattleRoyaleEnd() {
	if !m.battleRoyale || m.brPhase != brFight || m.brWinner != 0 {
		return
	}
	// A one-player BR is a valid private/test session. Do not immediately declare
	// that player the winner and return them to the lobby; this was the main cause
	// of the apparent "automatic elimination" when testing BR alone.
	//
	// The check is against the roster AT THE DROP, not the CURRENT roster: with len(m.players)
	// a real match whose last two players quit would drop to 1 and hang forever instead of
	// crowning the survivor. brRoster is fixed once the match starts, so only a genuine solo
	// test session (dropped alone) stays open-ended.
	if m.brRoster < 2 {
		return
	}
	alive := 0
	var winner uint32
	for _, p := range m.players {
		if m.lifeOf(p.entityID) == lifeAlive {
			alive++
			winner = p.entityID
		}
	}
	if alive == 1 {
		m.brWinner = winner
		m.winnerTeam = 1
		m.stopZone()
		m.brPhase = brMatchEnd
		m.enter(time.Now(), phMatchEndPause, matchEndPause)
		log.Printf("[mm-udp] BR winner ent=%#x roster=%d", winner, len(m.players))
	} else if alive == 0 {
		m.brWinner = ^uint32(0)
		m.winnerTeam = 0
		m.stopZone()
		m.brPhase = brMatchEnd
		m.enter(time.Now(), phMatchEndPause, matchEndPause)
	}
}

// BR uses the same ground-loot wire protocol as the existing world-loot system,
// but the table is owned only by this BR match and is never reset by CS rounds.
func (m *Match) broadcastBattleRoyaleLoot() {
	// Build a dense, deterministic set of map locations from the real BR spawn
	// locations, then randomise the locations and the item assigned to each one.
	// This gives every match a different loot layout without placing loot outside
	// the known playable area.
	points := make([]message.Vec3, 0, len(brSpawnPoints)*3)
	for _, base := range brSpawnPoints {
		for n := 0; n < 3; n++ {
			p := base
			if n != 0 {
				ang := brRand.Float64() * 2 * math.Pi
				r := 8.0 + brRand.Float64()*22.0
				p.X += math.Cos(ang) * r
				p.Z += math.Sin(ang) * r
			}
			points = append(points, p)
		}
	}
	brRand.Shuffle(len(points), func(i, j int) { points[i], points[j] = points[j], points[i] })

	// Guaranteed categories: many weapons + ammo, medkits, armor/helmets and gloo walls.
	// The IDs are verified against protocol/ItemTable.csv in this project.
	guns := []uint32{2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 18, 19, 20, 21, 23, 24, 25, 32, 33, 41, 43}
	ammo := []uint32{201, 202, 203, 204, 205}
	gear := []uint32{102, 102, 102, 102, 301, 302, 303, 304, 305, 306, 1201, 1201, 1201, 1201}

	items := make([]uint32, 0, len(points))
	items = append(items, guns...)
	items = append(items, ammo...)
	items = append(items, gear...)
	for len(items) < len(points) {
		switch brRand.Intn(10) {
		case 0, 1:
			items = append(items, guns[brRand.Intn(len(guns))])
		case 2:
			items = append(items, 102)
		case 3:
			items = append(items, 1201)
		case 4, 5:
			items = append(items, ammo[brRand.Intn(len(ammo))])
		case 6:
			items = append(items, 301+uint32(brRand.Intn(6)))
		default:
			items = append(items, 201+uint32(brRand.Intn(5)))
		}
	}
	brRand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })

	if m.containers == nil {
		m.containers = map[uint16]*container{}
	}
	for i, pos := range points {
		data := items[i]
		count := uint32(1)
		runtime := uint32(0)
		switch {
		case data == 102:
			count, runtime = 2, 2
		case isAmmoData(data):
			count, runtime = 30, 30
		case data == 1201:
			count, runtime = 2, 0
		case data >= 301 && data <= 306:
			count, runtime = 1, 0
		}
		uid := uint32(50000 + i + 1)
		id := uint16(uint32(runtimeContainerBase) + uint32(i))
		if id > runtimeContainerMax {
			break
		}
		item := lootItem{unique: uid, data: data, count: count, runtime: runtime}
		box := &container{id: id, pos: pos, ctype: message.ContainerDynamic, items: []lootItem{item}}
		m.containers[id] = box
		m.broadcastData(packet.CmdAddPickup, message.AddPickup(item.pickupItem(box.id), pos, box.ctype),
			fmt.Sprintf("BR map loot data=%d uid=%d count=%d pos=(%.1f,%.1f,%.1f)", data, uid, count, pos.X, pos.Y, pos.Z))
	}
	log.Printf("[mm-udp] BR map loot seeded: %d pickup locations", len(m.containers))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Match) sendBattleRoyaleLootTo(s *session) {
	if s == nil || s.out == nil {
		return
	}
	for _, box := range m.containers {
		for _, item := range box.items {
			s.sendData(packet.CmdAddPickup, message.AddPickup(item.pickupItem(box.id), box.pos, box.ctype))
		}
	}
}
