import React from "react";
import { useGameContext } from "../context/GameContext";
import "./GameLobbyWaiting.css";

export function GameLobbyWaiting() {
  const game = useGameContext();
  const playersNeeded = 3 - game.playerCount;

  return (
    <div className="lobby-waiting">
      <div className="lobby-content">
        <h3>Waiting for Players</h3>

        {game.gameCode && (
          <div className="game-code-section">
            <div className="game-code-label">Game Code</div>
            <div className="game-code">{game.gameCode}</div>
            <div className="game-code-hint">
              Share this code with friends to join
            </div>
          </div>
        )}

        <div className="player-count">
          {game.playerCount} / 3 players joined
        </div>
        {playersNeeded > 0 && (
          <>
            <p className="waiting-text">
              Waiting for {playersNeeded} more player
              {playersNeeded > 1 ? "s" : ""}...
            </p>
            <button
              className="add-ai-button"
              onClick={() => game.addAgent("mcts")}
            >
              Add AI Player
            </button>
          </>
        )}
      </div>
    </div>
  );
}
