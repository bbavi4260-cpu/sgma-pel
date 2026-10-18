#!/data/data/com.termux/files/usr/bin/sh
set -u
cd "$(dirname "$0")"
for PID_FILE in logs/server.pid logs/notification.pid logs/game_node.pid logs/udp_game_node.pid; do
  if [ -s "$PID_FILE" ]; then
    kill "$(cat "$PID_FILE")" 2>/dev/null || true
  fi
done
rm -f logs/server.pid logs/notification.pid logs/game_node.pid logs/udp_game_node.pid
echo "Sigma HTTP, TCP, UDP, and notification listeners stopped."
