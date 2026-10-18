FROM golang:1.24-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends python3 nodejs && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY . .
RUN mkdir -p logs data && cd match-server && go build -o ../logs/sigma_match_server .
CMD ["bash", "render_start.sh"]
