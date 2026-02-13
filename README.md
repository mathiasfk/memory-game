# Memory Game

Multiplayer memory card game: React client + Go WebSocket server.

## Run the whole game

1. **Start the server** (terminal 1):

   ```bash
   cd server && go run .
   ```

2. **Start the client** (terminal 2):

   ```bash
   cd client && pnpm install && pnpm dev
   ```

3. Open **http://localhost:5173** in your browser. Use two tabs/windows to play with two players (or share the link).

Server defaults: `:8080`. Client defaults: `ws://localhost:8080/ws`. See `client/README.md` and `server/README.md` for run/test details.
