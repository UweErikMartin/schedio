# schedio

Standard Go web-service scaffold with:

- embedded static web assets (HTML/CSS/JS) compiled into the binary
- OpenAPI endpoint and Swagger UI
- CalDAV endpoint skeleton
- Docker container build

## Project structure

```text
cmd/schedio/          # application entrypoint
internal/server/      # router/server composition
internal/handlers/    # OpenAPI, Swagger, CalDAV handlers
api/                  # OpenAPI source document (embedded)
web/static/           # frontend assets (embedded)
```

## Run locally

```bash
go run ./cmd/schedio
```

Server runs on `:8080` by default. Override with:

```bash
HTTP_ADDR=:9090 go run ./cmd/schedio
```

## Endpoints

- `/` web page served from embedded assets
- `/healthz` health check
- `/openapi.yaml` OpenAPI specification
- `/swagger/` Swagger UI
- `/caldav` CalDAV endpoint skeleton

## Build

```bash
go build -o bin/schedio ./cmd/schedio
```

## Container

```bash
docker build -t schedio:local .
docker run --rm -p 8080:8080 schedio:local
```
