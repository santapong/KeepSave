# Build stage. golang:1.25-alpine matches go.mod (go 1.25.0) and CI's toolchain.
# Digest-pinned for supply-chain integrity (INF-2); the trailing `# tag` comment
# is the human-readable version Dependabot/Renovate bump against.
FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder
# tag: golang:1.25-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static, CGO-free binary so it runs on the distroless `static` base below.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# Runtime stage: distroless static (Debian 12) `nonroot` variant (INF-1/INF-2).
# - No shell, no package manager, no libc beyond what a static Go binary needs.
# - Ships CA certificates and /etc/passwd with a `nonroot` (uid/gid 65532) user.
# - Already runs as non-root by default; we make it explicit below.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
# tag: gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy as the distroless nonroot uid/gid (65532) so the files are owned by the
# runtime user without a `chown` step (distroless has no shell to run one).
COPY --from=builder --chown=65532:65532 /server /app/server
COPY --from=builder --chown=65532:65532 /app/migrations /app/migrations

# Run as the distroless `nonroot` user (uid/gid 65532) per audit S-M8.
USER 65532:65532

EXPOSE 8080

# Exec form + absolute path (no shell in distroless).
ENTRYPOINT ["/app/server"]
