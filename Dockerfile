# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.22.12

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/google-worker ./cmd/google-worker \
 && go build -trimpath -ldflags="-s -w" -o /out/advice-scheduler ./cmd/advice-scheduler \
 && go build -trimpath -ldflags="-s -w" -o /out/cleanup ./cmd/cleanup \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/api /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]

FROM gcr.io/distroless/static-debian12:nonroot AS google-worker
COPY --from=build /out/google-worker /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]

FROM gcr.io/distroless/static-debian12:nonroot AS advice-scheduler
COPY --from=build /out/advice-scheduler /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]

FROM gcr.io/distroless/static-debian12:nonroot AS cleanup
COPY --from=build /out/cleanup /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=build /out/migrate /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]

# Cloudflare Containers のデフォルトターゲット
FROM api AS cloudflare-api