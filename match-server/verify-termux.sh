#!/data/data/com.termux/files/usr/bin/bash
set -eu
cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: Go 1.24+ is required. Install Go in Termux first." >&2
  exit 1
fi
version="$(go env GOVERSION)"
case "$version" in
  go1.24*|go1.25*|go1.26*|go1.27*|go1.28*) ;;
  *) echo "ERROR: found $version; this module requires Go 1.24+." >&2; exit 1 ;;
esac

echo "[1/4] gofmt check"
files="$(gofmt -l $(find . -type f -name '*.go' -not -path './vendor/*'))"
if [ -n "$files" ]; then
  printf '%s\n' "$files"
  echo "ERROR: run gofmt on the files above." >&2
  exit 1
fi

echo "[2/4] downloading dependencies"
go mod download

echo "[3/4] running all tests"
go test ./...

echo "[4/4] vet + build"
go vet ./...
go build -o match-server .
chmod 700 match-server
echo "PASS: tests, vet, and build completed. This does not replace in-game integration testing."
