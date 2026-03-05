import type { GameOverMsg } from "../types/messages";
import type { GameRecord } from "../types/history";

/**
 * Maps a GameRecord from the history API to the shape expected by GameOverScreen (GameOverMsg).
 * Used when the client missed the real-time game_over (e.g. device was locked) and we recover from history.
 */
export function recordToGameOverMsg(record: GameRecord): GameOverMsg {
  const yourIndex = record.your_index ?? 0;
  const winnerIndex = record.winner_index;

  const result: GameOverMsg["result"] =
    winnerIndex === null ? "draw" : winnerIndex === yourIndex ? "win" : "lose";

  const you =
    yourIndex === 0
      ? { name: record.player0_name, score: record.player0_score }
      : { name: record.player1_name, score: record.player1_score };
  const opponent =
    yourIndex === 0
      ? { name: record.player1_name, score: record.player1_score }
      : { name: record.player0_name, score: record.player0_score };

  const youEloBefore =
    yourIndex === 0 ? record.player0_elo_before : record.player1_elo_before;
  const youEloAfter =
    yourIndex === 0 ? record.player0_elo_after : record.player1_elo_after;

  const msg: GameOverMsg = {
    type: "game_over",
    result,
    you,
    opponent,
  };
  if (youEloBefore != null) msg.you_elo_before = youEloBefore;
  if (youEloAfter != null) msg.you_elo_after = youEloAfter;
  return msg;
}
