FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} CGO_ENABLED=1 \
    go build -trimpath -ldflags="-s -w -buildid=" -o /out/reminderin ./cmd/api

RUN mkdir -p /out/data

FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app

ENV PORT=8080 \
    DB_PATH=/app/data/reminderin.db \
    WA_LOAD_ALL_CLIENTS=false \
    HTTP_ACCESS_LOG=false

COPY --from=build --chown=nonroot:nonroot /out/reminderin /app/reminderin
COPY --from=build --chown=nonroot:nonroot /out/data /app/data

VOLUME ["/app/data"]
EXPOSE 8080

ENTRYPOINT ["/app/reminderin"]
