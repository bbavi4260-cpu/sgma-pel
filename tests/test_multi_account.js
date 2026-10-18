'use strict';
// Two different accounts (two rows in sigma.db) authenticate on the SAME matchmaking server, queue together
// and each receives a prepare token for ITS OWN account id, with the SAME match id (multiplayer test).
const test = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { createMatchmakingServer, decodeFields, constants } = require('../transport/tcp_matchmaking');
const { encrypt } = require('../transport/framing');

function varint(n) { let v = BigInt(n); const o = []; while (v > 127n) { o.push(Number((v & 127n) | 128n)); v >>= 7n; } o.push(Number(v)); return Buffer.from(o); }
const bytesField = (id, b) => Buffer.concat([varint((id << 3) | 2), varint(b.length), b]);
const uintField = (id, n) => Buffer.concat([varint(id << 3), varint(n)]);
function clientFrame(cmd, payload, region = 1) { const h = Buffer.alloc(6); h[0] = cmd; h[1] = region; h.writeUInt32BE(payload.length, 2); return Buffer.concat([h, payload]); }
class Reader {
  constructor(s) { this.buf = Buffer.alloc(0); this.w = []; s.on('data', (c) => { this.buf = Buffer.concat([this.buf, c]); this.flush(); }); }
  flush() { while (this.w.length && this.buf.length >= this.w[0].n) { const x = this.w.shift(); const o = this.buf.subarray(0, x.n); this.buf = this.buf.subarray(x.n); x.res(o); } }
  read(n) { return new Promise((res) => { this.w.push({ n, res }); this.flush(); }); }
  async frame() { const h = await this.read(5); return { cmd: h[0], payload: await this.read(h.readUInt32BE(1)) }; }
}

test('two guest accounts join the same match with their own ids', async (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mm2-'));
  const db = path.join(dir, 'sigma.db');
  const py = `import json,sqlite3,sys
c=sqlite3.connect(sys.argv[1]); c.execute('create table account_state(account_id integer primary key,state_json text,updated_at text)')
for i,tok in ((100000001,'tok-one-aaaaaaaa'),(100000002,'tok-two-bbbbbbbb')):
    c.execute('insert into account_state values(?,?,?)',(i,json.dumps({'account_id':i,'session_token':tok,'display_name':'P%d'%i,'region':'IND','role':1}),'x'))
c.commit()`;
  assert.equal(spawnSync('python3', ['-c', py, db]).status, 0);
  const app = createMatchmakingServer({ dbPath: db, statePath: path.join(dir, 'none.json'), minPlayers: 2, timeoutMs: 10000, matchHost: '127.0.0.1', udpPort: 10200 });
  await new Promise((r) => app.server.listen(0, '127.0.0.1', r));
  const { port } = app.server.address();
  const socks = [];
  t.after(async () => { socks.forEach((s) => s.destroy()); await new Promise((r) => app.server.close(r)); });

  async function login(token) {
    const s = net.createConnection({ host: '127.0.0.1', port });
    await new Promise((r) => s.once('connect', r));
    socks.push(s);
    const rd = new Reader(s);
    s.write(clientFrame(1, encrypt(Buffer.from(token))));
    assert.deepEqual(await rd.read(1), Buffer.from([2]));
    return { s, rd };
  }
  const a = await login('tok-one-aaaaaaaa');
  const b = await login('tok-two-bbbbbbbb');
  const start = Buffer.concat([uintField(1, 1), uintField(2, 15), uintField(4, 2), uintField(8, 1)]);
  const req = encrypt(Buffer.concat([uintField(1, constants.MM_START), bytesField(2, start)]));
  const result = [];
  for (const p of [a, b]) {
    p.s.write(clientFrame(3, req));
    const f1 = await p.rd.frame();
    assert.equal(Number(decodeFields(f1.payload).get(4)[0].value), constants.MM_START_NTF);
  }
  for (const p of [a, b]) {
    const f = await p.rd.frame();
    const n = decodeFields(f.payload);
    assert.equal(Number(n.get(4)[0].value), constants.MM_MATCHMAKINGSUSS_NTF);
    const suss = decodeFields(n.get(5)[0].value);
    const jwt = suss.get(4)[0].value.toString().split('.');
    result.push({ notifyId: Number(n.get(1)[0].value), matchId: suss.get(1)[0].value.toString(), aid: JSON.parse(Buffer.from(jwt[1], 'base64url').toString()).aid });
  }
  assert.deepEqual(result.map((x) => x.aid), [100000001, 100000002]);
  assert.deepEqual(result.map((x) => x.notifyId), [100000001, 100000002]);
  assert.equal(result[0].matchId, result[1].matchId);
});
