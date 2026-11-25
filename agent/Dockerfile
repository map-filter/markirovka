FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=${VERSION}" -o /out/markirovka-agent .

FROM alpine:3
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 agent
COPY --from=build /out/markirovka-agent /usr/local/bin/markirovka-agent
USER agent
ENTRYPOINT ["/usr/local/bin/markirovka-agent"]
CMD ["run"]
