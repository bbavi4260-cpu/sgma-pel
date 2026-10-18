#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Optional project settings; absence of .env is harmless.
if [ -f .env ]; then set -a; . ./.env; set +a; fi

PORT="${SIGMA_TEST_PORT:-3000}"
HOST="${SIGMA_TEST_HOST:-0.0.0.0}"
NOTIFICATION_PORT="${SIGMA_NOTIFICATION_PORT:-10301}"
GAME_NODE_PORT="${SIGMA_GAME_NODE_PORT:-10300}"
UDP_GAME_NODE_PORT="${SIGMA_UDP_GAME_NODE_PORT:-10200}"
DB_PATH="$PWD/data/sigma.db"
MATCH_HOST="${SIGMA_MATCH_HOST:-${SIGMA_ADVERTISED_HOST:-127.0.0.1}}"
export SIGMA_MATCH_HOST="$MATCH_HOST"
export SIGMA_GAME_NODE="${SIGMA_GAME_NODE:-${MATCH_HOST}:${GAME_NODE_PORT}}"
export SIGMA_NOTIFICATION_CHANNEL="${SIGMA_MATCHMAKING_CHANNEL:-${MATCH_HOST}:${GAME_NODE_PORT}}"

mkdir -p logs data resources

if ! command -v node >/dev/null 2>&1; then
  echo "Node.js is required for the TCP matchmaking gateway. In Termux: pkg install nodejs-lts"
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go is required for the UDP match node. In Termux: pkg install golang"
  exit 1
fi

for spec in "HTTP:$PORT:tcp" "Notification:$NOTIFICATION_PORT:tcp" "TCP-matchmaking:$GAME_NODE_PORT:tcp" "UDP-match:$UDP_GAME_NODE_PORT:udp"; do
  IFS=: read -r name port proto <<< "$spec"
  if command -v ss >/dev/null 2>&1; then
    if [ "$proto" = tcp ] && ss -ltn 2>/dev/null | grep -qE ":${port}[[:space:]]"; then
      echo "$name port $port is already occupied. Stop the other process and retry."
      exit 1
    fi
    if [ "$proto" = udp ] && ss -lun 2>/dev/null | grep -qE ":${port}[[:space:]]"; then
      echo "$name port $port is already occupied. Stop the other process and retry."
      exit 1
    fi
  fi
done

# Build Go source locally; generated binary is kept under logs/ and excluded from the package.
echo "Building local UDP match node (first build may take a while)..."
if ! (cd match-server && go build -o ../logs/sigma_match_server .); then
  echo "Go match-node build failed; review the error above and match-server/go.mod."
  exit 1
fi

rm -f logs/server.pid logs/notification.pid logs/game_node.pid logs/udp_game_node.pid
export MATCH_JWT_SECRET="${MATCH_JWT_SECRET:-dev-match-secret-change-me}"
# 16-byte UDP TEA key shared by the Go UDP node and the Node matchmaker (which sends it to the client).
# The Go node refuses to start without it. Default = the key hardcoded in transport/tcp_matchmaking.js.
export MATCH_UDP_SECRET_HEX="${MATCH_UDP_SECRET_HEX:-00112233445566778899aabbccddeeff}"
export MATCH_BOT="${MATCH_BOT:-1}"
# Test defaults (override with e.g. MATCH_SEND_110=0 bash termux_start_server.sh):
export MATCH_SEND_110="${MATCH_SEND_110:-0}"     # send cmd 110 BattleStart after the join burst (experiment)
export MATCH_DEBUG_POS="${MATCH_DEBUG_POS:-0}"   # log every client position packet (cmd 1001)
export MATCH_NO_ZONE="${MATCH_NO_ZONE:-1}"       # no safe-zone damage while testing
export MATCH_BOT_COUNT="${MATCH_BOT_COUNT:-5}"
# 1v1 by default. For 4v4 run: MATCH_BOT_ALLIES=3 MATCH_BOT_ENEMIES=3 bash termux_start_server.sh (heavier, can lag).
export MATCH_BOT_ALLIES="${MATCH_BOT_ALLIES:-0}"
export MATCH_BOT_ENEMIES="${MATCH_BOT_ENEMIES:-0}"

cleanup() {
  trap - INT TERM EXIT
  echo
  echo "Stopping Sigma services..."
  [ -n "${HTTP_TAIL_PID:-}" ] && kill "$HTTP_TAIL_PID" 2>/dev/null || true
  [ -n "${NOTIFY_TAIL_PID:-}" ] && kill "$NOTIFY_TAIL_PID" 2>/dev/null || true
  [ -n "${GAME_TAIL_PID:-}" ] && kill "$GAME_TAIL_PID" 2>/dev/null || true
  [ -n "${UDP_GAME_TAIL_PID:-}" ] && kill "$UDP_GAME_TAIL_PID" 2>/dev/null || true
  [ -s logs/server.pid ] && kill "$(cat logs/server.pid)" 2>/dev/null || true
  [ -s logs/notification.pid ] && kill "$(cat logs/notification.pid)" 2>/dev/null || true
  [ -s logs/game_node.pid ] && kill "$(cat logs/game_node.pid)" 2>/dev/null || true
  [ -s logs/udp_game_node.pid ] && kill "$(cat logs/udp_game_node.pid)" 2>/dev/null || true
  rm -f logs/server.pid logs/notification.pid logs/game_node.pid logs/udp_game_node.pid
  echo "Sigma HTTP, TCP matchmaking, UDP match, and notification services stopped."
  exit 0
}
trap cleanup INT TERM EXIT

# Start the old Python HTTP/SQLite service first; it owns the canonical account/session token.
python3 -u test_server.py --host "$HOST" --port "$PORT" --root "$PWD" > logs/console.log 2>&1 &
HTTP_PID=$!
echo "$HTTP_PID" > logs/server.pid

account_db_ready() {
  python3 -c "import sqlite3,sys; c=sqlite3.connect(sys.argv[1]); c.execute('SELECT state_json FROM account_state LIMIT 1').fetchone(); c.close()" "$DB_PATH" >/dev/null 2>&1
}
for _ in $(seq 1 40); do
  if account_db_ready; then break; fi
  if ! kill -0 "$HTTP_PID" 2>/dev/null; then
    echo "Old HTTP/SQLite server exited during startup; inspect logs/console.log"
    cleanup
  fi
  sleep 0.25
done
if ! account_db_ready; then
  echo "SQLite account state did not initialize; inspect logs/console.log"
  cleanup
fi

# Go UDP match node is needed by the matchmaking success handoff. Redis is optional.
logs/sigma_match_server -addr ":${UDP_GAME_NODE_PORT}" > logs/udp_game_node.console.log 2>&1 &
UDP_GAME_NODE_PID=$!
echo "$UDP_GAME_NODE_PID" > logs/udp_game_node.pid

# Keep the old notification service and expose the new PrjShadow-framed Sigma matcher at 10300.
python3 -u notification_gateway.py --host "$HOST" --port "$NOTIFICATION_PORT" --log-dir "$PWD/logs" > logs/notification.console.log 2>&1 &
NOTIFICATION_PID=$!
echo "$NOTIFICATION_PID" > logs/notification.pid

node transport/tcp_matchmaking.js --host "$HOST" --port "$GAME_NODE_PORT" --udp-port "$UDP_GAME_NODE_PORT" --state "$PWD/data/account_state.json" --db "$DB_PATH" > logs/game_node.console.log 2>&1 &
GAME_NODE_PID=$!
echo "$GAME_NODE_PID" > logs/game_node.pid

sleep 1
if ! kill -0 "$HTTP_PID" 2>/dev/null || ! kill -0 "$NOTIFICATION_PID" 2>/dev/null || ! kill -0 "$GAME_NODE_PID" 2>/dev/null || ! kill -0 "$UDP_GAME_NODE_PID" 2>/dev/null; then
  echo "A service failed to start. Inspect logs/console.log, logs/notification.console.log, logs/game_node.console.log, and logs/udp_game_node.console.log"
  cleanup
fi

echo "Sigma V38 (old HTTP/SQLite) + TCP matchmaking started."
echo "HTTP listener: ${HOST}:${PORT}"
echo "Notification listener: ${HOST}:${NOTIFICATION_PORT} (legacy Python service retained)"
echo "TCP matchmaking listener: ${HOST}:${GAME_NODE_PORT} (PrjShadow frame/auth + Sigma matchmaking messages)"
echo "UDP match listener: :${UDP_GAME_NODE_PORT} (PrjShadow Go match node; MATCH_BOT=${MATCH_BOT})"
echo "SQLite database: $DB_PATH"
echo "Client-reachable match host: ${SIGMA_MATCH_HOST:-${SIGMA_ADVERTISED_HOST:-127.0.0.1}}"
echo "Live logs are shown below. Press Ctrl-C to stop all services."
echo "Persistent logs: $PWD/logs"
echo

tail -n 0 -F logs/console.log & HTTP_TAIL_PID=$!
tail -n 0 -F logs/notification.console.log & NOTIFY_TAIL_PID=$!
tail -n 0 -F logs/game_node.console.log & GAME_TAIL_PID=$!
tail -n 0 -F logs/udp_game_node.console.log & UDP_GAME_TAIL_PID=$!
wait
