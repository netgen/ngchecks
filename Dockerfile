# ngchecks CI image: ngchecks, the pinned Betterleaks and git.
# Built and published by .github/workflows/release.yml, see docs/maintenance.md.

FROM --platform=$BUILDPLATFORM golang:1.27-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
        go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ngchecks ./cmd/ngchecks \
    && scripts/fetch-betterleaks.sh "${TARGETOS}_$([ "$TARGETARCH" = amd64 ] && echo x64 || echo "$TARGETARCH")" /out \
    && cp LICENSE /out/LICENSE

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

LABEL org.opencontainers.image.source="https://github.com/netgen/ngchecks" \
      org.opencontainers.image.description="Project checks for git hooks and CI pipelines" \
      org.opencontainers.image.licenses="MIT"

# CI services check out the repository as another user than the container runs as.
RUN apk add --no-cache git \
    && git config --system --add safe.directory '*'

COPY --from=build /out/ngchecks /out/betterleaks /usr/local/bin/
COPY --from=build /out/LICENSE /out/LICENSE.betterleaks /usr/share/licenses/ngchecks/

CMD ["ngchecks", "help"]
