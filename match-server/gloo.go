package main

// GLOO WALL (ice wall) server logic for Contra Squad. A player buys a gloo wall
// (item DataID 1201, equipped in SlotBuilding=13) and PLACES it: the client sends a
// cmd 218 RUDP_ADD_ICEWALL place request (message::JEGADMHBFKB); the server spawns an
// authoritative wall, broadcasts it back (cmd 218 KACGKFMOHNP), and streams its state.
//
// Each wall has 200 HP and a crack STATE (intact -> cracked -> broken). It takes damage
// via cmd 219 (client reports the delta); the server subtracts HP and, at 0, removes it
// (cmd 221). After ~60s a wall force-breaks regardless of HP/state (the lifetime sweep).
// Each wall owns its OWN PRI (RepID == its game id), streamed on the existing cmd 900 loop
// as a single byte (LevelIceWall field 0 crack state). The client self-binds a server-spawned
// level object's PRI on OnStart using get_UniqueID() (== the game id) as the ReplicationID and
// registers it in MatchGame.m_ReplicationEntitis under that id — the map the cmd 900 handler
// (DNCBMEDLPGP -> GetRepEntity) routes through. So the wall's PRI RepID MUST equal its game id;
// keying it on anything else means the crack-state block is silently skipped.
//
// Placing DEDUCTS one gloo wall from the player's inventory (clientUIDs data 1201) via a
// cmd 112 DropInventoryRes, UNLESS MATCH_INFINITE_GLOO is set. Even with infinite, the
// number of ACTIVE walls is capped at the player's current gloo-wall inventory count:
// placing beyond that removes the OLDEST placed wall (FIFO). The cap is read BEFORE the
// deduction so a single place trims at most one wall.

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"time"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

const (
	glooDataID   uint32 = 1201             // gloo-wall item DataID (equips SlotBuilding=13)
	glooMaxHP    uint16 = 750              // fresh-wall HP (matches the client's spawn default)
	glooCrackHP  uint16 = 330              // <=44% of 750 -> cracked (tune here)
	glooLifetime        = 90 * time.Second // hard force-break regardless of HP/state
	glooWallCap         = 3                // max active walls per player (FIFO trim)

	// Wall entity/game id band, DISJOINT from player (0x01xxxxxx) + bot (0x02xxxxxx) + the
	// container id band. The wall's PRI RepID is the SAME value as its game id (see
	// nextWall): the client self-binds a level object's PRI under its UniqueID (= game id).
	wallEntityBase uint32 = 0x03000000 // wall entity/game id = wallEntityBase | seq (also the PRI RepID)
)

// Wall crack-state byte, as the client's LevelIceWall field-0 change handler
// (HPGILFDILLD @0x2c33e2c) reads it: the enum counts DOWN — 2 = intact, 1 = cracked
// (broken geometry + SOUND_ICEWALL_BROKEN), 0 = broken. The field's default is 2.
const (
	wallBroken  byte = 0
	wallCracked byte = 1
	wallIntact  byte = 2
)

// iceWall is one placed gloo wall (server-owned world object).
type iceWall struct {
	id       uint32       // wall entity game id (wallEntityBase band)
	repID    uint32       // per-wall PRI RepID (wallRepBase band)
	owner    uint32       // placer entity id (playerEntityID)
	pos      message.Vec3 // world metres
	rot      message.Quat // facing quaternion, echoed back opaquely
	wallType int32        // building/skin variant from the place request
	hp       uint16
	state    byte
	placedAt time.Time
}

// iceWallState builds the wire spawn/resync record (message::JJIODFAHEOG) for a wall.
// MovePlatformID/HealthExtra/Flag stay at their world-anchored defaults.
func (w *iceWall) iceWallState() message.IceWallState {
	return message.IceWallState{
		ID:       w.id,
		WallType: w.wallType,
		HP:       int32(w.hp),
		Pos:      w.pos,
		Rot:      w.rot,
		RepID:    w.repID,
	}
}

// nextWall allocates a fresh (id, RepID) pair, monotonic and never reused so a new
// wall can't collide with a stale one the client hasn't torn down.
func (s *session) nextWall() (id, repID uint32) {
	s.match.wallSeq++
	id = wallEntityBase | s.match.wallSeq
	// RepID == the game id: the client self-binds a level object's PRI on OnStart using
	// get_UniqueID() (== this id) as the ReplicationID, so the cmd 900 stream must key on it.
	return id, id
}

// wallByID returns the live wall with the given id, or nil.
func (s *session) wallByID(id uint32) *iceWall {
	for _, w := range s.match.walls {
		if w.id == id {
			return w
		}
	}
	return nil
}

// ownerWallCount counts an owner's live walls.
func (s *session) ownerWallCount(owner uint32) (n int) {
	for _, w := range s.match.walls {
		if w.owner == owner {
			n++
		}
	}
	return
}

// popOldestWall removes and returns the owner's oldest wall (walls is FIFO), or nil.
func (s *session) popOldestWall(owner uint32) *iceWall {
	for i, w := range s.match.walls {
		if w.owner == owner {
			s.match.walls = append(s.match.walls[:i], s.match.walls[i+1:]...)
			return w
		}
	}
	return nil
}

// removeWallByID drops the wall with the given id from the FIFO list.
func (s *session) removeWallByID(id uint32) {
	for i, w := range s.match.walls {
		if w.id == id {
			s.match.walls = append(s.match.walls[:i], s.match.walls[i+1:]...)
			return
		}
	}
}

// glooCount returns the (uid, count) of the player's gloo stack (data 1201), or (0,0).
// It PREFERS the gloo equipped in SlotBuilding — the stack the client is actually holding —
// so a placement decrements the SAME stack the client consumes. The merge-on-add invariant
// (purchase + pickup) keeps gloo to a single stack, but preferring the equipped one keeps
// placement correct even if a stray duplicate ever slips through (a bare map scan would
// otherwise return one at random, decrement the wrong stack, and desync the count).
func (s *session) glooCount() (uid, count uint32) {
	for _, e := range s.equipment {
		if e.Slot == message.SlotBuilding {
			if it, ok := s.clientUIDs[e.Unique]; ok && it.data == glooDataID {
				return e.Unique, it.count
			}
		}
	}
	for u, it := range s.clientUIDs {
		if it.data == glooDataID {
			return u, it.count
		}
	}
	return 0, 0
}

// iceWallPRIBlock builds a wall's per-wall replication block: a single byte field 0 (the
// crack state). LevelIceWall::GetMaxRepDataCount is 1 — HP is NOT replicated, only this
// state byte. Streamed on the cmd 900 PRI loop keyed by the wall's own RepID.
func iceWallPRIBlock(state byte) []byte {
	return message.ReplicationBlock([]message.RepEntry{
		{Type: message.RepU8, Field: 0, Value: uint64(state)},
	})
}

// wallStateForHP maps current HP to the crack-state byte (counts DOWN, see the consts).
func wallStateForHP(hp uint16) byte {
	switch {
	case hp == 0:
		return wallBroken
	case hp <= glooCrackHP:
		return wallCracked
	default:
		return wallIntact
	}
}

// handlePlaceIcewall handles the cmd 218 client->server PLACE request. It deducts one
// gloo wall (unless MATCH_INFINITE_GLOO), spawns the wall (cmd 218 broadcast), and
// FIFO-trims the active walls down to the inventory-count cap (evicting the oldest via
// cmd 221). Packets are built into a slice then flushed in order (out/flush pattern).
func (s *session) handlePlaceIcewall(p *packet.Packet) {
	req, ok := message.ParsePlaceIcewallReq(p.Payload)
	if !ok {
		// Short / differently-shaped request (e.g. the cmd 333 ICEWALL_FIRE alias): don't silently eat the
		// tap — place the wall in front of the player instead.
		log.Printf("[mm-udp] cmd=%d place icewall req odd (%dB): %x -> placing in front of player", p.Cmd, len(p.Payload), p.Payload)
		req = s.frontPlaceReq()
	}

	var out []outPkt

	glooUID, glooCount := s.glooCount()
	log.Printf("[mm-udp] cmd=218 gloo place request pos=(%.1f,%.1f,%.1f) gloo=%d", req.Pos.X, req.Pos.Y, req.Pos.Z, glooCount)
	untracked := glooCount == 0 // client says it threw a wall but the server lost the stack: still place it (never silently eat the throw)

	// 1) DEDUCT one, unless infinite. UnlimitedAmmo (room_setting bit 0x2) covers gloo too: the
	//    client already gives guns infinite ammo, but gloo walls are server-tracked, so we must
	//    also skip the deduction here or the wall count still drains. cmd 112 DropInventoryRes SETS
	//    the client's stack to the new count (client removes the item at 0).
	infiniteGloo := cfg.infiniteGloo || s.match.settings.unlimitedAmmo || untracked
	var autoSwitch uint32 // if the last gloo was thrown, the weapon to auto-switch the placer back to
	if !infiniteGloo {
		newCount := glooCount - 1
		if newCount == 0 {
			delete(s.clientUIDs, glooUID)
			s.clearEquip(message.SlotBuilding) // free slot 13
			// No gloo walls left — auto-switch the placer back to a real weapon so they aren't left
			// holding the now-empty Building slot (which the client would otherwise resolve to fists).
			// Prefer the gun held right before the gloo; if that was never captured or has since been
			// dropped, fall back to their best remaining weapon. Emitted AFTER the cmd 112 consume
			// below so it overrides the client's own switch-to-fists on removal.
			target := s.heldBeforeGloo
			if !s.isWeaponUID(target) { // 0 (never captured), dropped, or somehow a non-weapon
				target = s.bestWeaponUID()
			}
			if target != 0 {
				autoSwitch = target
				s.itemOnHand = target
			}
			s.heldBeforeGloo = 0
		} else {
			it := s.clientUIDs[glooUID]
			it.count = newCount
			s.clientUIDs[glooUID] = it
		}
		// cmd 112 (DropInventoryRes) carries NO entity id -> the client applies it to its LOCAL
		// inventory, so it MUST go ONLY to the thrower. Broadcasting it (as this did) desynced any
		// OTHER player who happened to hold an item with the SAME unique as this gloo — item uniques
		// are per-session (nextUID), so they collide across players, and the collided item got set
		// to the gloo's new count / removed. Sent before the broadcast below so the thrower still
		// sees the consume before the auto-switch (cmd 108) that overrides the empty-slot fists swap.
		s.sendDataLog(packet.CmdDropInventory,
			message.DropInventoryRes(glooUID, newCount, 0),
			fmt.Sprintf("cmd=112 gloo consume uid=%d -> %d left (thrower only)", glooUID, newCount))
		if autoSwitch != 0 { // after the consume: switch the placer's hand back to their last weapon (all clients)
			out = append(out, outPkt{packet.CmdChangeHeldItem,
				message.ChangeInventoryOnHand(s.player.EntityID, autoSwitch),
				fmt.Sprintf("cmd=108 auto-switch to uid=%d after last gloo thrown", autoSwitch), false})
		}
	}

	// 2) SPAWN the wall (cmd 218) with its game id as the ReplicationID, so the cmd 218
	//    self-bind (NBEKINCFDBB when RepID!=0) AND the client's own OnStart self-bind
	//    (get_UniqueID() == the game id) agree on the same RepID — the id the wall is
	//    registered under in MatchGame.m_ReplicationEntitis. Our cmd 900 crack-state stream is
	//    keyed by that same id (w.repID == w.id), so it routes to the wall. Trailing byte 0 =
	//    "not destroyed on spawn". No cmd 118 needed — the wall self-binds under its own id.
	_, wallOut := s.match.addWall(s.entityID, req.Pos, req.Rot, req.WallType)
	out = append(out, wallOut...)

	s.match.flushBroadcast(out)                           // flush the built batch (outPkt/flush pattern)
	s.sendVar(packet.CmdPRISync, s.match.priPayload(), 1) // eager wall-state PRI (don't wait ~300ms)
}

// handleIcewallDamage handles cmd 219 (client->server): the wall took damage. Subtract
// from its server-side HP; at 0 remove it (cmd 221), otherwise update its crack state
// (the cmd 900 PRI stream replicates the new state — no cmd 220 on crack, which would
// re-instantiate the wall client-side; cmd 220 is only for reconnect resync).
func (s *session) handleIcewallDamage(p *packet.Packet) {
	req, ok := message.ParseIcewallDamageReq(p.Payload)
	if !ok {
		return
	}
	var out []outPkt
	w := s.wallByID(req.WallID)
	if w == nil {
		return
	}
	dmg := uint16(req.Damage)
	if dmg == 0 { // a 0-damage report never no-ops
		log.Printf("[mm-udp] cmd=219 wall id=%d took 0 damage (weapon=%d) — forcing damage", w.id, req.Weapon)
		dmg = 25
	}
	if dmg >= w.hp {
		w.hp = 0
	} else {
		w.hp -= dmg
	}
	if w.hp == 0 { // broken -> remove
		s.removeWallByID(w.id)
		out = append(out, outPkt{packet.CmdRemoveIcewall, message.RemoveIcewall(w.id),
			fmt.Sprintf("cmd=221 REMOVE_ICEWALL id=%d (destroyed, weapon=%d)", w.id, req.Weapon), false})
	} else {
		w.state = wallStateForHP(w.hp) // crack; the eager PRI below replicates it
		log.Printf("Updated wall state for wall id=%d", w.id)
	}

	s.match.flushBroadcast(out)                         // break/remove must reach EVERY client, not just the reporter
	s.sendVar(packet.CmdPRISync, s.resyncWallsPri(), 1) // replicate the new HP/state on the PRI
}

// sweepWalls force-breaks any wall older than glooLifetime regardless of HP/state, and
// queues its cmd 221. Called on the streamPhase loop (~300ms) — 60s granularity is fine
// and it can't leak goroutines when the player quits or a round ends.
func (s *session) sweepWalls() {
	now := time.Now()
	var out []outPkt
	kept := s.match.walls[:0]
	for _, w := range s.match.walls {
		if now.Sub(w.placedAt) >= glooLifetime {
			out = append(out, outPkt{packet.CmdRemoveIcewall, message.RemoveIcewall(w.id),
				fmt.Sprintf("cmd=221 REMOVE_ICEWALL id=%d (lifetime)", w.id), false})
		} else {
			kept = append(kept, w)
		}
	}
	s.match.walls = kept
	s.match.flushBroadcast(out) // break/remove must reach EVERY client, not just the reporter
}

// clearWalls removes every live wall (cmd 221 each) and empties the list. Walls are
// per-round world state, so this is called on a round reset (roundTransition) so a stale
// wall doesn't survive the black-screen transition.
func (s *session) clearWalls() {
	var out []outPkt
	for _, w := range s.match.walls {
		out = append(out, outPkt{packet.CmdRemoveIcewall, message.RemoveIcewall(w.id),
			fmt.Sprintf("cmd=221 REMOVE_ICEWALL id=%d (round reset)", w.id), false})
	}
	s.match.walls = nil
	s.match.flushBroadcast(out) // break/remove must reach EVERY client, not just the reporter
}

// resyncWalls re-adds every live wall in one cmd 220 RESYNC_ICEWALL — used on a mid-match
// transport re-handshake (cmd 5) to redraw walls the fresh transport session lost.
func (s *session) resyncWalls() {
	states := make([]message.IceWallState, 0, len(s.match.walls))
	for _, w := range s.match.walls {
		states = append(states, w.iceWallState())
	}
	if len(states) == 0 {
		return
	}
	s.sendDataLog(packet.CmdResyncIcewall, message.ResyncIcewalls(states),
		fmt.Sprintf("cmd=220 RESYNC_ICEWALL (%d live walls)", len(states)))
}

// frontPlaceReq builds a place request ~2.5 m ahead of the player along its last aim direction (falls
// back to +Z), used when the client's own place payload can't be parsed.
func (s *session) frontPlaceReq() message.PlaceIcewallReq {
	fx, fz := 0.0, 1.0
	if len(s.moveBody) == moveBodyLen {
		ax := float64(int16(binary.LittleEndian.Uint16(s.moveBody[30:]))) / 100
		az := float64(int16(binary.LittleEndian.Uint16(s.moveBody[34:]))) / 100
		if d := math.Hypot(ax, az); d > 0.01 {
			fx, fz = ax/d, az/d
		}
	}
	pos := s.playerPos
	pos.X += fx * 2.5
	pos.Z += fz * 2.5
	half := math.Atan2(fx, fz) / 2 // yaw about +Y as a quaternion
	return message.PlaceIcewallReq{Pos: pos, Rot: message.Quat{Y: math.Sin(half), W: math.Cos(half)}}
}

// ---- match-level wall helpers (used by humans' place requests AND by bots) ----

const (
	glooHalfW = 1.75 // wall half-width (m) along its local X — generous so nothing slips past an edge
	glooHalfT = 0.5  // wall half-thickness (m) along its local Z (facing)
)

// yaw returns the wall's heading about +Y from its quaternion (scale-agnostic).
func (w *iceWall) yaw() float64 { return 2 * math.Atan2(w.rot.Y, w.rot.W) }

// toLocal maps a world XZ point into the wall's local frame (x = along the wall, z = through it).
func (w *iceWall) toLocal(x, z float64) (lx, lz float64) {
	th := w.yaw()
	s, c := math.Sin(th), math.Cos(th)
	dx, dz := x-w.pos.X, z-w.pos.Z
	return dx*c - dz*s, dx*s + dz*c
}

// contains reports whether the XZ point is inside the wall's footprint grown by pad metres.
func (w *iceWall) contains(x, z, pad float64) bool {
	lx, lz := w.toLocal(x, z)
	return math.Abs(lx) <= glooHalfW+pad && math.Abs(lz) <= glooHalfT+pad
}

// blocksSeg reports whether the XZ segment a->b crosses the wall (slab test in the wall's local frame).
func (w *iceWall) blocksSeg(ax, az, bx, bz float64) bool {
	x0, z0 := w.toLocal(ax, az)
	x1, z1 := w.toLocal(bx, bz)
	tmin, tmax := 0.0, 1.0
	for _, s := range [][3]float64{{x0, x1 - x0, glooHalfW}, {z0, z1 - z0, glooHalfT}} {
		o, d, h := s[0], s[1], s[2]
		if math.Abs(d) < 1e-9 {
			if math.Abs(o) > h {
				return false
			}
			continue
		}
		t1, t2 := (-h-o)/d, (h-o)/d
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		tmin, tmax = math.Max(tmin, t1), math.Min(tmax, t2)
		if tmin > tmax {
			return false
		}
	}
	return true
}

// wallBetween returns the nearest live wall that blocks the line a->b (bullets / sight), or nil.
func (m *Match) wallBetween(ax, az, bx, bz float64) *iceWall {
	var best *iceWall
	bd := math.MaxFloat64
	for _, w := range m.walls {
		if w.blocksSeg(ax, az, bx, bz) {
			if d := math.Hypot(w.pos.X-ax, w.pos.Z-az); d < bd {
				best, bd = w, d
			}
		}
	}
	return best
}

// insideAnyWall reports whether the XZ point is inside any live wall (grown by pad).
func (m *Match) insideAnyWall(x, z, pad float64) bool {
	for _, w := range m.walls {
		if w.contains(x, z, pad) {
			return true
		}
	}
	return false
}

// addWall spawns a wall for owner, FIFO-trims the owner's walls to glooWallCap and returns the packets to
// broadcast (reliable). Shared by a human's place request and by bots.
func (m *Match) addWall(owner uint32, pos message.Vec3, rot message.Quat, wallType int32) (*iceWall, []outPkt) {
	m.wallSeq++
	id := wallEntityBase | m.wallSeq
	w := &iceWall{id: id, repID: id, owner: owner, pos: pos, rot: rot, wallType: wallType,
		hp: glooMaxHP, state: wallIntact, placedAt: time.Now()}
	m.walls = append(m.walls, w)
	m.lastWallType = wallType
	out := []outPkt{{packet.CmdAddIcewall, message.AddIcewall(w.iceWallState(), 0),
		fmt.Sprintf("cmd=218 ADD_ICEWALL id=%d owner=%#x hp=%d", w.id, owner, w.hp), false}}
	for {
		n := 0
		for _, o := range m.walls {
			if o.owner == owner {
				n++
			}
		}
		if n <= glooWallCap {
			break
		}
		for i, o := range m.walls {
			if o.owner == owner {
				m.walls = append(m.walls[:i], m.walls[i+1:]...)
				out = append(out, outPkt{packet.CmdRemoveIcewall, message.RemoveIcewall(o.id),
					fmt.Sprintf("cmd=221 REMOVE_ICEWALL id=%d (FIFO cap=%d)", o.id, glooWallCap), false})
				break
			}
		}
	}
	return w, out
}

// hitWall subtracts dmg from a wall (bots shooting it); at 0 it is removed for everyone.
func (m *Match) hitWall(w *iceWall, dmg uint16) {
	if dmg >= w.hp {
		w.hp = 0
	} else {
		w.hp -= dmg
	}
	if w.hp == 0 {
		for i, o := range m.walls {
			if o == w {
				m.walls = append(m.walls[:i], m.walls[i+1:]...)
				break
			}
		}
		m.flushBroadcast([]outPkt{{packet.CmdRemoveIcewall, message.RemoveIcewall(w.id),
			fmt.Sprintf("cmd=221 REMOVE_ICEWALL id=%d (shot by bot)", w.id), false}})
	} else {
		w.state = wallStateForHP(w.hp)
	}
	m.pushPRI()
}
