FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
RUN mkdir -p /out \
  && go build -o /out/jangolova ./cmd/jangolova \
  && go build -o /out/xpost ./cmd/xpost \
  && go build -o /out/xpost-playwright ./cmd/xpost-playwright \
  && go build -o /out/playwright-install ./cmd/playwright-install

FROM node:22-bookworm

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    curl \
    fonts-liberation \
    x11vnc \
    xvfb \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev

COPY scripts ./scripts
COPY tests ./tests
COPY README.md ./
COPY examples ./examples
COPY --from=builder /out/jangolova /app/bin/jangolova
COPY --from=builder /out/xpost /app/bin/xpost
COPY --from=builder /out/xpost-playwright /app/bin/xpost-playwright
COPY --from=builder /out/playwright-install /app/bin/playwright-install

RUN chmod +x scripts/*.sh

ENV PLAYWRIGHT_DRIVER_PATH=/opt/ms-playwright-go \
  PLAYWRIGHT_NODEJS_PATH=/usr/local/bin/node

RUN bin/playwright-install

ENV DISPLAY_NUM=99 \
  GEOMETRY=1920x1080x24 \
  CDP_HOST=0.0.0.0 \
  CDP_PORT=9222 \
  VNC_LOCALHOST=0 \
  VNC_PORT=5999 \
  PROFILE_DIR=/data/chromium-profile \
  PLAYWRIGHT_BROWSER_PATH=/usr/bin/chromium \
  CHROMIUM_CLEAR_STALE_LOCKS=1 \
  RUN_DIR=/tmp/xpost-run \
  LOG_DIR=/tmp/xpost-logs

EXPOSE 9222 5999

ENTRYPOINT ["scripts/docker-entrypoint.sh"]
