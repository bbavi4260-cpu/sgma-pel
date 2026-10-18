package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

// Server-driven bots. A bot is a normal remote Player entity on the client, but bots run no client AI, so
// the server (a) streams their movement like a human's (cmd 1001 / 1012 state sync), (b) reports their hits
// itself (applyDamage with attacker=bot entity). Runs on the match loop (run()), only during the Fight phase
// while the bot is alive.
//
// Teams (4v4 vs a lone human): the PRIMARY bot (m.bot / m.botEntity, enemy team 2) plus extras — 3 more enemy
// bots and 3 ALLY bots on the human's team 1. Allies shoot enemies, enemies shoot the human and the allies, and
// ally bots revive a knocked human teammate. MATCH_BOT_ALLIES / MATCH_BOT_ENEMIES (default 3 / 3 extras)
// change the numbers (0 disables).
//
// Tunables (env): MATCH_BOT_SPEED (m/s, 4.5), MATCH_BOT_DAMAGE (HP per hit, 10), MATCH_BOT_RANGE (m, 14),
// MATCH_BOT_FIRE_MS (700), MATCH_BOT_ACCURACY (0..1, 0.4), MATCH_BOT_PASSIVE=1 (disable the AI).

type botAI struct {
	init        bool
	pos         message.Vec3
	face        message.Vec3 // last facing (unit, XZ)
	lastStep    time.Time
	lastShot    time.Time
	strafe      float64 // +1/-1 lateral direction while in range
	strafeAt    time.Time
	moving      bool
	firingUntil time.Time // PRI fire-state window (muzzle flash on the client)
	firing      bool

	// "Real player" behaviour state.
	lastHP     uint16    // HP at the previous step (a drop = we are being shot)
	glooLeft   int       // gloo walls still carried this round
	nextWallAt time.Time // cooldown between wall placements
	coverUntil time.Time // keep backing behind our own wall until then
	sawTarget  bool      // had line of sight last step (reaction-time edge)
	reactAt    time.Time // earliest first shot after spotting a target
	burstLeft  int       // shots left in the current burst
	pauseUntil time.Time // gap between bursts
	mag        int       // bullets left in the magazine
	reloadTill time.Time // reloading until
	wallShotAt time.Time // last time we shot at a blocking wall
}

// botUnit is one server-driven participant. The primary enemy bot's identity lives in Match.bot; its unit is
// refreshed from it by botList().
type botUnit struct {
	ent        uint32
	team       byte
	repID      uint32
	faction    byte
	pl         joinPlayer
	ai         botAI
	weaponData uint32
	weaponUID  uint32
	idx        int // 0-based index within its group (spread / spacing)
}

// botShared is bot state common to every unit.
type botShared struct {
	idleState, runState byte
	learnedIdle         bool
	learnedRun          bool
	moveTick0           int32
	seqN                uint32
}

const (
	botWeaponData   = 6      // AK (primary bot's weapon)
	botWeaponUnique = 900001 // arbitrary unique outside the players' uid range
)

var botPassive = os.Getenv("MATCH_BOT_PASSIVE") == "1"

var envCache = map[string]float64{}

func envFloat(key string, def float64) float64 {
	if v, ok := envCache[key]; ok {
		return v
	}
	v := envFloatRaw(key, def)
	envCache[key] = v
	return v
}

func envFloatRaw(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// botList returns every bot in the match: the primary enemy bot (if any) then the extras.
func (m *Match) botList() []*botUnit {
	var out []*botUnit
	if m.botEntity != 0 {
		u := &m.primary
		u.ent, u.team, u.repID, u.faction = m.botEntity, enemyTeamID, botRepID, botFaction
		u.pl = m.bot
		u.weaponData, u.weaponUID = botWeaponData, botWeaponUnique
		out = append(out, u)
	}
	return append(out, m.extras...)
}

func (m *Match) botByEntity(ent uint32) *botUnit {
	for _, u := range m.botList() {
		if u.ent == ent {
			return u
		}
	}
	return nil
}

func (m *Match) isBot(ent uint32) bool { return m.botByEntity(ent) != nil }

// seedBots sets every bot to full HP + ALIVE.
func (m *Match) seedBots() {
	for _, u := range m.botList() {
		m.hp[u.ent] = m.settings.maxHP
		m.life[u.ent] = lifeAlive
	}
}

// spawnExtraBots creates the extra enemy + ally bots (called right after the primary bot spawns) and sends
// their cmd 101 joins. Idempotent.
func (m *Match) spawnExtraBots() {
	if len(m.extras) > 0 || m.botEntity == 0 {
		return
	}
	allies, enemies := envInt("MATCH_BOT_ALLIES", 0), envInt("MATCH_BOT_ENEMIES", 0)
	weapons := []uint32{2, 12, 33, 24, 46, 6} // M4A1 / SCAR / AN94 / FAMAS / AUG / AK
	mk := func(team byte, i int, name string, faction byte) *botUnit {
		slot := uint32(0x0B + len(m.extras))
		u := &botUnit{
			ent: uint32(team)<<24 | slot, team: team, repID: 1010 + uint32(len(m.extras)), faction: faction, idx: i,
			weaponData: weapons[len(m.extras)%len(weapons)], weaponUID: botWeaponUnique + 1 + uint32(len(m.extras)),
		}
		u.pl.AccountID = botAccountID + 1 + uint64(len(m.extras))
		u.pl.EntityID = u.ent
		u.pl.Name = name
		u.pl.TeamIdx = byte(i + 1)
		return u
	}
	for i := 0; i < allies; i++ {
		m.extras = append(m.extras, mk(localTeamID, i, fmt.Sprintf("ALLY%d", i+1), localFaction))
	}
	for i := 0; i < enemies; i++ {
		m.extras = append(m.extras, mk(enemyTeamID, i, fmt.Sprintf("BOT%d", i+2), botFaction))
	}
	m.placeExtras()
	for _, u := range m.extras {
		m.hp[u.ent] = m.settings.maxHP
		m.life[u.ent] = lifeAlive
		m.broadcastData(packet.CmdPlayerJoin, message.PlayerJoin(u.pl, m.serverTick()),
			fmt.Sprintf("cmd=101 EXTRA BOT ent=%#x %q team=%d pos=(%.1f,%.1f,%.1f)", u.ent, u.pl.Name, u.team, u.pl.SpawnPos.X, u.pl.SpawnPos.Y, u.pl.SpawnPos.Z))
	}
	log.Printf("[mm-udp] extra bots: %d allies + %d enemies (plus the primary enemy bot)", allies, enemies)
}

// placeExtras (re)computes the extras' spawn positions for the current round: allies in a line beside the
// human, enemies spread around the primary bot, everyone facing the other side.
func (m *Match) placeExtras() {
	if len(m.extras) == 0 {
		return
	}
	hp, hf := m.s.player.SpawnPos, m.s.player.SpawnFace
	if hf.X == 0 && hf.Z == 0 {
		hf = message.Vec3{Z: 1}
	}
	px, pz := -hf.Z, hf.X // perpendicular
	allyOff := []float64{-2.5, 2.5, 5}
	enemyOff := []float64{-3, 3, 6}
	for _, u := range m.extras {
		i := u.idx % 3
		if u.team == localTeamID {
			u.pl.SpawnPos = message.Vec3{X: hp.X + px*allyOff[i] - hf.X*1.0, Y: hp.Y, Z: hp.Z + pz*allyOff[i] - hf.Z*1.0}
			u.pl.SpawnFace = hf
		} else {
			bp := m.bot.SpawnPos
			u.pl.SpawnPos = message.Vec3{X: bp.X + px*enemyOff[i] + hf.X*1.5, Y: bp.Y, Z: bp.Z + pz*enemyOff[i] + hf.Z*1.5}
			u.pl.SpawnFace = faceToward(u.pl.SpawnPos, hp)
		}
	}
}

// retireExtraBots removes the extras (a 2nd human joined -> human vs human).
func (m *Match) retireExtraBots() {
	for _, u := range m.extras {
		m.broadcastData(packet.CmdDead, message.PlayerDead(u.ent, 0, 0, 0, 0, 0, u.pl.SpawnPos, true, false),
			fmt.Sprintf("cmd=107 EXTRA BOT retired ent=%#x", u.ent))
		delete(m.hp, u.ent)
		delete(m.life, u.ent)
		delete(m.kills, u.ent)
		delete(m.deaths, u.ent)
		delete(m.damage, u.ent)
	}
	m.extras = nil
}

// sendBotJoins sends every bot's cmd 101 to one session.
func (m *Match) sendBotJoins(s *session, what string) {
	for _, u := range m.botList() {
		s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(u.pl, m.serverTick()),
			fmt.Sprintf("cmd=101 %s bot ent=%#x %q", what, u.ent, u.pl.Name))
	}
}

// respawnBots re-sends each bot's join + teleport for the new round (same entities / RepIDs).
func (m *Match) respawnBots() {
	for _, u := range m.botList() {
		if m.rejoinBots == nil || m.rejoinBots[u.ent] { // an ALIVE bot already has its pawn: re-joining it can glitch/crash the client
			m.broadcastData(packet.CmdPlayerJoin, message.PlayerJoin(u.pl, m.serverTick()),
				fmt.Sprintf("cmd=101 BOT respawn ent=%#x pos=(%.1f,%.1f,%.1f)", u.ent, u.pl.SpawnPos.X, u.pl.SpawnPos.Y, u.pl.SpawnPos.Z))
		}
		m.tpSeq++
		m.broadcastData(packet.CmdTeleport, message.ForceTeleport(u.ent, m.tpSeq, u.pl.SpawnPos, u.pl.SpawnFace, 0), "")
	}
}

// learnMoveStates samples a human's latest state body: velocity bytes [22:28], moveState [28].
func (m *Match) learnMoveStates() {
	sh := &m.bshared
	for _, p := range m.players {
		if len(p.moveBody) != moveBodyLen {
			continue
		}
		vx := float64(int16(binary.LittleEndian.Uint16(p.moveBody[22:]))) / 100
		vz := float64(int16(binary.LittleEndian.Uint16(p.moveBody[26:]))) / 100
		speed := math.Hypot(vx, vz)
		st := p.moveBody[28]
		if speed < 0.2 && !sh.learnedIdle {
			sh.idleState, sh.learnedIdle = st, true
		} else if speed > 2.5 && speed < 8 && !sh.learnedRun {
			sh.runState, sh.learnedRun = st, true
		}
	}
}

// target is something a bot can shoot: a human or an opposing bot.
type botTarget struct {
	ent  uint32
	pos  message.Vec3
	dist float64
}

func (m *Match) stepBot(now time.Time) {
	if botPassive {
		return
	}
	units := m.botList()
	if len(units) == 0 {
		return
	}
	fighting := m.phase == phFight && !m.roundOver
	var base uint32
	for _, p := range m.players {
		if p.moveBase != 0 {
			base = p.moveBase
			break
		}
	}
	m.learnMoveStates()

	var poses []botPose
	pushPRI := false
	for _, u := range units {
		ai := &u.ai
		if !fighting || m.lifeOf(u.ent) != lifeAlive {
			if ai.firing {
				ai.firing = false
				pushPRI = true
			}
			ai.init = false
			continue
		}
		if base == 0 {
			continue
		}
		if !ai.init {
			ai.init = true
			ai.firing = false
			ai.pos, ai.face = u.pl.SpawnPos, u.pl.SpawnFace
			ai.lastStep, ai.lastShot, ai.strafeAt = now, now.Add(time.Duration(rand.Intn(600))*time.Millisecond), now
			ai.glooLeft, ai.mag, ai.lastHP = 3, 30, m.hp[u.ent]
			ai.nextWallAt = now.Add(time.Duration(2000+rand.Intn(2000)) * time.Millisecond)
			ai.burstLeft, ai.coverUntil, ai.sawTarget = 0, time.Time{}, false
			ai.strafe = 1
			if rand.Intn(2) == 0 {
				ai.strafe = -1
			}
			log.Printf("[mm-udp] BOT AI active ent=%#x %q team=%d", u.ent, u.pl.Name, u.team)
		}
		dt := now.Sub(ai.lastStep).Seconds()
		if dt > 0.5 {
			dt = 0.5
		}
		ai.lastStep = now

		// Opponents: humans of the other team + other-team bots. Prefer a target we can SEE (no gloo wall in
		// between); if none is visible fall back to the nearest one and go around / shoot through the wall.
		var tgt *botTarget
		seen := false
		consider := func(ent uint32, pos message.Vec3) {
			d := math.Hypot(pos.X-ai.pos.X, pos.Z-ai.pos.Z)
			vis := m.wallBetween(ai.pos.X, ai.pos.Z, pos.X, pos.Z) == nil
			if tgt == nil || (vis && !seen) || (vis == seen && d < tgt.dist) {
				tgt = &botTarget{ent: ent, pos: pos, dist: d}
				seen = vis
			}
		}
		for _, p := range m.players {
			if p.team != u.team && m.lifeOf(p.entityID) == lifeAlive {
				consider(p.entityID, p.playerPos)
			}
		}
		for _, o := range units {
			if o.team != u.team && m.lifeOf(o.ent) == lifeAlive && o.ai.init {
				consider(o.ent, o.ai.pos)
			}
		}

		speed := envFloat("MATCH_BOT_SPEED", 4.5)
		rng := envFloat("MATCH_BOT_RANGE", 14)
		keep := math.Min(rng*0.6, 6+float64(u.idx)) // stop closing in here (spread so they don't stack)
		var mx, mz float64

		// Took damage since last step? (humans react: take cover / drop a wall.)
		curHP := m.hp[u.ent]
		hurt := curHP < ai.lastHP
		ai.lastHP = curHP

		// Reaction time: a freshly spotted target is not shot instantly.
		if tgt != nil && seen && !ai.sawTarget {
			ai.reactAt = now.Add(time.Duration(250+rand.Intn(350)) * time.Millisecond)
		}
		ai.sawTarget = tgt != nil && seen

		// Ally bots: revive a knocked human teammate first.
		var revive *session
		if u.team == localTeamID {
			for _, p := range m.players {
				if p.team == u.team && m.lifeOf(p.entityID) == lifeKnocked {
					revive = p
					break
				}
			}
		}
		if revive != nil {
			dx, dz := revive.playerPos.X-ai.pos.X, revive.playerPos.Z-ai.pos.Z
			d := math.Hypot(dx, dz)
			if d > 1.4 {
				mx, mz = dx/d, dz/d
				ai.face = message.Vec3{X: mx, Z: mz}
			} else {
				m.startRescue(u.ent, revive.entityID, uint64(time.Now().UnixMilli()))
			}
		} else if tgt != nil {
			dx, dz := tgt.pos.X-ai.pos.X, tgt.pos.Z-ai.pos.Z
			fx, fz := ai.face.X, ai.face.Z
			if tgt.dist > 0.01 {
				fx, fz = dx/tgt.dist, dz/tgt.dist
			}
			ai.face = message.Vec3{X: fx, Z: fz}

			// Real players drop a gloo wall when they are being shot (or sometimes just to push): place it
			// between us and the target, then back up behind it.
			if ai.glooLeft > 0 && seen && now.After(ai.nextWallAt) && tgt.dist > 3 &&
				(hurt || (tgt.dist < rng && rand.Intn(220) == 0)) {
				m.botPlaceWall(u, fx, fz)
				ai.glooLeft--
				ai.nextWallAt = now.Add(time.Duration(4000+rand.Intn(3000)) * time.Millisecond)
				ai.coverUntil = now.Add(time.Duration(1800+rand.Intn(1500)) * time.Millisecond)
			}

			switch {
			case now.Before(ai.coverUntil): // hide behind our wall, small sidesteps, keep shooting
				mx, mz = -fx*0.7-fz*ai.strafe*0.3, -fz*0.7+fx*ai.strafe*0.3
			case !seen: // a wall is in the way: flank around it instead of walking into it
				if now.After(ai.strafeAt) {
					ai.strafe = -ai.strafe
					ai.strafeAt = now.Add(time.Duration(2500+rand.Intn(2000)) * time.Millisecond)
				}
				mx, mz = fx*0.5-fz*ai.strafe, fz*0.5+fx*ai.strafe
			case tgt.dist > keep:
				mx, mz = fx, fz
			default:
				if now.After(ai.strafeAt) {
					ai.strafe = -ai.strafe
					ai.strafeAt = now.Add(time.Duration(800+rand.Intn(1200)) * time.Millisecond)
				}
				mx, mz = -fz*ai.strafe*0.5, fx*ai.strafe*0.5
				if tgt.dist < 2.5 {
					mx, mz = -fx*0.6, -fz*0.6
				}
			}
		}
		// Separation: push away from any other bot / human closer than 1.5 m.
		for _, o := range units {
			if o != u && o.ai.init {
				ox, oz := ai.pos.X-o.ai.pos.X, ai.pos.Z-o.ai.pos.Z
				if d := math.Hypot(ox, oz); d > 0.01 && d < 1.5 {
					mx += ox / d * 0.8
					mz += oz / d * 0.8
				}
			}
		}
		// Run slower while shooting (like a real player), never walk through a gloo wall: slide along it.
		if ai.firing {
			mx, mz = mx*0.7, mz*0.7
		}
		nx, nz := ai.pos.X+mx*speed*dt, ai.pos.Z+mz*speed*dt
		switch {
		case !m.insideAnyWall(nx, nz, 0.45):
		case !m.insideAnyWall(nx, ai.pos.Z, 0.45):
			nz, mz = ai.pos.Z, 0
		case !m.insideAnyWall(ai.pos.X, nz, 0.45):
			nx, mx = ai.pos.X, 0
		default:
			nx, nz, mx, mz = ai.pos.X, ai.pos.Z, 0, 0
		}
		ai.moving = mx != 0 || mz != 0
		ai.pos.X, ai.pos.Z = nx, nz
		if len(m.players) > 0 {
			ai.pos.Y = m.players[0].playerPos.Y // no terrain data server-side: follow the human's ground height
		}
		poses = append(poses, botPose{u, mx * speed, mz * speed})

		if ai.firing && now.After(ai.firingUntil) {
			ai.firing = false
			pushPRI = true
		}
		// Attack: bursts with pauses, a magazine + reload, reaction delay, and NO shooting through walls.
		if revive != nil || tgt == nil || tgt.dist > rng || now.Before(ai.reactAt) || now.Before(ai.reloadTill) {
			continue
		}
		if ai.mag <= 0 {
			ai.mag, ai.reloadTill = 30, now.Add(time.Duration(1800+rand.Intn(900))*time.Millisecond)
			continue
		}
		wall := m.wallBetween(ai.pos.X, ai.pos.Z, tgt.pos.X, tgt.pos.Z)
		if wall != nil { // blocked: sometimes shoot the wall to break it, like a player would
			if byte(wall.owner>>24) == u.team { // our own side's wall protects us — don't shoot it
				continue
			}
			if now.Sub(ai.wallShotAt) >= 600*time.Millisecond && math.Hypot(wall.pos.X-ai.pos.X, wall.pos.Z-ai.pos.Z) < 9 {
				ai.wallShotAt = now
				ai.firing, ai.firingUntil = true, now.Add(250*time.Millisecond)
				pushPRI = true
				ai.mag--
				m.hitWall(wall, uint16(envFloat("MATCH_BOT_DAMAGE", 10)*2))
			}
			continue
		}
		if ai.burstLeft <= 0 {
			if now.Before(ai.pauseUntil) {
				continue
			}
			ai.burstLeft = 3 + rand.Intn(4)
		}
		gap := time.Duration(envFloat("MATCH_BOT_FIRE_MS", 700)/3.5) * time.Millisecond // ~200 ms inside a burst
		if now.Sub(ai.lastShot) < gap {
			continue
		}
		ai.lastShot = now
		ai.burstLeft--
		ai.mag--
		if ai.burstLeft <= 0 {
			ai.pauseUntil = now.Add(time.Duration(450+rand.Intn(700)) * time.Millisecond)
		}
		ai.firing, ai.firingUntil = true, now.Add(250*time.Millisecond)
		pushPRI = true
		acc := envFloat("MATCH_BOT_ACCURACY", 0.3) * (1.2 - 0.6*tgt.dist/rng) // closer = more accurate
		if rand.Float64() < acc {
			dmg := uint16(envFloat("MATCH_BOT_DAMAGE", 10))
			part := uint8(2) // BODY
			if rand.Intn(8) == 0 {
				part, dmg = bodyHead, dmg*2
			}
			m.applyDamage(tgt.ent, u.ent, dmg, part)
		}
	}
	if pushPRI {
		m.pushPRI()
	}
	if len(poses) == 0 || base == 0 {
		return
	}
	m.relayBots(base, poses)
}

type botPose struct {
	u      *botUnit
	vx, vz float64
}

// relayBots streams every moving bot's pose in ONE batch, as both the cmd-1001 batch (same path humans use)
// and the compact cmd-1012 V2 batch (the form FF 1.70.1 applies to remote pawns), to every human.
func (m *Match) relayBots(base uint32, poses []botPose) {
	sh := &m.bshared
	matchTime := int32(time.Since(m.matchStart).Seconds() * 30)
	if sh.moveTick0 == 0 {
		sh.moveTick0 = matchTime
	}
	n := uint32(matchTime - sh.moveTick0)
	if n <= sh.seqN {
		n = sh.seqN + 1
	}
	sh.seqN = n
	seq := base + 100000 + n // distinct from the humans' seq, strictly increasing (1 per 30Hz client frame)

	i32 := func(b []byte, v float64) { binary.LittleEndian.PutUint32(b, uint32(int32(math.Round(v*1000)))) }
	i16 := func(b []byte, v float64) { binary.LittleEndian.PutUint16(b, uint16(int16(math.Round(v*100)))) }

	var e1 []message.MoveEntry
	var e2 []message.MoveEntryV2
	for _, ps := range poses {
		ai := &ps.u.ai
		state := sh.idleState
		if ai.moving {
			state = sh.runState
		}
		slot := byte(ps.u.ent & 0xFF)
		pos := make([]byte, 12)
		i32(pos[0:], ai.pos.X)
		i32(pos[4:], ai.pos.Y)
		i32(pos[8:], ai.pos.Z)
		vel := make([]byte, 6)
		i16(vel[0:], ps.vx)
		i16(vel[4:], ps.vz)
		body := make([]byte, moveBodyLen)
		copy(body[0:12], pos)
		i16(body[12:], ai.face.X) // moveDir
		i16(body[16:], ai.face.Z)
		copy(body[22:28], vel) // velocity
		body[28] = state
		i16(body[30:], ai.face.X) // aimDir
		i16(body[34:], ai.face.Z)
		e1 = append(e1, message.MoveEntry{Slot: slot, Body: body})
		yaw := angleByte(ai.face.X, ai.face.Z)
		e2 = append(e2, message.MoveEntryV2{Slot: slot, Pos: pos, Vel: vel, BodyYaw: yaw, AimYaw: yaw, MoveState: state})
	}
	b1001 := message.MoveBatch(matchTime, seq, e1)
	b1012 := message.MoveBatchV2(matchTime, seq, e2)
	for _, recv := range m.players {
		if recv.out == nil {
			continue
		}
		recv.out.send(&packet.Packet{SendOption: packet.SendUnreliable, Cmd: packet.CmdClientPos, Payload: b1001}, "")
		recv.out.send(&packet.Packet{SendOption: packet.SendUnreliable, Cmd: packet.CmdClientPosV2, Payload: b1012}, "")
	}
}

// pushPRI sends the current PRI block to every human right away (instead of waiting for the 300ms stream).
func (m *Match) pushPRI() {
	pl := m.priPayload()
	for _, p := range m.players {
		if p.out != nil {
			p.sendVar(packet.CmdPRISync, pl, 1)
		}
	}
}

func angleByte(fx, fz float64) byte {
	deg := math.Atan2(fx, fz) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return byte(int(deg*255/360) & 0xFF)
}

// botFireState is PRI field 21 for a bot: 2 (READY) while it is shooting so the client shows muzzle flash.
func (u *botUnit) fireState() byte {
	if u.ai.firing {
		return 2
	}
	return 0
}

// noteDeadBots records which bots are not alive right now (called just before the round-reset revive).
func (m *Match) noteDeadBots() {
	m.rejoinBots = map[uint32]bool{}
	for _, u := range m.botList() {
		if m.lifeOf(u.ent) != lifeAlive {
			m.rejoinBots[u.ent] = true
		}
	}
}

// botPlaceWall drops a gloo wall ~2 m in front of the bot, facing the target (fx,fz = unit XZ aim), exactly
// like a human's place request: spawn broadcast + FIFO cap to every client.
func (m *Match) botPlaceWall(u *botUnit, fx, fz float64) {
	pos := message.Vec3{X: u.ai.pos.X + fx*2.0, Y: u.ai.pos.Y, Z: u.ai.pos.Z + fz*2.0}
	half := math.Atan2(fx, fz) / 2
	rot := message.Quat{Y: math.Sin(half), W: math.Cos(half)}
	_, out := m.addWall(u.ent, pos, rot, m.lastWallType)
	m.flushBroadcast(out)
	m.pushPRI()
	log.Printf("[mm-udp] BOT %q placed a gloo wall at (%.1f,%.1f,%.1f)", u.pl.Name, pos.X, pos.Y, pos.Z)
}
