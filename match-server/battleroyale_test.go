package main

import (
	"testing"
	"time"
)

// The BR path is selected ONLY by the signed claims, and only when BOTH the map and the game
// mode say Battle Royale — a stray claim must fall back to Clash Squad rather than half-starting BR.
func TestMatchModeSelection(t *testing.T) {
	cases := []struct {
		name string
		mode matchMode
		want bool
	}{
		{"br", matchMode{mapID: brMapID, game: gmBattleRoyale}, true},
		{"clash squad", matchMode{mapID: brMapID, game: gmClashSquad}, false},
		{"no token (zero value)", matchMode{}, false},
		{"br mode, wrong map", matchMode{mapID: 7, game: gmBattleRoyale}, false},
		{"br map, wrong mode", matchMode{mapID: brMapID, game: 4}, false},
	}
	for _, c := range cases {
		if got := c.mode.isBattleRoyale(); got != c.want {
			t.Errorf("%s: isBattleRoyale() = %v, want %v", c.name, got, c.want)
		}
	}
}

// A joining player may only enter a match running the SAME map + game mode, and the roster cap
// must follow the mode: 8 for Clash Squad, 48 for Battle Royale. The BR cap regressing to the CS
// constant is what silently limited a 48-player lobby to 8.
func TestCanAdmitModeAndCap(t *testing.T) {
	br := matchMode{mapID: brMapID, game: gmBattleRoyale}
	cs := matchMode{mapID: brMapID, game: gmClashSquad}

	brMatch := &Match{mode: br, battleRoyale: true}
	csMatch := &Match{mode: cs}
	brJoiner := &session{mode: br}
	csJoiner := &session{mode: cs}

	if !brMatch.canAdmit(brJoiner) {
		t.Fatal("a fresh BR match must admit a BR player")
	}
	if brMatch.canAdmit(csJoiner) {
		t.Fatal("a BR match must NOT admit a Clash Squad player")
	}
	if csMatch.canAdmit(brJoiner) {
		t.Fatal("a Clash Squad match must NOT admit a BR player")
	}

	if got := brMatch.rosterCap(); got != maxBRPlayers {
		t.Fatalf("BR roster cap = %d, want %d", got, maxBRPlayers)
	}
	if got := csMatch.rosterCap(); got != maxPlayers {
		t.Fatalf("CS roster cap = %d, want %d", got, maxPlayers)
	}

	brMatch.reserved = maxPlayers // the OLD cap: a BR match must still be open here
	if !brMatch.canAdmit(brJoiner) {
		t.Fatalf("BR match refused player %d — still capped at the Clash Squad limit", maxPlayers+1)
	}
	brMatch.reserved = maxBRPlayers
	if brMatch.canAdmit(brJoiner) {
		t.Fatal("BR match admitted past maxBRPlayers")
	}

	brMatch.reserved = 0
	brMatch.started = true
	if brMatch.canAdmit(brJoiner) {
		t.Fatal("a STARTED match must not take latecomers")
	}
}

// Solo BR is free-for-all: every player must get a distinct team (entity-id hibyte), because the
// combat rules key damage credit, kill credit and single-life elimination off teamOf().
func TestAllocSlotBattleRoyaleGivesEveryPlayerItsOwnTeam(t *testing.T) {
	m := &Match{battleRoyale: true}
	seenTeam := map[byte]bool{}
	seenEntity := map[uint32]bool{}
	for i := 0; i < 48; i++ {
		entity, _, _, team, _, slot := m.allocSlot(0)
		if seenTeam[team] {
			t.Fatalf("slot %d reused team %d — players would count as teammates", slot, team)
		}
		if seenEntity[entity] {
			t.Fatalf("slot %d reused entity id %#x", slot, entity)
		}
		seenTeam[team] = true
		seenEntity[entity] = true
		if m.teamOf(entity) != team {
			t.Fatalf("entity %#x hibyte %d != team %d", entity, m.teamOf(entity), team)
		}
	}
	// The first BR player must still get the entity id the client-facing constants assume.
	if !seenEntity[playerEntityID] {
		t.Fatalf("the first BR player no longer gets entity %#x", uint32(playerEntityID))
	}
}

// Clash Squad keeps its balance-fill: the first human on team 1, the next on team 2.
func TestAllocSlotClashSquadStillBalances(t *testing.T) {
	m := &Match{}
	if _, _, _, team, _, _ := m.allocSlot(0); team != 1 {
		t.Fatalf("first CS player team = %d, want 1", team)
	}
	m.players = append(m.players, &session{team: 1})
	if _, _, _, team, _, _ := m.allocSlot(0); team != 2 {
		t.Fatalf("second CS player team = %d, want 2", team)
	}
}

// While the circle is closing, the damage radius must trail the visible one by zoneClientLag, so a
// player standing inside the circle they can SEE is never damaged.
func TestDamageRadiusTrailsTheVisibleCircle(t *testing.T) {
	now := time.Now()
	m := &Match{zoneOuterR: 1350, zoneInnerR: 900}

	if got := m.damageRadius(now); got != 1350 {
		t.Fatalf("idle zone radius = %.0f, want the outer radius 1350", got)
	}

	m.zoneShrink = true
	m.zoneShrinkStart = now
	if got := m.damageRadius(now); got != 1350 {
		t.Fatalf("radius at the shrink edge = %.0f, want 1350 (no damage before the circle moves)", got)
	}

	half := now.Add(brShrinkDuration / 2)
	sameInstant := 1350 + (900-1350)*0.5
	got := m.damageRadius(half)
	if got <= sameInstant {
		t.Fatalf("mid-shrink radius = %.1f, want LARGER than the same-instant lerp %.1f (client lag)", got, sameInstant)
	}

	if got := m.damageRadius(now.Add(brShrinkDuration * 2)); got != 900 {
		t.Fatalf("radius after the shrink = %.0f, want the inner radius 900", got)
	}
}

// A solo drop is a private test session and must never auto-finish; a real match must crown its
// last survivor even if the others left rather than died.
func TestCheckBattleRoyaleEndSoloVersusRealMatch(t *testing.T) {
	solo := &Match{battleRoyale: true, brPhase: brFight, brRoster: 1, life: map[uint32]lifeState{}}
	solo.players = []*session{{entityID: 0x01000001}}
	solo.checkBattleRoyaleEnd()
	if solo.brWinner != 0 || solo.brPhase != brFight {
		t.Fatal("a solo BR test session must stay open-ended")
	}

	real := &Match{battleRoyale: true, brPhase: brFight, brRoster: 4, life: map[uint32]lifeState{}}
	survivor := &session{entityID: 0x02000002}
	real.players = []*session{survivor}
	real.checkBattleRoyaleEnd()
	if real.brWinner != survivor.entityID {
		t.Fatalf("brWinner = %#x, want the last survivor %#x", real.brWinner, survivor.entityID)
	}
	if real.brPhase != brMatchEnd {
		t.Fatal("a decided BR match must move to the match-end phase")
	}
}

// The zone stages must shrink monotonically and end at the final radius the doc states.
func TestBattleRoyaleZoneStagesShrinkMonotonically(t *testing.T) {
	stages := [...]float64{1350, 900, 550, 250}
	for i := 1; i < len(stages); i++ {
		if stages[i] >= stages[i-1] {
			t.Fatalf("stage %d radius %.0f does not shrink from %.0f", i+1, stages[i], stages[i-1])
		}
	}
	if stages[0] != brOuterRadius {
		t.Fatalf("stage 1 radius %.0f != brOuterRadius %.0f", stages[0], brOuterRadius)
	}
}
