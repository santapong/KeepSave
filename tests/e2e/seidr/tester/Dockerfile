FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /out/tester .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/tester /usr/local/bin/tester
ENTRYPOINT ["/usr/local/bin/tester"]
