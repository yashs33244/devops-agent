# Multi-stage build for the Nightshift reference API server.

FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/nightshift-api ./cmd/nightshift-api

# Distroless static — no shell, no libc, reduced attack surface.
# Run as non-root by default.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/nightshift-api /usr/local/bin/nightshift-api
USER nonroot:nonroot
EXPOSE 50051 8080
ENTRYPOINT ["/usr/local/bin/nightshift-api"]
