# Build the manager binary
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

# Optional: custom CA certificates (corporate VPN/proxy)
COPY certs/ /tmp/certs/
RUN for f in /tmp/certs/*.pem /tmp/certs/*.crt; do \
      [ -f "$f" ] && cp "$f" /usr/local/share/ca-certificates/"$(basename "${f%.*}").crt" || true; \
    done && update-ca-certificates

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager ./cmd/...

FROM alpine:3.21
RUN apk add --no-cache ca-certificates && addgroup -S operator && adduser -S operator -G operator
WORKDIR /
COPY --from=builder /workspace/manager .
USER operator:operator

ENTRYPOINT ["/manager"]
