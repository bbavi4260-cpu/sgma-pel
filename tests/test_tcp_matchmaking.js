'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const crypto = require('node:crypto');
const { createMatchmakingServer, decodeFields, fieldVarint, fieldBytes, constants } = require('../transport/tcp_matchmaking');
const { encrypt, encodeServerFrame, parseClientFrames } = require('../transport/framing');

function varint(n) {
  let v = BigInt(n); const out = [];
  while (v > 127n) { out.push(Number((v & 127n) | 128n)); v >>= 7n; }
  out.push(Number(v)); return Buffer.from(out);
}
function bytesField(id, b) { return Buffer.concat([varint((id << 3) | 2), varint(b.length), b]); }
function uintField(id, n) { return Buffer.concat([varint(id << 3), varint(n)]); }
function clientFrame(cmd, payload, region = 1) {
  if (cmd === 2) return Buffer.from([2, region]);
  const head = Buffer.alloc(6); head[0] = cmd; head[1] = region; head.writeUInt32BE(payload.length, 2);
  return Buffer.concat([head, payload]);
}

class Reader {
  constructor(socket) {
    this.buffer = Buffer.alloc(0); this.waiters = [];
    socket.on('data', (chunk) => { this.buffer = Buffer.concat([this.buffer, chunk]); this.flush(); });
  }
  flush() {
    while (this.waiters.length && this.buffer.length >= this.waiters[0].n) {
      const waiter = this.waiters.shift();
      const out = this.buffer.subarray(0, waiter.n); this.buffer = this.buffer.subarray(waiter.n); waiter.resolve(out);
    }
  }
  read(n) { return new Promise((resolve, reject) => { this.waiters.push({ n, resolve, reject }); this.flush(); }); }
  async frame() {
    const head = await this.read(5); const len = head.readUInt32BE(1);
    return { cmd: head[0], payload: await this.read(len) };
  }
}

function parseNotify(payload) {
  const fields = decodeFields(payload);
  return {
    accountId: Number(fields.get(1)?.[0]?.value || 0n),
    protocol: Number(fields.get(2)?.[0]?.value || 0n),
    cmd: Number(fields.get(4)?.[0]?.value || 0n),
    content: fields.get(5)?.[0]?.value || Buffer.alloc(0)
  };
}

test('client TCP frames use Cmd+Region+BE-length; partial input is buffered', () => {
  const payload = Buffer.from([1, 2, 3, 4]);
  const frame = clientFrame(3, payload, 1);
  const first = parseClientFrames(frame.subarray(0, 4));
  assert.equal(first.frames.length, 0);
  const second = parseClientFrames(Buffer.concat([first.rest, frame.subarray(4)]));
  assert.equal(second.frames.length, 1);
  assert.equal(second.frames[0].cmd, 3);
  assert.equal(second.frames[0].region, 1);
  assert.deepEqual(second.frames[0].payload, payload);
});

test('solo matchmaking wait default is short (3s) so a start does not feel frozen', () => {
  assert.equal(constants.DEFAULT_MM_TIMEOUT_MS, 3000);
});

test('TCP auth, heartbeat, matchmaking START_NTF, then SussNtf JWT handoff', async (t) => {
  const account = { uid: 100000001, token: 'test-session-token', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, minPlayers: 2, timeoutMs: 80, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  const reader = new Reader(socket);
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });

  // A build may send an empty Cmd=1 connection-control before the actual token.
  socket.write(clientFrame(1, Buffer.alloc(0), 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  const authCipher = encrypt(Buffer.from(account.token));
  socket.write(clientFrame(1, authCipher, 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));

  socket.write(Buffer.from([2, 1]));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));

  const startReq = Buffer.concat([uintField(1, 1), uintField(2, 15), uintField(4, 2), uintField(8, 1)]);
  const protoReq = Buffer.concat([uintField(1, constants.MM_START), bytesField(2, startReq)]);
  socket.write(clientFrame(3, encrypt(protoReq), 1));
  const startFrame = await reader.frame();
  assert.equal(startFrame.cmd, 3);
  const startNotify = parseNotify(startFrame.payload);
  assert.equal(startNotify.accountId, account.uid);
  assert.equal(startNotify.protocol, 3);
  assert.equal(startNotify.cmd, constants.MM_START_NTF);
  const startContent = decodeFields(startNotify.content);
  assert.equal(Number(startContent.get(2)[0].value), 15);
  assert.equal(Number(startContent.get(3)[0].value), 2);
  assert.equal(Number(startContent.get(5)[0].value), 15);

  const matchFrame = await reader.frame();
  assert.equal(matchFrame.cmd, 3);
  const matchNotify = parseNotify(matchFrame.payload);
  assert.equal(matchNotify.cmd, constants.MM_MATCHMAKINGSUSS_NTF);
  const suss = decodeFields(matchNotify.content);
  assert.equal(suss.get(2)[0].value.toString(), '127.0.0.1:10200');
  assert.equal(Number(suss.get(12)[0].value), 2);
  const jwt = suss.get(4)[0].value.toString();
  const [header, body, signature] = jwt.split('.');
  const expected = crypto.createHmac('sha256', 'dev-match-secret-change-me').update(`${header}.${body}`).digest('base64url');
  assert.equal(signature, expected);
  assert.equal(JSON.parse(Buffer.from(body, 'base64url').toString()).aid, account.uid);
});

test('mode selection GameOpeningInfo cmd 7 advertises CS and Fight Out metadata', async (t) => {
  const account = { uid: 100000004, token: 'opening-info-token', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, minPlayers: 2, timeoutMs: 10000 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  const reader = new Reader(socket);
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });

  socket.write(clientFrame(1, Buffer.from(account.token), 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  socket.write(Buffer.from([2, 1]));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));

  const reqData = Buffer.concat([bytesField(1, Buffer.from('IND')), bytesField(2, Buffer.from('en'))]);
  const protoReq = Buffer.concat([uintField(1, constants.MM_GAMEOPENINGINFO), bytesField(2, reqData)]);
  socket.write(clientFrame(3, protoReq, 1)); // plaintext request from this authenticated client build

  const response = await reader.frame();
  assert.equal(response.cmd, 3);
  const notify = parseNotify(response.payload);
  assert.equal(notify.accountId, account.uid);
  assert.equal(notify.protocol, 3);
  assert.equal(notify.cmd, constants.MM_GAMEOPENINGINFO);
  const responseFields = decodeFields(notify.content);
  const list = decodeFields(responseFields.get(1)[0].value);
  const mode = decodeFields(list.get(1)[0].value);
  assert.equal(Number(mode.get(1)[0].value), 1); // map_id
  assert.equal(Number(mode.get(13)[0].value), 15); // Contra Squad
  assert.equal(Number(mode.get(14)[0].value), 1); // match mode
  const fightOut = decodeFields(list.get(1)[1].value);
  assert.equal(Number(fightOut.get(13)[0].value), 36); // Fight Out / CS StrikeOut alias
  assert.equal(fightOut.get(3)[0].value.toString(), 'Fight Out');
  const palace = decodeFields(list.get(1)[2].value);
  assert.equal(Number(palace.get(13)[0].value), 41); // Battle Royale Palace selector entry
  assert.equal(palace.get(3)[0].value.toString(), 'Battle Royale Palace');
});

test('Fight Out START echoes mode 36 in START_NTF but hands off the CS mode the UDP node serves', async (t) => {
  const account = { uid: 100000006, token: 'fight-out-token', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, minPlayers: 2, timeoutMs: 10000, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  const reader = new Reader(socket);
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });
  socket.write(clientFrame(1, Buffer.from(account.token), 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  socket.write(Buffer.from([2, 1]));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  const req = Buffer.concat([uintField(1, 1), uintField(2, 36), uintField(4, 2), uintField(8, 1)]);
  socket.write(clientFrame(3, encrypt(Buffer.concat([uintField(1, constants.MM_START), bytesField(2, req)])), 1));
  const start = parseNotify((await reader.frame()).payload);
  assert.equal(Number(decodeFields(start.content).get(2)[0].value), 36);
  const suss = parseNotify((await reader.frame()).payload);
  // The UDP node only runs Contra Squad: handing off 36 left the client stuck on loading.
  assert.equal(Number(decodeFields(suss.content).get(7)[0].value), 15);
  assert.equal(Number(decodeFields(suss.content).get(6)[0].value), 1);
});

test('unsupported Battle Royale START is normalized to the implemented CS handoff', async (t) => {
  const account = { uid: 100000005, token: 'br-fallback-token', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, minPlayers: 2, timeoutMs: 10000, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  const reader = new Reader(socket);
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });
  socket.write(clientFrame(1, Buffer.from(account.token), 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  socket.write(Buffer.from([2, 1]));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  const brReq = Buffer.concat([uintField(1, 1), uintField(2, 41), uintField(4, 2), uintField(8, 1)]);
  socket.write(clientFrame(3, encrypt(Buffer.concat([uintField(1, constants.MM_START), bytesField(2, brReq)])), 1));
  const start = parseNotify((await reader.frame()).payload);
  assert.equal(Number(decodeFields(start.content).get(2)[0].value), 15);
  const suss = parseNotify((await reader.frame()).payload);
  assert.equal(Number(decodeFields(suss.content).get(7)[0].value), 15);
  assert.equal(Number(decodeFields(suss.content).get(6)[0].value), 1);
});


test('plain auth compatibility accepts only an exact local session token', async (t) => {
  const account = { uid: 100000002, token: 'plain-test-session', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, minPlayers: 2, timeoutMs: 10000, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  const reader = new Reader(socket);
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });
  socket.write(clientFrame(1, Buffer.from(account.token), 1));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  socket.write(Buffer.from([2, 1]));
  assert.deepEqual(await reader.read(1), Buffer.from([2]));
  const startReq = Buffer.concat([uintField(1, 1), uintField(2, 15), uintField(4, 2), uintField(8, 1)]);
  const protoReq = Buffer.concat([uintField(1, constants.MM_START), bytesField(2, startReq)]);
  socket.write(clientFrame(3, protoReq, 1)); // plaintext ProtoReq from this client build
  const startFrame = await reader.frame();
  const startNotify = parseNotify(startFrame.payload);
  assert.equal(startNotify.cmd, constants.MM_START_NTF);
  assert.equal(Number(decodeFields(startNotify.content).get(3)[0].value), 2);
});

test('rejects a nonmatching plaintext auth payload', async (t) => {
  const account = { uid: 100000003, token: 'canonical-session', nickname: 'SigmaTest', region: 'IND', role: 0, selected_items: {} };
  const app = createMatchmakingServer({ account, timeoutMs: 100, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((resolve) => app.server.listen(0, '127.0.0.1', resolve));
  const { port } = app.server.address();
  const socket = net.createConnection({ host: '127.0.0.1', port });
  await new Promise((resolve, reject) => { socket.once('connect', resolve); socket.once('error', reject); });
  t.after(async () => { socket.destroy(); await new Promise((resolve) => app.server.close(resolve)); });
  const closed = new Promise((resolve) => socket.once('close', resolve));
  socket.write(clientFrame(1, Buffer.from('not-the-session-token'), 1));
  await closed;
});
