FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=1.0.0
ARG FG_LICENSE_PUBLIC_KEY
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X github.com/mah3sec/forgeguardian/internal/license.publicKeyHex=${FG_LICENSE_PUBLIC_KEY}" \
    -o /forgeguardian-api \
    ./internal/api/
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X github.com/mah3sec/forgeguardian/internal/license.publicKeyHex=${FG_LICENSE_PUBLIC_KEY}" \
    -o /fgctl \
    ./cmd/fgctl/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /forgeguardian-api /forgeguardian-api
COPY --from=builder /fgctl /usr/local/bin/fgctl
EXPOSE 8080
ENTRYPOINT ["/forgeguardian-api"]
