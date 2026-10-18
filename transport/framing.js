'use strict';

// Selective port of the reviewed PrjShadow TCP transport framing.
// Client -> server: Cmd, Region, [u32 BE length, AES payload], except heartbeat.
// Server -> client: Cmd, [u32 BE length, plaintext protobuf], except heartbeat ACK.
const crypto = require('node:crypto');

const KEY = Buffer.from('1860A234CF6F8B853DF690344BE791DD', 'hex');
const IV = Buffer.from('02F5989EC026B2DFB24421AD1A9F908D', 'hex');
const CMD = Object.freeze({ AUTH: 1, HEARTBEAT: 2, KICK: 11 });
const INIT_ACK = Buffer.from([CMD.HEARTBEAT]);
const MAX_FRAME_BYTES = 20 * 1024; // client EProtocol.MAXPACKETSIZE

function encrypt(plain) {
  const c = crypto.createCipheriv('aes-128-cbc', KEY, IV);
  return Buffer.concat([c.update(plain), c.final()]);
}

function decrypt(cipher) {
  if (!Buffer.isBuffer(cipher) || cipher.length === 0 || cipher.length % 16 !== 0) {
    throw new Error('invalid AES-CBC ciphertext length');
  }
  const d = crypto.createDecipheriv('aes-128-cbc', KEY, IV);
  return Buffer.concat([d.update(cipher), d.final()]);
}

function parseClientFrames(buf) {
  const frames = [];
  let off = 0;
  while (buf.length - off >= 2) {
    const cmd = buf[off];
    const region = buf[off + 1];
    if (cmd === CMD.HEARTBEAT) {
      frames.push({ cmd, region, payload: null });
      off += 2;
      continue;
    }
    if (buf.length - off < 6) break;
    const len = buf.readUInt32BE(off + 2);
    if (len > MAX_FRAME_BYTES) throw new Error(`frame too large (${len})`);
    if (buf.length - off < 6 + len) break;
    frames.push({ cmd, region, payload: buf.subarray(off + 6, off + 6 + len) });
    off += 6 + len;
  }
  return { frames, rest: buf.subarray(off) };
}

function encodeServerFrame(cmd, payload) {
  if (cmd === CMD.HEARTBEAT) return INIT_ACK;
  const p = payload || Buffer.alloc(0);
  if (p.length > MAX_FRAME_BYTES) throw new Error(`frame too large (${p.length})`);
  const head = Buffer.alloc(5);
  head[0] = cmd;
  head.writeUInt32BE(p.length, 1);
  return Buffer.concat([head, p]);
}

module.exports = { KEY, IV, CMD, INIT_ACK, MAX_FRAME_BYTES, encrypt, decrypt, parseClientFrames, encodeServerFrame };
