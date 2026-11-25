FROM golang:1.23-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/emulator ./cmd/emulator

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 emu && mkdir -p /data /seed && chown emu /data
COPY --from=build /out/emulator /usr/local/bin/emulator
USER emu
ENV EMU_ADDR=:8080 EMU_SEED=/seed/seed.json EMU_STATE_FILE=/data/state.json
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/emulator"]
