#
# syntax=docker/dockerfile:1.7
#
# Build stage compiles the Go microservice statically.
FROM golang:1.25-alpine AS build

WORKDIR /app

# Install build deps (git for go modules, CA certs for HTTPS).
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -trimpath -ldflags="-s -w" -o /bin/redbridge ./cmd/api

# Runtime stage uses distroless base for minimal footprint.
FROM gcr.io/distroless/base-debian12

ENV LISTEN_ADDR=:8080 \
    TZ=Europe/London

WORKDIR /srv

COPY --from=build /bin/redbridge /usr/local/bin/redbridge

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/redbridge"]
