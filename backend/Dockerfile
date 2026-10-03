# Build stage. The exact toolchain matches CI; go.mod retains its minimum version.
# Digest-pinned for supply-chain integrity (INF-2); the trailing `# tag` comment
# is the human-readable version Dependabot/Renovate bump against.
FROM golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190 AS builder
# tag: golang:1.27.1

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static, CGO-free binary so it runs on the distroless `static` base below.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /keepsave-operator ./cmd/keepsave-operator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /keepsave-vault ./cmd/keepsave-vault
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /keepsave-worker ./cmd/keepsave-worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /keepsave-harness ./cmd/keepsave-harness
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /keepsave ./cmd/keepsave

# Runtime stage: distroless static (Debian 12) `nonroot` variant (INF-1/INF-2).
# - No shell, no package manager, no libc beyond what a static Go binary needs.
# - Ships CA certificates and /etc/passwd with a `nonroot` (uid/gid 65532) user.
# - Already runs as non-root by default; we make it explicit below.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# tag: gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy as the distroless nonroot uid/gid (65532) so the files are owned by the
# runtime user without a `chown` step (distroless has no shell to run one).
COPY --from=builder --chown=65532:65532 /server /app/server
COPY --from=builder --chown=65532:65532 /keepsave-operator /app/keepsave-operator
COPY --from=builder --chown=65532:65532 /keepsave-vault /app/keepsave-vault
COPY --from=builder --chown=65532:65532 /keepsave-worker /app/keepsave-worker
COPY --from=builder --chown=65532:65532 /keepsave-harness /app/keepsave-harness
COPY --from=builder --chown=65532:65532 /keepsave /app/keepsave
COPY --from=builder --chown=65532:65532 /app/migrations /app/migrations

# Run as the distroless `nonroot` user (uid/gid 65532) per audit S-M8.
USER 65532:65532

EXPOSE 8080

# Exec form + absolute path (no shell in distroless).
ENTRYPOINT ["/app/server"]
