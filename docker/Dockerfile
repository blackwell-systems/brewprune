FROM golang:1.23-bookworm

# ── Real binaries to be shimmed ──────────────────────────────────────────────
RUN apt-get update && apt-get install -y \
    git curl jq ripgrep htop tree sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# ── Fake brew cellar structure ────────────────────────────────────────────────
# Mirrors /opt/homebrew/Cellar/<pkg>/<version>/bin/<binary> layout so that
# RefreshBinaryPaths (which follows symlinks to extract package names from
# cellar paths) exercises the real code path without needing Homebrew.
RUN mkdir -p \
    /opt/mock-brew/bin \
    /opt/mock-brew/Cellar/git/2.43.0/bin \
    /opt/mock-brew/Cellar/curl/8.6.0/bin \
    /opt/mock-brew/Cellar/jq/1.7.1/bin \
    /opt/mock-brew/Cellar/ripgrep/14.1.0/bin \
    /opt/mock-brew/Cellar/htop/3.3.0/bin \
    /opt/mock-brew/Cellar/tree/2.1.1/bin \
    /opt/mock-brew/Cellar/openssl@3/3.2.0/bin \
    /opt/mock-brew/Cellar/oniguruma/6.9.8/bin \
    /opt/mock-brew/Cellar/ca-certificates/2024.1.1 \
    /opt/mock-brew/Caskroom

# Cellar wrapper scripts (call real system binaries)
RUN printf '#!/bin/sh\nexec /usr/bin/git "$@"\n'   > /opt/mock-brew/Cellar/git/2.43.0/bin/git      && \
    printf '#!/bin/sh\nexec /usr/bin/curl "$@"\n'  > /opt/mock-brew/Cellar/curl/8.6.0/bin/curl     && \
    printf '#!/bin/sh\nexec /usr/bin/jq "$@"\n'    > /opt/mock-brew/Cellar/jq/1.7.1/bin/jq         && \
    printf '#!/bin/sh\nexec /usr/bin/rg "$@"\n'    > /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/rg   && \
    printf '#!/bin/sh\nexec /usr/bin/rg "$@"\n'    > /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/ripgrep && \
    printf '#!/bin/sh\nexec /usr/bin/htop "$@"\n'  > /opt/mock-brew/Cellar/htop/3.3.0/bin/htop     && \
    printf '#!/bin/sh\nexec /usr/bin/tree "$@"\n'  > /opt/mock-brew/Cellar/tree/2.1.1/bin/tree      && \
    chmod +x \
        /opt/mock-brew/Cellar/git/2.43.0/bin/git \
        /opt/mock-brew/Cellar/curl/8.6.0/bin/curl \
        /opt/mock-brew/Cellar/jq/1.7.1/bin/jq \
        /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/rg \
        /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/ripgrep \
        /opt/mock-brew/Cellar/htop/3.3.0/bin/htop \
        /opt/mock-brew/Cellar/tree/2.1.1/bin/tree

# Prefix/bin symlinks (as brew keg-linking normally creates)
RUN ln -s /opt/mock-brew/Cellar/git/2.43.0/bin/git          /opt/mock-brew/bin/git      && \
    ln -s /opt/mock-brew/Cellar/curl/8.6.0/bin/curl         /opt/mock-brew/bin/curl     && \
    ln -s /opt/mock-brew/Cellar/jq/1.7.1/bin/jq             /opt/mock-brew/bin/jq       && \
    ln -s /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/rg       /opt/mock-brew/bin/rg       && \
    ln -s /opt/mock-brew/Cellar/ripgrep/14.1.0/bin/ripgrep  /opt/mock-brew/bin/ripgrep  && \
    ln -s /opt/mock-brew/Cellar/htop/3.3.0/bin/htop         /opt/mock-brew/bin/htop     && \
    ln -s /opt/mock-brew/Cellar/tree/2.1.1/bin/tree         /opt/mock-brew/bin/tree

# Symlink /opt/homebrew → /opt/mock-brew so brewprune-shim's findRealBinary()
# (which hardcodes /opt/homebrew and /usr/local) finds binaries at exec time.
RUN ln -s /opt/mock-brew /opt/homebrew

# ── Mock brew CLI ─────────────────────────────────────────────────────────────
COPY docker/brew          /usr/local/bin/brew
COPY docker/packages.json /opt/mock-brew/packages.json
COPY docker/deps-all.txt  /opt/mock-brew/deps-all.txt
RUN chmod +x /usr/local/bin/brew

# ── Build brewprune from source ───────────────────────────────────────────────
WORKDIR /brewprune
COPY . .
RUN go build \
    -ldflags="-X main.shimVersion=container-test" \
    -o /usr/local/bin/brewprune-shim \
    ./cmd/brewprune-shim

RUN go build \
    -o /usr/local/bin/brewprune \
    ./cmd/brewprune

# ── Test entrypoint ───────────────────────────────────────────────────────────
COPY docker/test.sh /test.sh
RUN chmod +x /test.sh

# ~/.brewprune/bin is pre-created; brewprune scan will populate it
RUN mkdir -p /root/.brewprune/bin

# Shim dir first on PATH, then mock-brew bin (so exec.LookPath finds brew
# versions during scan), then system dirs.
ENV PATH="/root/.brewprune/bin:/opt/mock-brew/bin:/usr/local/bin:$PATH"
ENV HOME=/root

ENTRYPOINT ["/test.sh"]
