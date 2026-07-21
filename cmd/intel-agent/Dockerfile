FROM golang:1.25-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /intel-agent \
    ./cmd/intel-agent/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /intel-agent /intel-agent
ENTRYPOINT ["/intel-agent"]
