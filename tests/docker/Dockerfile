# Test-only Linux engine environment. This is not product topology.
FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY adapters ./adapters
COPY internal ./internal
RUN mkdir -p /out \
  && go build -o /out/jangolova ./cmd/jangolova \
  && go build -o /out/native-bridge-fixture ./cmd/native-bridge-fixture

FROM node:22-bookworm

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    curl \
    fonts-liberation \
    xvfb \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY tests ./tests
COPY examples ./examples
COPY integrations ./integrations
COPY internal/bridge ./internal/bridge
COPY --from=builder /out/jangolova /app/bin/jangolova
COPY --from=builder /out/native-bridge-fixture /app/bin/native-bridge-fixture
RUN chmod +x tests/docker/*.sh examples/native-process/fixture.sh

ENTRYPOINT ["tests/docker/entrypoint.sh"]
