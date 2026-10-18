package packet

import (
	"encoding/binary"
	"errors"

	"libmadoka/match-server/crypto"
)

// MsgKey is the fixed first byte of every match packet (GCommon::UDPSession
// ::HandleRecv rejects anything else).
const MsgKey = 0x6C

// send_option values (see UDP_MATCH_PROTOCOL.md).
const (
	SendUnreliable = 0 // cmd routed normally, no seq/order
	SendHello      = 1 // connection/HELLO response (S2C_Hello_Res)
	SendReliable   = 2 // normal reliable app packet
	SendKick       = 3 // "kicked by server"
	SendVar        = 4 // variables
	SendReliable5  = 5 // reliable variables - never used?
)

// flags bits.
const (
	FlagEncrypted = 1 // payload is TEA(SECRET_KEY)
	FlagGzip      = 2 // payload is gzip(SECRET_KEY)
)

// cmds (message.OPNCEEBBMJH enum values recovered from the client).
const (
	CmdHello                = 1    // client HELLO / server S2C_Hello_Res
	CmdAck                  = 2    // reliable ACK / keepalive
	CmdPing                 = 3    // unreliable ping (client<->server)
	CmdReconnect            = 5    // client reconnect hello (carries SessionKey)
	CmdBattleStart          = 110  // server -> client: JAKKPBMHLAI (start the match)
	CmdForceSync            = 144  // server -> client: JIIKBLKJCKM force-sync (SyncStateWithServer) — position + phys state (handler DDBOMMOJHDD)
	CmdTeleport             = 145  // server -> client: JIIKBLKJCKM teleport (SyncTeleportInfo) — position + ROTATION (handler OHPEJIKNCMD)
	CmdJoinMatchFinished    = 130  // server -> client: LGBENPIAFIN (join complete / HUD)
	CmdJoinMatchRes         = 100  // server -> client: LGIGCGIDOKP (join result)
	CmdPlayerJoin           = 101  // server -> client: GKBDLJFGGMI (player join)
	CmdMatchEnd             = 103  // server -> client: RUDP_MATCH_END (also used as the SendKick(so=3) carrier on shutdown)
	CmdChangeHeldItem       = 108  // client <-> server: RUDP_CHANGE_INVENTORY_ON_HAND — [entity u32][itemUnique u32] (C2S rx GMEMMIIODBK; S2C re-assert send class JLMEBGKMNIL)
	CmdEquip                = 119  // client -> server: JHMFPMMDOLP — RUDP_EQUIP (inventory slot swap): [u8 slotID][u32 uniqueID]
	CmdEquipmentChanged     = 121  // server -> client: LJGFNPIMGMA — RUDP_EQUIPMENT_CHANGED (visual slot move): [u32 pid][u8 slot][u32 oldUID][u32 newUID][u32 dataID][u32 runtime]
	CmdPickupInventory      = 111  // client -> server: LHODJLEHDND — RUDP_PICKUP_INVENTORY (pick a ground item by UniqueID)
	CmdDropInventory        = 112  // client <-> server: KJBONEENCAL — RUDP_DROP_INVENTORY (req: u32 unique,u32 count,u8 reason; res reuses class)
	CmdUseInventory         = 113  // client -> server: RUDP_USE_INVENTORY — finished channelling a consumable ([u32 uid][u32 count][u32 param]); server applies the effect + tracks the consume
	CmdTryUseInventory      = 131  // client -> server: RUDP_TRYUSE_INVENTORY (KOMODKGGDBG, [u32 entity][u32 item][u32 tick]) — started channelling a consumable
	CmdDoAction             = 201  // server -> client: FPCOHGNMLBG — RUDP_DoAction (0xC9): plays an action anim on a REMOTE pawn (medkit cure begin/end); NOT a cmd 131/113 relay
	CmdEmote                = 369  // client <-> server: 0x171 — play emote. C2S LAPCIFKMAAP [u32 emoteId][u8 effIdx][u8 loop][u8 leadIngame][u32 followLeader][u32 leadTick]; S2C NIBOCOJJCCJ [u32 pid][u32 emoteId][u8 effIdx][u8 loop] relayed to others
	CmdBattleFlagReq        = 276  // client -> server: 0x114 GILPGGDOOGE — RequestUseBattleFlag: [u32 owner][i32 x][i32 y][i32 z /1000][u32 flagConfigId]; client planted a flag-emote flag
	CmdBattleFlagSync       = 277  // server -> client: 0x115 AACLJKNELJE — [i16 count]{AMPGELLIDGH: u32 owner, u32 objectId, u8 active(0=despawn), i32 x/y/z /1000, u32 flagConfigId, u8 extra=0}×count; authoritative named-flag spawn/despawn
	CmdEquipAttachment      = 122  // client -> server: RUDP_EQUIP_ATTACHMENT — client's manual attachment equip (server auto-maxes on buy, so logged/ignored)
	CmdUnequipAttachment    = 123  // client -> server: RUDP_UNEQUIP_ATTACHMENT — IGNORED (attachments are kept locked on the weapon)
	CmdAttachmentChanged    = 124  // server -> client: KBDODAHANGB — RUDP_ATTACHMENT_CHANGED (force-equip a maxed attachment onto a bought weapon)
	CmdAddPickup            = 114  // server -> client: INMIMMDMPFM — RUDP_ADD_PICKUP (one ground-loot item at a position)
	CmdDelPickup            = 115  // server -> client: KBHENKFBDAJ — RUDP_DEL_PICKUP (remove one ground-loot item)
	CmdAddIcewall           = 218  // client <-> server: RUDP_ADD_ICEWALL (0xDA) — C->S place request (JEGADMHBFKB, 29B); S->C spawn broadcast (KACGKFMOHNP, 41B)
	CmdIcewallTakeDamage    = 219  // client -> server: CHDLJFJCPFN — RUDP_ICEWALL_TAKE_DAMAGE (0xDB) (wallID + delta damage; server subtracts HP)
	CmdResyncIcewall        = 220  // server -> client: EKKLNKLGFIM — RUDP_RESYNC_ICEWALL (0xDC) ([i16 count] + JJIODFAHEOG×N; full re-add of live walls)
	CmdRemoveIcewall        = 221  // server -> client: APPOHELKIIG — RUDP_REMOVE_ICEWALL (0xDD) ([u32 wallID]; break/timeout/FIFO-evict)
	CmdAddPickupList        = 225  // server -> client: ANICDCDDCLN — RUDP_ADD_PICKUPLIST (a shared-position list of items in a container)
	CmdAddContainer         = 227  // server -> client: GONMBPDMEBE — RUDP_ADD_CONTAINER ([i16 count]{u32,f32,bool}; no position)
	CmdDelContainer         = 228  // server -> client: OPNFJLJGPFC — RUDP_DEL_CONTAINER ([u16 containerID][u8 containerType])
	CmdTakeDamage           = 106  // client -> server: hit report (victim u32, damage u16, attacker u32, ...); server applies HP
	CmdShowDamage           = 168  // server -> client: RUDP_SHOW_DAMAGE — floating damage numbers; relayed to a spectator of the shooter
	CmdSwitchObserve        = 149  // server -> client: RUDP_SWITCH_OBSERVE — re-point a spectator to a new target ([bool apply][u32 entity])
	CmdObserverJoin         = 230  // server -> client: RUDP_OBSERVER_JOIN (ILIKAAODPKH) — type=2 EObserverType_Replay: LIVE no-op, seeds the replay observer + dismisses its loading mask
	CmdWeaponAction         = 104  // client -> server: MFCMPPGIMNH — weapon action (u32 pid, u32 weapon, u32 tick, u32 actionType[2=START FIRE], f32 turn); relay to others for the fire visual
	CmdStopFire             = 105  // client -> server: FCHFNBNCJLB — stop firing (u32 pid, u32 weapon, ...); relay to others
	CmdReloadStart          = 133  // client -> server: DAGDONBPDDC — reload started (u32 pid, f32 time, bool, u32 weapon); relay to others
	CmdReloadFinish         = 134  // client -> server: GKPFCFLPBNE — reload finished (u32 pid, u32, u32 weapon); relay to others
	CmdProjectileThrow      = 160  // client -> server: RUDP_PROJECTILE_THROW (FEPGIAHJGHC) — spawns a thrown grenade on remotes; relay
	CmdProjectileExplode    = 161  // client -> server: RUDP_PROJECTILE_EXPLODE (FMMCDAENDPG) — grenade explosion FX; relay
	CmdStartGrenade         = 162  // client -> server: RUDP_START_GRENADE — pull-pin/cook windup cue; relay
	CmdStopGrenade          = 163  // client -> server: RUDP_STOP_GRENADE — windup end; relay
	CmdStartSniper          = 137  // client -> server: RUDP_START_SNIPER (HPAEFLDNDDG, 12B 3×u32) — scope open; set PRI field 12 + relay
	CmdStopSniper           = 138  // client -> server: RUDP_STOP_SNIPER (JANBDNDCGFF, empty) — scope close; clear PRI field 12 + relay
	CmdHurtAnim             = 1010 // server -> client: get-hurt / hit-reaction animation — broadcast on damage so the victim flinches (even with no attacker, e.g. zone)
	CmdZoneIndex            = 457  // server -> client: GELOHPLLLHA — per-player CS zone index (round-intro CITY NAME); [i16 count][u32 uid][u8 packed(bits0-3=ZoneID)]×count
	CmdDead                 = 107  // server -> client: OGCHKCGKGKN — entity died (victim, killer, position, ...)
	CmdKnockDown            = 139  // server -> client: NKDBFGLPCCF — RUDP_KNOCK_DOWN (DBNO: downed+bleeding, still alive; distinct from cmd 107)
	CmdKnockRevive          = 140  // server -> client: DJOOMNBJMOG — RUDP_KNOCK_REVIVE (teammate revive done: revivedId, rescuerId)
	CmdStartRescue          = 142  // bidirectional: client EMOBCDJEOLN START_RESCURE (u32 target, u64 startMs); server DCBAMPDIHIG rescue-ack/progress (bool ok, u32 RESCUER, u32 RESCUED, u64 start, u64 end, u32 result) — same opcode; 2nd u32 (rescued) drives the downed player's being-revived indicator
	CmdStopRescue           = 143  // client -> server: CNCBPOJGNGK — RUDP_STOP_RESCURE (u32 targetId)
	CmdBindPRI              = 118  // server -> client: S2C_RUDP_BindPRI (reliable) — maps RepIDs to entities; local player FIRST
	CmdSyncInventory        = 174  // server -> client: BGKCMKNDAGA (typed) — player inventory/equipment sync (additive; send once)
	CmdSyncRegionActivities = 176  // server -> client: DKKNMJDABAC — RUDP_SYNC_REGION_ACTIVITIES; triggers the dynamic-prefab visibility pass (jump pads / invisible boxes). Send once on join.
	CmdRemoveInventoryList  = 327  // server -> client: ABJFDIFIILN (typed) — REMOVE items by unique from a player's inventory (the only remove path)
	CmdPlayerQuit           = 102  // server -> client: RUDP_PLAYER_QUIT — ANOTHER player left (u32 entity); the client removes just that player
	CmdPlayerQuitReq        = 191
	CmdPlayerQuitRes        = 192 // server -> client: RUDP_PLAYER_QUIT_RES — the server's ack of the RECIPIENT's OWN quit (makes THEM leave)
	CmdCSShop               = 413 // server -> client: RUDP_SYNC_CSSHOPITEMS_LIST (NDBLNLLMMJA) — Contra Squad buy-phase shop (LOJA) item list; sigma.cs 1.106
	CmdCSPurchase           = 414 // client <-> server: RUDP_PURCHASE_CSITEMS (IIOCLGDNBBB) — buy request / result; sigma.cs 1.106
	// Older dialect (1.70.x) IDs for the same shop messages. The target client's dialect is not certain,
	// so the server speaks BOTH: shop list sent on 413 + 407, buy requests accepted on 414 + 408 (the
	// reply goes out on whichever the client used), mushroom count on 547 + 533.
	CmdIcewallFire             = 333 // client -> server: RUDP_ICEWALL_FIRE — alternate gloo place path some builds use
	CmdCSShopLegacy            = 407
	CmdCSPurchaseLegacy        = 408
	CmdShopPurchaseCountLegacy = 533
	CmdShopPurchaseCount       = 547 // server -> client: RUDP_SYNC_LIMITED_PURCHASE_ITEMS — limited-item purchase counts; sigma.cs 1.106 (verify payload against CHEEGFNLIOE)
	CmdCSRoundResult           = 415 // server -> client: RUDP_CS_CURROUND_RESULT (DMJPAJFMMMB) — round result (winner team id -> VICTORY/DEFEAT + score tally); sigma.cs 1.106
	CmdSafeZoneChange          = 117 // server -> client: KGOHADAMBLI (typed) — circular safezone (outer->inner shrink; EndMs drives the Fight timer)
	CmdNotifyRevive            = 394 // server -> client: RUDP_NOTIFYREVIVE (MJCMIMNHILD) — revive an EXISTING dead player at a position (keeps team/faction/loadout); sigma.cs 1.106
	CmdPRISync                 = 900 // server -> client: PRI replication (per-player), 0x384. (1.70.1; ref's 500 is an unrelated typed handler AJNCGCHMNIP)
	CmdGRISync                 = 901 // server -> client: GRI replication (game-wide match state), single block, 0x385. (1.70.1; ref's 501 is an unrelated typed handler JPKINIIHOFP)
	// Current Sigma protocol enum values verified in sigma.cs.
	CmdJoinMatchPrepare = 447 // client -> server: RUDP_JOIN_MATCH_PREPARE
	CmdJoinMatchPost    = 448 // client -> server: RUDP_JOIN_MATCH_POST
	// Legacy dialect values retained separately; do not confuse them with the current enum.
	CmdJoinMatchPrepareLegacy = 439
	CmdJoinMatchPostLegacy    = 440
	// CmdClientReady (190) and CmdClientLoaded (340) are the other two messages a pre-1.70
	// client sends that the 1.70 path never sees, and that this server left unanswered. They
	// arrive once the client is IN the match — 190 carries a single byte, 340 a u32 (observed
	// value 2) — which is the shape of a "loaded / ready to spawn" handshake. Answering cmd 480
	// is what first produced is_battle_started=true, so the remaining two unanswered messages
	// are the next candidates for whatever still holds the pawn back.
	CmdClientReady  = 190 // client -> server: ready/loaded signal (pre-1.70)
	CmdClientLoaded = 340 // client -> server: load-stage report (pre-1.70)

	// CmdDeviceReport is sent by pre-1.70 clients right after the join: a length-prefixed
	// hardware-fingerprint string plus a few trailing fields. The 1.70 path never sees it. It is
	// the ONLY message this client sends that the server leaves unanswered, so it is the
	// remaining candidate for a gate the client waits on before it spawns its pawn.
	CmdDeviceReport   = 480  // client -> server: device/integrity report (pre-1.70)
	CmdSyncServerTime = 1000 // server -> client: uint32 ServerGameTickCount; client clock = tick/30s (drives the CS timers)
	CmdClientPos      = 1001 // client -> server: per-tick position/state update (carries the local player's world pos)
	CmdClientPosV2    = 1012 // server -> client: UDP_PLAYER_STATE_SYNC_V2 — the remote-movement broadcast 1.70.1 actually applies (1001 downstream is ignored on this build)
	CmdInGameChat     = 1030 // server -> client: apparently sends a chat message? unsure
)

var (
	ErrShort  = errors.New("packet: too short")
	ErrMsgKey = errors.New("packet: bad msg_key")
	ErrCRC    = errors.New("packet: bad crc7")
	ErrGzip   = errors.New("packet: gzip payload not supported")
	ErrLength = errors.New("packet: declared payload length does not match datagram")
)

// IsReliable mirrors GCommon::UDPMsgPacket::IsReliable.
func IsReliable(cmd uint16, sendOption byte) bool {
	if cmd == 2 {
		return false
	}
	return sendOption == 1 || sendOption == 2 || sendOption == 5
}

// Packet is a decoded/decodable match packet. Payload is always PLAINTEXT here
// (Decode decrypts; Encode encrypts when Flags&FlagEncrypted).
type Packet struct {
	SendOption byte
	Cmd        uint16
	SeqID      uint16
	OrderID    uint16
	Flags      byte
	Payload    []byte
}

// Decode parses one inbound datagram. key is the 16-byte TEA key.
func Decode(buf, key []byte) (*Packet, error) {
	if len(buf) < 8 {
		return nil, ErrShort
	}
	if buf[0] != MsgKey {
		return nil, ErrMsgKey
	}
	if crypto.CRC7(0, buf[2:]) != buf[1] {
		return nil, ErrCRC
	}
	p := &Packet{}
	off := 2
	p.SendOption = buf[off]
	off++
	p.Cmd = binary.LittleEndian.Uint16(buf[off:])
	off += 2
	if IsReliable(p.Cmd, p.SendOption) {
		if len(buf) < 12 {
			return nil, ErrShort
		}
		p.SeqID = binary.LittleEndian.Uint16(buf[off:])
		off += 2
		p.OrderID = binary.LittleEndian.Uint16(buf[off:])
		off += 2
	}
	p.Flags = buf[off]
	off++
	length := int(binary.LittleEndian.Uint16(buf[off:]))
	off += 2
	if off+length > len(buf) {
		return nil, ErrShort
	}
	if off+length != len(buf) {
		return nil, ErrLength
	}
	payload := buf[off : off+length]
	if p.Flags&FlagGzip != 0 {
		return nil, ErrGzip
	}
	if p.Flags&FlagEncrypted != 0 {
		dec, err := crypto.TeaDecrypt(payload, key)
		if err != nil {
			return nil, err
		}
		payload = dec
	}
	p.Payload = append([]byte(nil), payload...)
	return p, nil
}

// Encode serializes the packet to wire bytes. Encrypts the payload when
// Flags&FlagEncrypted is set.
func (p *Packet) Encode(key []byte) ([]byte, error) {
	payload := p.Payload
	if p.Flags&FlagEncrypted != 0 {
		enc, err := crypto.TeaEncrypt(payload, key)
		if err != nil {
			return nil, err
		}
		payload = enc
	}
	if len(payload) > 0xFFFF {
		return nil, ErrLength
	}
	body := make([]byte, 0, 10+len(payload))
	body = append(body, p.SendOption)
	body = binary.LittleEndian.AppendUint16(body, p.Cmd)
	if IsReliable(p.Cmd, p.SendOption) {
		body = binary.LittleEndian.AppendUint16(body, p.SeqID)
		body = binary.LittleEndian.AppendUint16(body, p.OrderID)
	}
	body = append(body, p.Flags)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(payload)))
	body = append(body, payload...)

	out := make([]byte, 0, 2+len(body))
	out = append(out, MsgKey, crypto.CRC7(0, body))
	out = append(out, body...)
	return out, nil
}

// BuildAck builds the unreliable cmd=2 ACK for a received reliable seq.
func BuildAck(seq uint16, ackBits uint32, key []byte) ([]byte, error) {
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:], seq)
	binary.LittleEndian.PutUint32(payload[2:], ackBits)
	return (&Packet{SendOption: SendUnreliable, Cmd: CmdAck, Payload: payload}).Encode(key)
}

// BuildHelloRes builds the S2C_Hello_Res payload:
//
//	keyLen(i32) | SessionKey(UTF-8) | OrderID(u16) | RequiredID(u16) | EnableFastProto(u8)
func BuildHelloRes(sessionKey string, orderID, requiredID uint16, enableFastProto bool) []byte {
	b := make([]byte, 0, 9+len(sessionKey))
	b = binary.LittleEndian.AppendUint32(b, uint32(len(sessionKey)))
	b = append(b, []byte(sessionKey)...)
	b = binary.LittleEndian.AppendUint16(b, orderID)
	b = binary.LittleEndian.AppendUint16(b, requiredID)
	var fp byte
	if enableFastProto {
		fp = 1
	}
	return append(b, fp)
}
