FROM golang:1.26-alpine@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/proofline-server ./cmd/api

FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S proofline \
	&& adduser -S -G proofline -h /nonexistent -s /sbin/nologin proofline \
	&& mkdir -p /var/lib/proofline /etc/proofline \
	&& chown -R proofline:proofline /var/lib/proofline

COPY --from=builder /out/proofline-server /usr/local/bin/proofline-server
COPY docker-default-config.toml /etc/proofline/proofline.toml

USER proofline
WORKDIR /var/lib/proofline
VOLUME ["/var/lib/proofline"]
EXPOSE 8080 8081

ENTRYPOINT ["proofline-server", "--config", "/etc/proofline/proofline.toml"]
