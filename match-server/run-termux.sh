#!/data/data/com.termux/files/usr/bin/bash
set -eu

cd "$(dirname "$0")"

if [ ! -x ./match-server ]; then
  echo "Binário do servidor não encontrado ou sem permissão de execução." >&2
  echo "Execute npm run match-server-build primeiro." >&2
  exit 1
fi

echo "Iniciando servidor de partida UDP em :10100..."
exec ./match-server
