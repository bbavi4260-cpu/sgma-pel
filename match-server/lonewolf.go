package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

const (
	loneWolfWait = 5 * time.Second
)

// loneWolfSpawn is deliberately independent from every CS arena and BR point.
// The defaults target the small LabMap/Iron-Cage play area; deployments using a
// different resource revision can override them with MATCH_LONEWOLF_A / B.
func loneWolfSpawn(slot byte) (message.Vec3, message.Vec3) {
	def := message.Vec3{X: 0, Y: 6, Z: 0}
	if slot == 2 {
		def = message.Vec3{X: 14, Y: 6, Z: 0}
	}
	key := "MATCH_LONEWOLF_A"
	if slot == 2 {
		key = "MATCH_LONEWOLF_B"
	}
	if v, ok := parseVec3(os.Getenv(key)); ok {
		def = v
	}
	if slot == 2 {
		return def, message.Vec3{X: -1, Y: 0, Z: 0}
	}
	return def, message.Vec3{X: 1, Y: 0, Z: 0}
}

func (m *Match) startLoneWolfLifecycle(now time.Time) {
	m.started = false
	m.phase = phSetupWait
	m.deadline = now.Add(loneWolfWait)
	// No automatic opponent: Lone Wolf is human-only. A single player may enter
	// and test the controls; a second human can join the same cage later.
	log.Printf("[mm-udp] Lone Wolf / Iron Cage lifecycle started: players=%d bot=%#x", len(m.players), m.botEntity)
}

func (m *Match) spawnLoneWolfBot() {
	m.botEntity = botEntityID
	m.bot = botPlayer(m.s.player, m.botEntity)
	m.bot.SpawnPos, m.bot.SpawnFace = loneWolfSpawn(2)
	m.hp[m.botEntity] = m.settings.maxHP
	m.life[m.botEntity] = lifeAlive
	m.broadcastData(packet.CmdPlayerJoin, message.PlayerJoin(m.bot, m.serverTick()),
		fmt.Sprintf("cmd=101 LONE_WOLF BOT ent=%#x pos=(%.1f,%.1f,%.1f)", m.botEntity, m.bot.SpawnPos.X, m.bot.SpawnPos.Y, m.bot.SpawnPos.Z))
	m.bindAll()
}

func (m *Match) startLoneWolfFight(now time.Time) {
	m.started = true
	m.phase = phFight
	m.deadline = time.Time{}
	for _, p := range m.players {
		pos, face := loneWolfSpawn(p.slot)
		p.player.SpawnPos, p.player.SpawnFace = pos, face
		p.playerPos = pos
		m.tpSeq++
		p.sendData(packet.CmdTeleport, message.ForceTeleport(p.entityID, m.tpSeq, pos, face, 0))
	}
	if m.botEntity != 0 {
		pos, face := loneWolfSpawn(2)
		m.bot.SpawnPos, m.bot.SpawnFace = pos, face
		m.tpSeq++
		m.broadcastData(packet.CmdTeleport, message.ForceTeleport(m.botEntity, m.tpSeq, pos, face, 0), "Lone Wolf bot cage spawn")
	}
	log.Printf("[mm-udp] Lone Wolf / Iron Cage FIGHT started")
}

func (m *Match) advanceLoneWolf(now time.Time) {
	if m.phase == phSetupWait {
		if now.Before(m.deadline) {
			return
		}
		m.startLoneWolfFight(now)
		return
	}
	if m.phase != phFight {
		return
	}
	// Do not declare a winner while the cage only contains one human.
	// The second human may join after the fight has already started.
	if len(m.players) < 2 {
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
	if m.botEntity != 0 && m.lifeOf(m.botEntity) == lifeAlive {
		alive++
		winner = m.botEntity
	}
	if alive != 1 {
		return
	}

	m.winnerTeam = 2
	if winner != 0 && winner != m.botEntity {
		m.winnerTeam = m.teamOf(winner)
	}
	m.ended = true
	m.phase = phMatchEndPause
	m.deadline = now.Add(matchEndPause)
	log.Printf("[mm-udp] Lone Wolf winner ent=%#x", winner)
}
