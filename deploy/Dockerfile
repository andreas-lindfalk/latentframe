# Latent Frame container build. Two runtime images:
#   • services  — thin, for the API/HTTPS-only services (director, render, assembler)
#   • ingest    — heavier, adds ffmpeg + Whisper for stage-1 media processing
#
#   docker build --target services -t latentframe/services deploy/..  -f deploy/Dockerfile .
#   docker build --target ingest   -t latentframe/ingest   -f deploy/Dockerfile .
#   docker run --rm latentframe/services assembler build --spec /data/p.json --out /data/site.html

# ---- build all service binaries (static, no cgo) ----
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ingest    ./services/ingest    && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/director  ./services/director  && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/render     ./services/render    && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/assembler  ./services/assembler

# ---- thin runtime: director, render, assembler (only need TLS roots) ----
# Pick the service at run time, e.g. `docker run <img> render restage ...`.
FROM alpine:3.21 AS services
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/director /out/render /out/assembler /usr/local/bin/
EXPOSE 8080
CMD ["assembler", "serve", "--site", "/data/index.html", "--leads", "/data/leads.jsonl", "--addr", ":8080"]

# ---- ingest runtime: needs ffmpeg (keyframes/audio) + Whisper (transcription) ----
FROM debian:bookworm-slim AS ingest
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates ffmpeg python3 python3-pip && \
    rm -rf /var/lib/apt/lists/*
RUN python3 -m pip install --break-system-packages --no-cache-dir openai-whisper
COPY --from=builder /out/ingest /usr/local/bin/ingest
ENTRYPOINT ["/usr/local/bin/ingest"]
