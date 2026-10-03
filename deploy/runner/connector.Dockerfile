# Build only in a reviewed operator build workflow. The supervisor cannot build
# or pull images. Enroll the resulting immutable registry digest before use.
FROM golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal/runner ./internal/runner
COPY cmd/keepsave-connector ./cmd/keepsave-connector
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /keepsave-connector ./cmd/keepsave-connector
FROM scratch
COPY --from=builder /keepsave-connector /keepsave-connector
USER 65532:65532
ENTRYPOINT ["/keepsave-connector"]
