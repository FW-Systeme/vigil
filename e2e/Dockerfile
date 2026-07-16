FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd systemd-sysv \
    dbus \
    nginx \
    cron \
    curl procps ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://go.dev/dl/go1.22.2.linux-amd64.tar.gz -o /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:$PATH"
ENV GOPATH="/go"
ENV GOFLAGS=-mod=mod

COPY vigil /usr/local/bin/vigil

WORKDIR /src
CMD ["/bin/bash"]
