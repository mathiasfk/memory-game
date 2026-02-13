# Memory Game — Client

React + TypeScript + Vite frontend. Connects to the game server via WebSocket.

## Run

```bash
pnpm install
pnpm dev
```

App runs at `http://localhost:5173`. It uses `ws://localhost:8080/ws` by default. Override with a `.env` file:

```
VITE_WS_URL=ws://localhost:8080/ws
```

## Test

```bash
pnpm build
pnpm preview
```

`pnpm build` runs the TypeScript compiler and Vite build; `pnpm preview` serves the production build locally.
