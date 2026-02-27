# Memory Game -- Global Specification

This document is the single source of truth for the competitive online memory card game. Both the server and the client implementations must conform to everything described here.

---

## 1. Game Overview

Two players are matched online and compete on a shared board of face-down cards.

### 1.1 User interface language

All user-facing text in the client (buttons, labels, messages, and accessibility attributes such as `aria-label`) must be in English. Players take turns flipping two cards per turn. If the two cards match, the player scores points and the cards remain face-up. If they do not match, both cards are flipped back face-down and the turn passes to the opponent. The game ends when all pairs have been found. The player with the highest score wins.

---

## 2. Session Identity

- There are no persistent user accounts, logins, or profiles.
- When a player connects, they choose a **display name** for the current session.
- The display name is used only for in-game identification and is discarded when the session ends.
- Display names must be between 1 and 24 characters.

---

## 3. Matchmaking

- After choosing a name, a player enters a **matchmaking queue**.
- The server pairs two queued players at random to start a new game.
- Multiple independent games may run simultaneously; each game is fully isolated.
- If no opponent is available, the player waits until one connects.
- If a player disconnects while waiting, they are silently removed from the queue.

---

## 4. Game Rules

### 4.1 Board Setup

- The board is a grid of cards arranged in `BOARD_ROWS x BOARD_COLS` cells.
- Each card belongs to exactly one pair (there are `(BOARD_ROWS * BOARD_COLS) / 2` distinct pairs).
- Card positions are randomized by the server at the start of the game.
- Each card has:
  - A unique positional **index** (0-based).
  - A **pairId** that identifies which pair it belongs to.
  - A **state**: `hidden`, `revealed`, or `matched`.

### 4.2 Turns

1. The server randomly selects which player goes first.
2. On their turn, the active player flips two cards by sending their indices to the server one at a time.
3. After the first card is flipped, the server broadcasts the updated state (that card is now `revealed`) to both players.
4. After the second card is flipped:
   - If the two revealed cards share the same `pairId`, they become `matched` and remain face-up. The active player scores points (see Section 5). **The active player keeps the turn.**
   - If the two revealed cards do not match, both are set back to `hidden` after a brief reveal window (`REVEAL_DURATION_MS`). The turn passes to the opponent.
5. A player may only flip cards that are currently `hidden`.

### 4.3 Game End

- The game ends when every card on the board is `matched`.
- The server sends a final `GameOver` message with the result (`win`, `lose`, or `draw`) and final scores.
- After the game, each player may choose to re-enter the matchmaking queue.

### 4.4 Disconnections

- If a player disconnects during a game, the opponent is notified and wins by default.
- The abandoned game is cleaned up on the server.

---

## 5. Scoring

Each matched pair awards **1 point** to the active player, regardless of how many pairs they match in the same turn. The game ends when all pairs have been matched; the player with the higher score wins (draw if tied).

---

## 6. Power-Up System

### 6.1 Overview

Power-ups are special actions a player can use from their **hand**. They are **earned by matching pairs**: when a player matches a pair whose board **pairId** is associated with a power-up, one copy of that power-up is added to their hand. Using a power-up costs **no points**; it consumes one copy from the hand. The system is **extensible**: new power-ups can be added without modifying existing game logic.

### 6.2 Pair-to-power-up mapping

Each power-up is associated with **exactly one** board pairId (1:1). This allows consistent art or symbols on the board and in the hand. **Current rule**: the first power-ups in registry order are assigned to the first pair IDs in order. For example, if the registry lists chaos, clairvoyance, necromancy, unveiling in that order, then pairId 0 → chaos, pairId 1 → clairvoyance, pairId 2 → necromancy, pairId 3 → unveiling. All other pairIds (4, 5, …) grant no power-up.

### 6.3 When and how to use power-ups

A player may use a power-up **only during their own turn** and **before flipping any card** in that turn. Using a power-up does **not** end the turn. The player must have at least one copy of that power-up in their hand; using it consumes one copy. No points are deducted.

### 6.4 Power-up contract (metadata)

Every power-up has an `id`, `name`, and `description` for display. The server does not send these in every game state; the client can use a local registry keyed by `id` for tooltips and labels.

### 6.5 Initial power-ups

| `id`            | Effect |
|-----------------|--------|
| `chaos`         | Reshuffles the positions of all cards that are not yet matched. When used, all "known" tile tracking is cleared (affects Unveiling). |
| `clairvoyance`  | Reveals a 3×3 area around a chosen card for a short duration, then hides it again. Requires a card target. |
| `necromancy`    | Returns all collected (matched) tiles back to the board in new random positions; tiles that were never revealed stay in place. |
| `unveiling`   | Highlights (without revealing) all tiles that have **never** been revealed. Effect lasts only the current turn. When Chaos is used, known state is cleared and the highlight resets. |

### 6.6 Adding new power-ups

- **Server**: implement the `PowerUp` interface and register it in the power-up registry (registration order defines pairId mapping).
- **Client**: add a visual entry (icon, label, description) in the client-side power-up registry keyed by the power-up `id`.

---

## 7. Communication Protocol

### 7.1 Transport

- **WebSocket** over a single persistent connection per player.
- All messages are **JSON-encoded** UTF-8 text frames.
- Every message has a top-level `type` field that determines its schema.

### 7.2 Message Direction Convention

- **Client -> Server**: player actions.
- **Server -> Client**: state updates and notifications.

### 7.3 Client-to-Server Messages

#### `SetName`

Sent once after connecting to declare the player's display name.

```json
{
  "type": "set_name",
  "name": "<string, 1-24 chars>"
}
```

#### `FlipCard`

Sent during the player's turn to flip a card.

```json
{
  "type": "flip_card",
  "index": "<int, 0-based card index>"
}
```

#### `UsePowerUp`

Sent during the player's turn, before any card is flipped, to activate a power-up.

```json
{
  "type": "use_power_up",
  "powerUpId": "<string, e.g. 'chaos'>"
}
```

#### `PlayAgain`

Sent after a game ends to re-enter the matchmaking queue.

```json
{
  "type": "play_again"
}
```

### 7.4 Server-to-Client Messages

#### `Error`

Sent when a client action is invalid.

```json
{
  "type": "error",
  "message": "<string, human-readable error description>"
}
```

#### `WaitingForMatch`

Confirms the player is in the matchmaking queue.

```json
{
  "type": "waiting_for_match"
}
```

#### `MatchFound`

Sent to both players when a match is made.

```json
{
  "type": "match_found",
  "opponentName": "<string>",
  "boardRows": "<int>",
  "boardCols": "<int>",
  "yourTurn": "<bool>"
}
```

#### `GameState`

Broadcast to both players after every state-changing event (card flip, power-up use). This is the **primary update mechanism**.

```json
{
  "type": "game_state",
  "cards": [
    {
      "index": "<int>",
      "pairId": "<int, only present if state != 'hidden'>",
      "state": "<'hidden' | 'revealed' | 'matched'>"
    }
  ],
  "you": {
    "name": "<string>",
    "score": "<int>"
  },
  "opponent": {
    "name": "<string>",
    "score": "<int>"
  },
  "yourTurn": "<bool>",
  "hand": [
    { "powerUpId": "<string>", "count": "<int>" }
  ],
  "flippedIndices": ["<int, indices of currently revealed (not yet resolved) cards>"],
  "phase": "<'first_flip' | 'second_flip' | 'resolve'>"
}
```

**Note on `pairId` visibility**: `pairId` is only sent for cards whose state is `revealed` or `matched`. For `hidden` cards, `pairId` is omitted to prevent client-side cheating.

#### `GameOver`

Sent when all pairs are matched.

```json
{
  "type": "game_over",
  "result": "<'win' | 'lose' | 'draw'>",
  "you": {
    "name": "<string>",
    "score": "<int>"
  },
  "opponent": {
    "name": "<string>",
    "score": "<int>"
  }
}
```

#### `OpponentDisconnected`

Sent if the opponent leaves mid-game.

```json
{
  "type": "opponent_disconnected"
}
```

### 7.5 Message Flow Diagram

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Server

    Note over C,S: Auth required (see Section 11.1)
    C->>S: Auth (JWT token)
    C->>S: SetName
    S-->>C: WaitingForMatch
    Note over S: Another player joins (or AI after timeout)
    S-->>C: MatchFound (includes gameId, rejoinToken)

    rect rgb(240, 240, 240)
    Note over C,S: Game Loop
    C->>S: FlipCard (1st)
    S-->>C: GameState (one card revealed)
    C->>S: FlipCard (2nd)
    S-->>C: GameState (match or mismatch resolved)
    C->>S: UsePowerUp (optional, before flipping; cardIndex for Clairvoyance)
    S-->>C: GameState (power-up applied)
    end

    S-->>C: GameOver
    C->>S: PlayAgain (optional)
    S-->>C: WaitingForMatch
```

**Reconnection flow** (Section 11.6): After disconnect, client sends `rejoin` (with gameId, rejoinToken, name) or `rejoin_my_game` (with JWT) to rejoin the same game.

---

## 8. Game State Model

The server maintains the following canonical state for each game:

```
Game {
  id:             string            // Unique game identifier
  phase:          GamePhase          // waiting | playing | finished
  board: {
    rows:         int
    cols:         int
    cards:        Card[]             // length = rows * cols
  }
  players:        [Player, Player]   // exactly two
  currentTurn:    int                // index into players (0 or 1)
  turnPhase:      TurnPhase          // first_flip | second_flip | resolve
  flippedIndices: int[]              // indices of cards flipped this turn (0, 1, or 2)
}

Card {
  index:          int                // positional index on the board
  pairId:         int                // which pair this card belongs to
  state:          CardState          // hidden | revealed | matched
}

Player {
  name:           string
  score:          int
}
```

**Important**: the server is the sole owner of game state. The client never computes or stores game logic -- it only renders what the server sends.

---

## 9. Configuration Parameters

All of the following values must be configurable (e.g., via config file, environment variables, or constants). Default values are provided.

| Parameter               | Type  | Default | Description                                                  |
|--------------------------|-------|---------|--------------------------------------------------------------|
| `BOARD_ROWS`             | int   | `4`     | Number of rows on the board.                                 |
| `BOARD_COLS`             | int   | `4`     | Number of columns on the board. `ROWS * COLS` must be even.  |
| `REVEAL_DURATION_MS`     | int   | `1000`  | How long mismatched cards stay revealed before hiding (ms).  |
| `POWERUP_SHUFFLE_COST`   | int   | `3`     | Point cost of the Shuffle power-up.                          |
| `MAX_NAME_LENGTH`        | int   | `24`    | Maximum characters for a player display name.                |
| `WS_PORT`                | int   | `8080`  | Port the WebSocket server listens on.                        |
| `MAX_LATENCY_MS`         | int   | `500`   | Acceptable one-way latency budget (for reference/monitoring).|

---

## 10. Extensibility Principles

1. **Power-ups are data-driven**: each power-up is a self-contained unit with an ID, cost, and effect. No switch/case on type -- use a registry/map pattern.
2. **Server is client-agnostic**: the protocol is plain JSON over WebSocket. Any client (web, mobile, CLI) can implement it.
3. **No client-side game logic**: the client is a thin renderer. All validation, state transitions, and rule enforcement happen on the server.
4. **Parametric tuning**: every gameplay-affecting constant is a configurable parameter, not a hard-coded literal.

---

## 11. Architectural Decisions (Implemented)

This section documents architectural decisions that extend the base specification. Both server and client implementations conform to these extensions.

### 11.1 Authentication (JWT via Neon Auth)

- **Decision**: Authentication is required before any game action. The client must send an `auth` message with a JWT token as the first message after connecting.
- **Rationale**: Enables persistent identity for game history, leaderboard, and cross-device reconnection.
- **Implementation**: Server validates JWT via Neon Auth JWKS (`NEON_AUTH_BASE_URL`). The display name is derived from the JWT `name` claim (first word). User ID comes from the `sub` claim.
- **Fallback**: If `NEON_AUTH_BASE_URL` is not set, the server rejects auth with "Server auth not configured."

### 11.2 AI Opponent

- **Decision**: When no human opponent is available within `AI_PAIR_TIMEOUT_SEC` seconds, the player is matched against an AI opponent.
- **Rationale**: Reduces wait time and allows single-player practice.
- **Implementation**: The AI uses only information from `game_state` messages (no access to board internals). Configurable profiles (e.g., Mnemosyne, Calliope, Thalia) with parameters: `delay_min_ms`, `delay_max_ms`, `use_known_pair_chance`, `forget_chance`. AI players have user IDs prefixed with `ai:` for storage/leaderboard.

### 11.3 Game History and Persistence

- **Decision**: Game results are persisted to PostgreSQL when `DATABASE_URL` is set.
- **Rationale**: Enables history view and ELO-based leaderboard.
- **Implementation**: Tables `game_history` (per-game records) and `player_ratings` (user_id, display_name, elo, wins, losses, draws). ELO is updated after each completed game using the standard K=32 formula. If `DATABASE_URL` is empty, no persistence occurs.

### 11.4 ELO Rating System

- **Decision**: Each player has an ELO rating (default 1000). Ratings are updated after each completed game.
- **Rationale**: Provides a competitive ranking for the leaderboard.
- **Implementation**: `computeEloUpdates(r0, r1, winnerIdx)` with K=32. Draws use 0.5/0.5 expected score. Ratings never go below 0.

### 11.5 REST APIs

- **Decision**: HTTP REST endpoints for authenticated data access.
- **Endpoints**:
  - `GET /api/history` — Returns game history for the authenticated user (JWT required).
  - `GET /api/leaderboard` — Returns global leaderboard ordered by ELO. Query params: `limit` (default 20), `offset`. Optional JWT to include `current_user_entry` when the user is not in the top N.

### 11.6 Reconnection and Rejoin

- **Decision**: Players can rejoin an in-progress game after disconnect or page refresh.
- **Rationale**: Improves UX when network drops or user refreshes.
- **Implementation**:
  - Each game issues a `rejoinToken` per player, sent in `match_found`.
  - `rejoin` message: `{ type: "rejoin", gameId, rejoinToken, name }` — rejoins by token.
  - `rejoin_my_game` message: rejoins by user ID (cross-device, no token needed).
  - `ReconnectTimeoutSec`: If the disconnected player does not rejoin within this window, the opponent wins by default.

### 11.7 Turn Limit

- **Decision**: Optional per-turn time limit (`TurnLimitSec`). When enabled, the turn passes to the opponent if the player does not act in time.
- **Rationale**: Prevents stalling and keeps games moving.
- **Implementation**: `TurnCountdownShowSec` controls when the countdown UI is shown to the client.

### 11.8 Additional Power-Ups

Beyond the base Chaos power-up, the following are implemented:

| Power-Up      | ID             | Effect                                                                 | Config                |
|---------------|----------------|-----------------------------------------------------------------------|------------------------|
| Chaos         | `chaos`        | Reshuffles all unmatched cards. Clears known-tile tracking (resets Unveiling). | `cost`                 |
| Clairvoyance  | `clairvoyance` | Reveals a 3x3 region around a chosen card for a short duration, then hides again. | `cost`, `reveal_duration_ms` |
| Necromancy    | `necromancy`   | Returns all collected tiles back to the board in new random positions. | —                      |
| Unveiling   | `unveiling`  | Highlights (without revealing) all tiles that have never been revealed (current turn only). | —                      |

Power-ups that target a card (e.g., Clairvoyance) use `cardIndex` in the `use_power_up` message.

### 11.9 Protocol Extensions

**Client-to-Server (additional):**

| Type           | Description                                                                 |
|----------------|-----------------------------------------------------------------------------|
| `auth`         | First message; sends JWT `token`. Required before any other action.        |
| `rejoin`       | Rejoin by `gameId`, `rejoinToken`, `name`.                                  |
| `rejoin_my_game` | Rejoin by authenticated user ID (no token).                              |

**Server-to-Client (additional):**

- `match_found` includes `gameId` and `rejoinToken` for reconnection support.

### 11.10 Configuration Extensions

| Parameter                    | Type  | Default | Description                                           |
|-----------------------------|-------|---------|-------------------------------------------------------|
| `NEON_AUTH_BASE_URL`        | string| —       | Base URL for Neon Auth (JWKS validation).             |
| `DATABASE_URL`              | string| —       | PostgreSQL connection string. Empty = no persistence. |
| `AI_PAIR_TIMEOUT_SEC`       | int   | `15`    | Seconds to wait for human opponent before AI match.  |
| `TurnLimitSec`              | int   | `60`    | Max seconds per turn; 0 = disabled.                  |
| `TurnCountdownShowSec`      | int   | `30`    | Seconds before turn end to show countdown.           |
| `ReconnectTimeoutSec`       | int   | `120`   | Seconds to wait for disconnected player to rejoin.   |
| `ReconnectTimeoutSec`       | int   | `120`   | Seconds to wait for disconnected player to rejoin.   |
| `POWERUP_CLAIRVOYANCE_REVEAL_MS` | int | `2000`  | How long Clairvoyance reveals the 3x3 area (ms).    |
