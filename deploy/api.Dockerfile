# TableFlow API image (MVP-21). Build from the repository root:
#   docker build -f deploy/api.Dockerfile -t tableflow-api .
# Static binaries on a distroless, non-root base; includes the migrate and
# tableflowctl commands and the SQL migrations. No docs/specs/AI files are
# copied (the root .dockerignore also excludes them).
FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY src/api/go.mod src/api/go.sum ./
RUN go mod download
COPY src/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tableflow-api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tableflowctl ./cmd/tableflowctl

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /src/db/migrations/ /app/db/migrations/
USER nonroot
EXPOSE 8080
ENV HTTP_ADDR=0.0.0.0:8080
ENTRYPOINT ["/usr/local/bin/tableflow-api"]
