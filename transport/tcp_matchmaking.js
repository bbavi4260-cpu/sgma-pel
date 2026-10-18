'use strict';

// Local-only matchmaking gateway for the old v38 HTTP/SQLite server.
// It reuses the reviewed PrjShadow TCP framing and the matching Sigma protobuf
// field numbers while leaving test_server.py, SQLite routes, and notification
// service unchanged. No HTTP login or account database is replaced here.
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');
const {
  CMD, INIT_ACK, MAX_FRAME_BYTES, decrypt, parseClientFrames, encodeServerFrame
} = require('./framing');

const PROTOCOL_MATCHMAKING = 3;
const MM_START = 1;
const MM_CANCEL = 2;
const MM_GAMEOPENINGINFO = 7;
const MM_MATCHMAKINGSUSS_NTF = 5;
const MM_START_NTF = 11;
const MM_STOP_NTF = 14;
// How long a lone player waits before the match is formed. The old 15 s default made every
// solo start feel frozen; the Go node fills the roster with bots anyway, so 3 s is plenty.
// The 15 in START_NTF field 5 is only the client's displayed estimate and is kept as-is.
const DEFAULT_MM_TIMEOUT_MS = 3000;
const MM_START_NTF_ESTIMATE_SECONDS = 15;
const UDP_SECRET = process.env.MATCH_UDP_SECRET_HEX || '00112233445566778899aabbccddeeff';
// The bundled UDP node implements Contra Squad only. Never hand off a mode the
// game node cannot initialise; doing so makes the retail client stop at loading.
const SUPPORTED_MAP_ID = 1;
const SUPPORTED_GAME_MODE = 15;
// Fight Out / CS StrikeOut uses the same map bootstrap in this build. Keep it
// as a protocol-visible alias so the client can resolve its MapModeData before
// START; the UDP node still runs the proven CS world implementation.
const FIGHT_OUT_GAME_MODE = 36;
const FIGHT_OUT_ROUNDS = Math.max(1, Number(process.env.MATCH_FIGHTOUT_ROUNDS || 7)); // Fight Out match length
const BATTLE_ROYALE_PALACE_GAME_MODE = 41;
const SUPPORTED_GAME_MODES = new Set([SUPPORTED_GAME_MODE, FIGHT_OUT_GAME_MODE]);

function readVarint(buf, state) {
  let value = 0n;
  let shift = 0n;
  while (state.offset < buf.length && shift <= 70n) {
    const b = buf[state.offset++];
    value |= BigInt(b & 0x7f) << shift;
    if ((b & 0x80) === 0) return value;
    shift += 7n;
  }
  throw new Error('malformed protobuf varint');
}

function decodeFields(buf) {
  const fields = new Map();
  const state = { offset: 0 };
  while (state.offset < buf.length) {
    const tag = readVarint(buf, state);
    const id = Number(tag >> 3n);
    const wire = Number(tag & 7n);
    if (!id) throw new Error('invalid protobuf field number');
    let value;
    if (wire === 0) value = readVarint(buf, state);
    else if (wire === 1) {
      if (state.offset + 8 > buf.length) throw new Error('truncated fixed64');
      value = buf.subarray(state.offset, state.offset + 8); state.offset += 8;
    } else if (wire === 2) {
      const len = Number(readVarint(buf, state));
      if (!Number.isSafeInteger(len) || len < 0 || state.offset + len > buf.length) throw new Error('truncated bytes field');
      value = buf.subarray(state.offset, state.offset + len); state.offset += len;
    } else if (wire === 5) {
      if (state.offset + 4 > buf.length) throw new Error('truncated fixed32');
      value = buf.subarray(state.offset, state.offset + 4); state.offset += 4;
    } else throw new Error(`unsupported protobuf wire type ${wire}`);
    if (!fields.has(id)) fields.set(id, []);
    fields.get(id).push({ wire, value });
  }
  return fields;
}

function encodeVarint(value) {
  let n = BigInt(value);
  if (n < 0n) throw new Error('negative unsigned protobuf value');
  const out = [];
  while (n > 0x7fn) { out.push(Number((n & 0x7fn) | 0x80n)); n >>= 7n; }
  out.push(Number(n));
  return Buffer.from(out);
}
function fieldVarint(id, value) { return Buffer.concat([encodeVarint((BigInt(id) << 3n) | 0n), encodeVarint(value)]); }
function fieldBytes(id, value) {
  const b = Buffer.isBuffer(value) ? value : Buffer.from(value);
  return Buffer.concat([encodeVarint((BigInt(id) << 3n) | 2n), encodeVarint(b.length), b]);
}
function fieldString(id, value) { return fieldBytes(id, Buffer.from(String(value), 'utf8')); }
function firstVarint(fields, id, fallback = 0n) {
  const entry = fields.get(id)?.find((x) => x.wire === 0);
  return entry ? entry.value : BigInt(fallback);
}
function firstBytes(fields, id) { return fields.get(id)?.find((x) => x.wire === 2)?.value || Buffer.alloc(0); }
function repeatedUInt32(fields, id) {
  const values = [];
  for (const item of fields.get(id) || []) {
    if (item.wire === 0) values.push(Number(item.value));
    else if (item.wire === 2) {
      const state = { offset: 0 };
      while (state.offset < item.value.length) values.push(Number(readVarint(item.value, state)));
    }
  }
  return values;
}
function encodeUIntList(id, values) { return Buffer.concat(values.map((v) => fieldVarint(id, v))); }

function gameOpeningInfoEntry(gameMode, name, sortId) {
  return Buffer.concat([
    fieldVarint(1, SUPPORTED_MAP_ID),
    fieldString(3, name),
    fieldString(6, '00:00'),
    fieldString(8, '23:59'),
    fieldString(9, 'Permanent'),
    fieldVarint(13, gameMode),
    fieldVarint(14, 1),
    fieldVarint(15, 1),
    fieldString(17, '2020-01-01 00:00:00'),
    fieldString(18, '2030-12-31 23:59:59'),
    fieldString(19, '4;4'),
    fieldString(21, '1'),
    fieldString(23, '1;2;3;4;5;6;7'),
    fieldVarint(24, sortId),
    fieldString(25, 'https://foices.github.io/minhas_resources/contra_squad.png'),
    fieldVarint(26, 1),
    fieldVarint(33, 1)
  ]);
}

function gameOpeningInfoContent() {
  const openingList = Buffer.concat([
    fieldBytes(1, gameOpeningInfoEntry(SUPPORTED_GAME_MODE, 'Contra Squad', 1)),
    fieldBytes(1, gameOpeningInfoEntry(FIGHT_OUT_GAME_MODE, 'Fight Out', 2)),
    // Keep Palace visible/clickable in the mode selector. START still
    // normalizes this unsupported world mode to the implemented CS node.
    fieldBytes(1, gameOpeningInfoEntry(BATTLE_ROYALE_PALACE_GAME_MODE, 'Battle Royale Palace', 3))
  ]);
  return Buffer.concat([fieldBytes(1, openingList), fieldVarint(2, 0)]);
}

function rawToAccount(raw) {
  const selected = raw.selected_items && typeof raw.selected_items === 'object' ? raw.selected_items : {};
  return {
    uid: Number(raw.account_id || 100000001),
    token: String(raw.session_token || process.env.SIGMA_SESSION_TOKEN || 'LOCAL_SIGMA_SESSION'),
    nickname: String(raw.display_name || 'LOCAL_SIGMA_TEST'),
    region: String(raw.region || 'IND'),
    role: Number(raw.role || 0),
    selected_items: selected
  };
}

// Every account in the database (multi-account / multiplayer testing). Falls back to the first-row loader.
function loadAccounts(statePath, dbPath) {
  if (dbPath && fs.existsSync(dbPath)) {
    const code = "import json,sqlite3,sys; c=sqlite3.connect(sys.argv[1]); print(json.dumps([json.loads(r[0]) for r in c.execute('SELECT state_json FROM account_state ORDER BY account_id')])); c.close()";
    const result = spawnSync('python3', ['-c', code, dbPath], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] });
    if (result.status === 0 && result.stdout.trim()) {
      try {
        const rows = JSON.parse(result.stdout);
        if (Array.isArray(rows) && rows.length) return rows.filter((r) => r && typeof r === 'object').map(rawToAccount);
      } catch (_) { /* fall through */ }
    }
  }
  return [loadAccount(statePath, dbPath)];
}

function loadAccount(statePath, dbPath) {
  let raw = {};
  // AccountStore treats SQLite as canonical after first boot. Read that same row
  // through Python's standard-library sqlite3 so the sidecar stays native-module-free.
  if (dbPath && fs.existsSync(dbPath)) {
    const code = "import json,sqlite3,sys; c=sqlite3.connect(sys.argv[1]); r=c.execute('SELECT state_json FROM account_state ORDER BY account_id LIMIT 1').fetchone(); print(r[0] if r else '{}'); c.close()";
    const result = spawnSync('python3', ['-c', code, dbPath], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] });
    if (result.status === 0 && result.stdout.trim()) {
      try { raw = JSON.parse(result.stdout); } catch (_) { raw = {}; }
    }
  }
  if (!raw || !Object.keys(raw).length) {
    try { raw = JSON.parse(fs.readFileSync(statePath, 'utf8')); } catch (err) {
      throw new Error(`cannot load old Sigma account state at ${statePath}: ${err.message}`);
    }
  }
  const uid = Number(raw.account_id || 100000001);
  const selected = raw.selected_items && typeof raw.selected_items === 'object' ? raw.selected_items : {};
  return {
    uid,
    token: String(raw.session_token || process.env.SIGMA_SESSION_TOKEN || 'LOCAL_SIGMA_SESSION'),
    nickname: String(raw.display_name || 'LOCAL_SIGMA_TEST'),
    region: String(raw.region || 'IND'),
    role: Number(raw.role || 0),
    selected_items: selected
  };
}

function base64url(value) { return Buffer.from(value).toString('base64').replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_'); }
function makePrepareToken(account, matchId, jwtSecret, rounds = 0) {
  const sel = account.selected_items || {};
  const emoteList = sel.emotes && Array.isArray(sel.emotes.emotes) ? sel.emotes.emotes.map((e) => Number(e.emote_id || 0)) : [];
  const claims = {
    aid: account.uid,
    name: account.nickname,
    region: account.region,
    role: account.role,
    mid: Number(matchId),
    team: 0,
    ...(rounds > 0 ? { rounds } : {}), // match length override (Fight Out = 7 rounds, like a custom room)
    show: {
      avatar: Number(sel.avatar_id || 0),
      color: Number(sel.skin_color || 0),
      head: Number(sel.head_pic || 0),
      banner: Number(sel.banner_id || 0),
      clothes: Array.isArray(sel.clothes) ? sel.clothes : [],
      slots: Array.isArray(sel.slots) ? sel.slots : [],
      emotes: emoteList,
      shows: Array.isArray(sel.shows) ? sel.shows : []
    },
    iat: Math.floor(Date.now() / 1000),
    exp: Math.floor(Date.now() / 1000) + 300
  };
  const head = base64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = base64url(JSON.stringify(claims));
  const signingInput = `${head}.${body}`;
  const sig = crypto.createHmac('sha256', jwtSecret).update(signingInput).digest('base64')
    .replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
  return `${signingInput}.${sig}`;
}

function modeKey(mode) { return `${mode.match_mode}:${mode.game_mode}:${mode.map_id}:${mode.difficulty}`; }

function createMatchmakingServer(options = {}) {
  const statePath = options.statePath || process.env.SIGMA_ACCOUNT_STATE || path.resolve(__dirname, '..', 'data', 'account_state.json');
  const dbPath = options.dbPath || process.env.SIGMA_ACCOUNT_DB || path.resolve(__dirname, '..', 'data', 'sigma.db');
  const account = options.account || loadAccount(statePath, dbPath);
  // Accounts are re-read at every auth so a guest added to the DB works without restarting this process.
  const currentAccounts = () => (options.account ? [options.account] : loadAccounts(statePath, dbPath));
  const bindHost = options.host || process.env.SIGMA_GAME_NODE_HOST || '127.0.0.1';
  const bindPort = Number(options.port ?? process.env.SIGMA_GAME_NODE_PORT ?? 10300);
  const udpPort = Number(options.udpPort ?? process.env.SIGMA_UDP_GAME_NODE_PORT ?? 10200);
  const matchHost = options.matchHost || process.env.SIGMA_MATCH_HOST || process.env.SIGMA_ADVERTISED_HOST || '127.0.0.1';
  const minPlayers = Math.max(1, Number(options.minPlayers ?? process.env.SIGMA_MM_MIN_PLAYERS ?? 2));
  const timeoutMs = Math.max(100, Number(options.timeoutMs ?? process.env.SIGMA_MM_TIMEOUT_MS ?? DEFAULT_MM_TIMEOUT_MS));
  const jwtSecret = options.jwtSecret || process.env.MATCH_JWT_SECRET || 'dev-match-secret-change-me';
  let nextMatchId = BigInt(Date.now()) * 1000n;
  const queues = new Map();
  const timers = new Map();
  const sessions = new Map();

  function sendNotify(entry, protocol, cmd, content) {
    if (entry.socket.destroyed) return;
    const payload = Buffer.concat([
      fieldVarint(1, (entry.account || account).uid),
      fieldVarint(2, protocol),
      fieldVarint(4, cmd),
      fieldBytes(5, content || Buffer.alloc(0))
    ]);
    entry.socket.write(encodeServerFrame(protocol, payload));
  }

  function startContent(mode) {
    return Buffer.concat([
      encodeUIntList(1, [mode.map_id]),
      fieldVarint(2, mode.game_mode),
      fieldVarint(3, mode.difficulty),
      fieldVarint(4, mode.match_mode),
      fieldVarint(5, MM_START_NTF_ESTIMATE_SECONDS)
    ]);
  }

  function sussContent(mode, matchId, acct) {
    const rounds = Number(mode.game_mode) === FIGHT_OUT_GAME_MODE ? FIGHT_OUT_ROUNDS : 0;
    const prepareToken = makePrepareToken(acct || account, matchId, jwtSecret, rounds);
    // IMPORTANT: the Go UDP node can only run Contra Squad (mode 15, map 1). Whatever the
    // player picked in the lobby (Fight Out = 36, Palace = 41, ...), the *loading handoff* must
    // describe the match the UDP node will really serve. Sending 36 here made the client load
    // a Fight Out scene/GRI the server never provides -> stuck forever on the loading screen.
    const handoffMode = SUPPORTED_GAME_MODE;
    return Buffer.concat([
      fieldVarint(1, matchId),
      fieldString(2, `${matchHost}:${udpPort}`),
      fieldString(3, UDP_SECRET),
      fieldString(4, prepareToken),
      fieldVarint(5, 1),
      fieldVarint(6, SUPPORTED_MAP_ID),
      fieldVarint(7, handoffMode),
      fieldVarint(8, 6), // client waits for players; the Go node can fill bots when enabled
      fieldVarint(9, 0),
      fieldVarint(12, mode.difficulty),
      fieldVarint(13, 0)
    ]);
  }

  function removeFromQueues(entry) {
    for (const [key, q] of queues) {
      const remaining = q.entries.filter((item) => item.entry !== entry);
      q.entries = remaining;
      if (!remaining.length) {
        queues.delete(key);
        const timer = timers.get(key);
        if (timer) clearTimeout(timer);
        timers.delete(key);
      }
    }
  }

  function formMatch(key) {
    const q = queues.get(key);
    if (!q || !q.entries.length) return;
    queues.delete(key);
    const timer = timers.get(key);
    if (timer) clearTimeout(timer);
    timers.delete(key);
    const matchId = ++nextMatchId;
    for (const item of q.entries) {
      if (!item.entry.socket.destroyed) {
        sendNotify(item.entry, PROTOCOL_MATCHMAKING, MM_MATCHMAKINGSUSS_NTF, sussContent(item.mode, matchId, item.entry.account));
        console.info(`[tcp-mm] SUSS match=${matchId} uid=${(item.entry.account || account).uid} addr=${matchHost}:${udpPort} requested_mode=${item.mode.game_mode} handoff_mode=${SUPPORTED_GAME_MODE} map=${SUPPORTED_MAP_ID} -> client should now open UDP; look for '[mm-udp] new session' in udp_game_node.console.log`);
      }
    }
  }

  function enqueue(entry, mode) {
    removeFromQueues(entry);
    const key = modeKey(mode);
    let q = queues.get(key);
    if (!q) { q = { mode, entries: [] }; queues.set(key, q); }
    if (!q.entries.some((item) => item.entry === entry)) q.entries.push({ entry, mode });
    if (q.entries.length >= minPlayers) formMatch(key);
    else if (!timers.has(key)) timers.set(key, setTimeout(() => formMatch(key), timeoutMs));
  }

  function decodeApplicationRequest(payload, preferredMode) {
    const modes = preferredMode === 'exact-token-plaintext' ? ['plaintext', 'aes-cbc'] : ['aes-cbc', 'plaintext'];
    const failures = [];
    for (const mode of modes) {
      let bytes;
      try { bytes = mode === 'aes-cbc' ? decrypt(payload) : payload; }
      catch (err) { failures.push(`${mode}: ${err.message}`); continue; }
      try {
        const fields = decodeFields(bytes);
        const command = Number(firstVarint(fields, 1, -1n));
        if (!Number.isSafeInteger(command) || ![MM_START, MM_CANCEL, MM_GAMEOPENINGINFO].includes(command)) {
          throw new Error('missing or unsupported ProtoReq command');
        }
        const commandField = fields.get(1)?.find((item) => item.wire === 0);
        if (!commandField) throw new Error('ProtoReq command field has wrong wire type');
        const data = firstBytes(fields, 2);
        if (command === MM_START && (!data.length || decodeFields(data).size === 0)) {
          throw new Error('empty or malformed MatchmakingStartReq');
        }
        return { fields, command, data, mode };
      } catch (err) { failures.push(`${mode}: ${err.message}`); }
    }
    throw new Error(`${failures.join('; ')}; payload_bytes=${payload.length}`);
  }

  function handleApplication(entry, frame) {
    if (frame.cmd !== PROTOCOL_MATCHMAKING || !frame.payload) return;
    let decoded;
    try { decoded = decodeApplicationRequest(frame.payload, entry.authMode); }
    catch (err) {
      console.warn(`[tcp-mm] uid=${(entry.account || account).uid} app request rejected: ${err.message} hex=${Buffer.from(frame.payload).subarray(0, 16).toString('hex')}`);
      // A tiny "08 NN" ProtoReq (e.g. a status/reconnect query the client sends after it returns to the
      // lobby) is never answered, so the client waits and finally shows "login server time out".
      // Answer it with an empty notify for the same sub-command so the client can move on.
      const raw = Buffer.from(frame.payload);
      if (raw.length === 2 && raw[0] === 0x08 && raw[1] < 0x80) {
        console.info(`[tcp-mm] uid=${(entry.account || account).uid} unknown sub-command ${raw[1]} -> empty reply`);
        sendNotify(entry, PROTOCOL_MATCHMAKING, raw[1], Buffer.alloc(0));
      }
      return;
    }
    const subcmd = decoded.command;
    const data = decoded.data;
    if (decoded.mode === 'plaintext') console.info(`[tcp-mm] uid=${(entry.account || account).uid} app payload mode=plaintext (authenticated)`);
    if (subcmd === MM_GAMEOPENINGINFO) {
      // The mode-selection screen requests opening/event info before START.
      // Respond with a valid empty GameOpeningInfoRes instead of dropping it.
      sendNotify(entry, PROTOCOL_MATCHMAKING, MM_GAMEOPENINGINFO, gameOpeningInfoContent());
      console.info(`[tcp-mm] GAMEOPENINGINFO uid=${(entry.account || account).uid} region=${entry.region} request_bytes=${data.length} response=contra-squad-only`);
    } else if (subcmd === MM_START) {
      let request;
      try { request = decodeFields(data); }
      catch (err) { console.warn(`[tcp-mm] uid=${(entry.account || account).uid} start decode failed: ${err.message}`); return; }
      const maps = repeatedUInt32(request, 1);
      const requestedMap = maps[0] || SUPPORTED_MAP_ID;
      const requestedGameMode = Number(firstVarint(request, 2, BigInt(SUPPORTED_GAME_MODE))) || SUPPORTED_GAME_MODE;
      const supported = SUPPORTED_GAME_MODES.has(requestedGameMode);
      const mode = {
        map_id: supported ? requestedMap : SUPPORTED_MAP_ID,
        // 36 is a CS-compatible alias for the client-facing Fight Out mode;
        // preserve it in START_NTF while the UDP node remains CS mode 15.
        game_mode: supported ? requestedGameMode : SUPPORTED_GAME_MODE,
        difficulty: Number(firstVarint(request, 4, 0n)),
        match_mode: Number(firstVarint(request, 8, 1n)) || 1
      };
      if (!supported) {
        console.warn(`[tcp-mm] unsupported game_mode=${requestedGameMode}; falling back to game_mode=${SUPPORTED_GAME_MODE} map=${SUPPORTED_MAP_ID}`);
      }
      sendNotify(entry, PROTOCOL_MATCHMAKING, MM_START_NTF, startContent(mode));
      enqueue(entry, mode);
      console.info(`[tcp-mm] START uid=${(entry.account || account).uid} mode=${modeKey(mode)}`);
    } else if (subcmd === MM_CANCEL) {
      removeFromQueues(entry);
      sendNotify(entry, PROTOCOL_MATCHMAKING, MM_STOP_NTF, Buffer.alloc(0));
      console.info(`[tcp-mm] CANCEL uid=${(entry.account || account).uid}`);
    } else {
      console.info(`[tcp-mm] unrouted protocol=${frame.cmd} subcmd=${subcmd}`);
    }
  }

  const server = net.createServer((socket) => {
    socket.setNoDelay(true);
    const peer = `${socket.remoteAddress}:${socket.remotePort}`;
    let pending = Buffer.alloc(0);
    const entry = { socket, authenticated: false, region: 0 };
    console.info(`[tcp-mm] connected ${peer}`);
    socket.on('data', (chunk) => {
      pending = Buffer.concat([pending, chunk]);
      if (pending.length > MAX_FRAME_BYTES * 8) { socket.destroy(); return; }
      let parsed;
      try { parsed = parseClientFrames(pending); }
      catch (err) { console.warn(`[tcp-mm] bad frame from ${peer}: ${err.message}`); socket.destroy(); return; }
      pending = Buffer.from(parsed.rest);
      for (const frame of parsed.frames) {
        if (frame.cmd === CMD.HEARTBEAT) {
          socket.write(INIT_ACK);
          continue;
        }
        if (frame.cmd === CMD.AUTH) {
          // Some Sigma builds emit an empty Cmd=1 connection-control frame before
          // sending the session token. ACK it, but keep the socket unauthenticated.
          if (!frame.payload || frame.payload.length === 0) {
            socket.write(INIT_ACK);
            console.info(`[tcp-mm] empty AUTH/control ${peer}; awaiting session token`);
            continue;
          }
          let token = '';
          let authMode = 'aes-cbc';
          let decryptError = null;
          try { token = decrypt(frame.payload).toString('utf8'); }
          catch (err) { decryptError = err; }
          // Compatibility for a client build with TCPTokenEncrypt disabled: accept
          // plaintext only if the complete payload exactly matches the canonical
          // local session token. Never accept arbitrary or partially decoded text.
          const candidates = currentAccounts();
          let acct = candidates.find((a) => token && token === a.token);
          if (!acct) {
            const plain = frame.payload.toString('utf8');
            acct = candidates.find((a) => plain === a.token);
            if (acct) { token = acct.token; authMode = 'exact-token-plaintext'; }
          }
          if (!token || !acct) {
            const reason = decryptError ? decryptError.message : 'decrypted token mismatch';
            console.warn(`[tcp-mm] auth rejected ${peer}: ${reason}; payload_bytes=${frame.payload.length}`);
            socket.destroy(); return;
          }
          entry.account = acct;
          const previous = sessions.get(acct.uid);
          if (previous && previous.socket !== socket) {
            removeFromQueues(previous);
            previous.socket.destroy();
          }
          entry.authenticated = true;
          entry.region = frame.region;
          entry.authMode = authMode;
          sessions.set(acct.uid, entry);
          socket.write(INIT_ACK);
          console.info(`[tcp-mm] authenticated uid=${acct.uid} region=${entry.region} mode=${authMode} (${peer})`);
          continue;
        }
        if (!entry.authenticated) { console.warn(`[tcp-mm] app frame before auth (${peer})`); socket.destroy(); return; }
        handleApplication(entry, frame);
      }
    });
    socket.on('close', () => {
      removeFromQueues(entry);
      if (entry.account && sessions.get(entry.account.uid) === entry) sessions.delete(entry.account.uid);
      console.info(`[tcp-mm] disconnected ${peer}`);
    });
    socket.on('error', (err) => console.warn(`[tcp-mm] socket ${peer}: ${err.code || err.message}`));
  });

  return { server, account, listen: () => server.listen(bindPort, bindHost), close: () => {
    for (const timer of timers.values()) clearTimeout(timer);
    timers.clear(); queues.clear();
    return new Promise((resolve) => server.close(() => resolve()));
  } };
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--host') out.host = argv[++i];
    else if (argv[i] === '--port') out.port = Number(argv[++i]);
    else if (argv[i] === '--udp-port') out.udpPort = Number(argv[++i]);
    else if (argv[i] === '--state') out.statePath = path.resolve(argv[++i]);
    else if (argv[i] === '--db') out.dbPath = path.resolve(argv[++i]);
  }
  return out;
}

if (require.main === module) {
  const instance = createMatchmakingServer(parseArgs(process.argv.slice(2)));
  instance.server.on('error', (err) => { console.error(`[tcp-mm] listen error: ${err.message}`); process.exitCode = 1; });
  instance.server.on('listening', () => {
    const addr = instance.server.address();
    console.info(`[tcp-mm] listening on ${addr.address}:${addr.port} for uid=${instance.account.uid}`);
  });
  instance.listen();
  const stop = () => instance.close().finally(() => process.exit(0));
  process.on('SIGINT', stop);
  process.on('SIGTERM', stop);
}

module.exports = { createMatchmakingServer, loadAccount, loadAccounts, decodeFields, encodeVarint, fieldVarint, fieldBytes, makePrepareToken, constants: { PROTOCOL_MATCHMAKING, MM_START, MM_CANCEL, MM_GAMEOPENINGINFO, MM_MATCHMAKINGSUSS_NTF, MM_START_NTF, MM_STOP_NTF, DEFAULT_MM_TIMEOUT_MS, UDP_SECRET } };
