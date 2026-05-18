FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:3.23

RUN apk --no-cache add ca-certificates \
 && addgroup -S keepsave \
 && adduser -S -G keepsave -u 10001 keepsave
WORKDIR /app

COPY --from=builder /server .
COPY --from=builder /app/migrations ./migrations
RUN chown -R keepsave:keepsave /app

# Run as a non-root user per audit S-M8. The fixed uid/gid (10001) lets
# operators map host volumes without surprise ownership.
USER keepsave:keepsave

EXPOSE 8080

CMD ["./server"]
