# Multi-stage build for the Nightshift reference worker image.
# The reference worker is intentionally minimal: it emits simulated
# protocol events (no LLM) to prove the agent-to-platform contract.

FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/nightshift-worker ./cmd/nightshift-worker

# Distroless static — no shell, no libc, reduced attack surface.
# Run as non-root by default.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/nightshift-worker /usr/local/bin/nightshift-worker
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/nightshift-worker"]
