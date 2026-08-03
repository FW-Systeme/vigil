# Stage 1: build the vigil binary
FROM golang:1.22 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/vigil ./cmd/vigil/

# Stage 2: runtime (systemd, nginx, node, cron)
FROM debian:bookworm-slim
ENV container=docker

RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd systemd-sysv \
    dbus \
    nginx \
    cron \
    curl procps ca-certificates \
    nodejs \
    sudo \
    && rm -rf /var/lib/apt/lists/*

RUN echo 'root ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/vigil

COPY --from=builder /out/vigil /usr/local/bin/vigil
COPY e2e/fixtures/ /e2e/fixtures/
COPY e2e/run.sh /e2e/run.sh
COPY e2e/scripts/ /e2e/scripts/
COPY e2e/vigil-e2e.service /etc/systemd/system/vigil-e2e.service

RUN chmod +x /e2e/run.sh /e2e/scripts/*.sh \
    && systemctl enable vigil-e2e.service

WORKDIR /e2e
ENTRYPOINT ["/e2e/run.sh"]
