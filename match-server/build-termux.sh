#!/data/data/com.termux/files/usr/bin/bash
set -eu

cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  echo "Go não encontrado. Instale o Go 1.24 ou superior no Termux e repita." >&2
  exit 1
fi

echo "Compilando servidor de partida..."
go version
go mod download
go build -o match-server .
chmod 700 match-server
echo "Servidor compilado em: $(pwd)/match-server"
