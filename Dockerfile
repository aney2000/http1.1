# syntax=docker/dockerfile:1

# ---- base: module download, cached independently of source changes ----
FROM golang:1.27 AS base
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .

# ---- test: full suite with the race detector (needs cgo, hence Debian) ----
FROM base AS test
RUN CGO_ENABLED=1 go test -race -count=1 ./...

# ---- build: static, stripped binary ----
FROM base AS build
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime: distroless static image, no shell, runs as non-root ----
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
