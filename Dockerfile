FROM golang:1.26-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/proofline-server ./cmd/api

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S safety \
	&& adduser -S -G safety -h /nonexistent -s /sbin/nologin safety \
	&& mkdir -p /data \
	&& chown -R safety:safety /data

ENV SAFE_MAIN_BIND_ADDRS=0.0.0.0:8080 \
	SAFE_ADMIN_BIND_ADDRS=0.0.0.0:8081 \
	SAFE_DATA_DIR=/data \
	SAFE_DB_PATH=/data/safety.db

COPY --from=builder /out/proofline-server /usr/local/bin/proofline-server

USER safety
WORKDIR /data
VOLUME ["/data"]
EXPOSE 8080 8081

ENTRYPOINT ["proofline-server"]
