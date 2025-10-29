FROM caddy:builder AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    xcaddy build \
        --with github.com/mnixry/caddy-edgeone-ip=. \
        --with github.com/WeidiDeng/caddy-cloudflare-ip \
        --with github.com/fvbommel/caddy-combine-ip-ranges \
        --with github.com/caddy-dns/cloudflare 

FROM caddy:alpine AS runner

COPY --from=builder /src/caddy /usr/bin/caddy