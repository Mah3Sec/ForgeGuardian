FROM golang:1.23-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /forgeguardian-api \
    ./internal/api/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /forgeguardian-api /forgeguardian-api
EXPOSE 8080
ENTRYPOINT ["/forgeguardian-api"]
