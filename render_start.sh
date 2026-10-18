#!/usr/bin/env bash
# Render start script: runs all match-related services in one container.
set -uo pipefail
cd "$(dirname "$0")"

HOST="0.0.0.0"
PORT="${PORT:-3000}"                                   # Render sets this (public HTTP)
NOTIFICATION_PORT="${SIGMA_NOTIFICATION_PORT:-10301}"
GAME_NODE_PORT="${SIGMA_GAME_NODE_PORT:-10300}"
UDP_GAME_NODE_PORT="${SIGMA_UDP_GAME_NODE_PORT:-10200}"
DB_PATH="$PWD/data/sigma.db"
MATCH_HOST="${SIGMA_MATCH_HOST:-${RENDER_EXTERNAL_HOSTNAME:-127.0.0.1}}"
export SIGMA_MATCH_HOST="$MATCH_HOST"
export SIGMA_GAME_NODE="${SIGMA_GAME_NODE:-${MATCH_HOST}:${GAME_NODE_PORT}}"
export SIGMA_NOTIFICATION_CHANNEL="${SIGMA_MATCHMAKING_CHANNEL:-${MATCH_HOST}:${GAME_NODE_PORT}}"
export MATCH_JWT_SECRET="${MATCH_JWT_SECRET:-dev-match-secret-change-me}"
export MATCH_UDP_SECRET_HEX="${MATCH_UDP_SECRET_HEX:-00112233445566778899aabbccddeeff}"
export MATCH_BOT="${MATCH_BOT:-1}"
export MATCH_SEND_110="${MATCH_SEND_110:-0}"
export MATCH_DEBUG_POS="${MATCH_DEBUG_POS:-0}"
export MATCH_NO_ZONE="${MATCH_NO_ZONE:-1}"
export MATCH_BOT_COUNT="${MATCH_BOT_COUNT:-5}"
export MATCH_BOT_ALLIES="${MATCH_BOT_ALLIES:-0}"
export MATCH_BOT_ENEMIES="${MATCH_BOT_ENEMIES:-0}"

mkdir -p logs data resources
[ -x logs/sigma_match_server ] || (cd match-server && go build -o ../logs/sigma_match_server .) || exit 1

PIDS=()
cleanup() { trap - INT TERM EXIT; kill "${PIDS[@]}" 2>/dev/null; exit 0; }
trap cleanup INT TERM EXIT

# 1) HTTP + SQLite (owns account/session state) - start first
python3 -u test_server.py --host "$HOST" --port "$PORT" --root "$PWD" & PIDS+=($!)
for _ in $(seq 1 40); do
  python3 -c "import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute('SELECT state_json FROM account_state LIMIT 1').fetchone()" "$DB_PATH" >/dev/null 2>&1 && break
  sleep 0.25
done
# 2) Go UDP match server
logs/sigma_match_server -addr ":${UDP_GAME_NODE_PORT}" & PIDS+=($!)
# 3) Notification / presence gateway
python3 -u notification_gateway.py --host "$HOST" --port "$NOTIFICATION_PORT" --log-dir "$PWD/logs" & PIDS+=($!)
# 4) Node TCP matchmaking
node transport/tcp_matchmaking.js --host "$HOST" --port "$GAME_NODE_PORT" --udp-port "$UDP_GAME_NODE_PORT" --state "$PWD/data/account_state.json" --db "$DB_PATH" & PIDS+=($!)

# If any service dies, exit so Render restarts the whole thing
wait -n
echo "A service exited; shutting down." >&2
exit 1
