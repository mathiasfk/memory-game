# Memory Game — Server

Go WebSocket server: matchmaking, game logic, and power-ups.

## Run

```bash
go run .
```

Listens on `:8080` by default. Optional: create a `config.json` in this directory or set env vars (e.g. `WS_PORT`, `LOG_LEVEL`) to override defaults. `LOG_LEVEL` controls log verbosity: `debug`, `info` (default), `warn`, or `error`. Use `LOG_LEVEL=debug` to see AI decision logs.

## Test

```bash
go test ./...
```

Runs unit and integration tests.
