import React from "react";
import { useNavigate } from "react-router-dom";
import { GameProvider, useGameContext } from "../context/GameContext";
import { MotionCardTable } from "../components/MotionCardTable";
import "./GameScreen.css";

function GameScreenContent() {
  const game = useGameContext();
  const navigate = useNavigate();

  const handlePlayAgain = () => {
    navigate("/");
  };

  const handleRetryLoad = () => {
    window.location.reload();
  };

  // Show loading state
  if (game.isLoading) {
    return (
      <div className="game-screen">
        <div className="loading-container">
          <div className="loading-spinner"></div>
          <p className="loading-text">Loading game...</p>
        </div>
      </div>
    );
  }

  // Show error state
  if (game.error) {
    return (
      <div className="game-screen">
        <div className="error-container">
          <div className="error-icon">⚠️</div>
          <h2 className="error-title">Unable to Load Game</h2>
          <p className="error-message">{game.error}</p>
          <div className="error-actions">
            <button className="retry-btn" onClick={handleRetryLoad}>
              Try Again
            </button>
            <button className="back-btn" onClick={() => navigate("/")}>
              Back to Lobby
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Main game screen - always use MotionCardTable for consistency
  return (
    <div className="game-screen">
      <MotionCardTable />

      {/* Game Over Modal Overlay */}
      {game.gameOver && (
        <div className="game-over-overlay">
          <div className="game-over-modal">
            <div className="game-over-header">
              <div
                className={`result-trophy ${game.playerWon ? "winner" : "loser"}`}
              >
                {game.playerWon ? "🏆" : "💔"}
              </div>
              <h1>{game.playerWon ? "YOU WON!" : "YOU LOST"}</h1>
            </div>

            <div className="game-over-scores">
              <div className="score-section">
                <div className="score-label">
                  DECLARER: {game.declarerScore}
                </div>
                <div className="score-label">
                  OPPONENTS: {game.opponentScore}
                </div>
              </div>

              {game.isSchneider && !game.isNull && (
                <div className="special-result schneider">
                  {game.isSchwarz ? "SCHWARZ!" : "SCHNEIDER!"}
                </div>
              )}

              {game.isNull && (
                <div className="null-result">
                  {game.declarerTricks === 0
                    ? "Perfect null game - no tricks taken!"
                    : `Null game failed - ${game.declarerTricks} trick${game.declarerTricks > 1 ? "s" : ""} taken`}
                </div>
              )}
            </div>

            <div className="game-mode-info">
              <span className="mode-label">
                GAME MODE: {game.gameMode || "1"}
              </span>
              {game.trumpSuit && (
                <span className="trump-suit">({game.trumpSuit})</span>
              )}
            </div>

            <div className="game-over-players">
              <div className="player-role-section">
                <div className="role-header">Declarer</div>
                <div className="player-entry declarer-entry">
                  {game.declarer ? (
                    <>
                      {game.declarer.name}
                      {game.declarer.is_agent && " (AI)"}
                      {game.declarer.player_id === game.playerId && " (You)"}
                    </>
                  ) : (
                    "Unknown"
                  )}
                </div>
              </div>

              <div className="player-role-section">
                <div className="role-header">Opponents</div>
                {game.opponents.map((player, idx) => (
                  <div
                    key={player.player_id}
                    className="player-entry opponent-entry"
                  >
                    {player.name}
                    {player.is_agent && " (AI)"}
                    {player.player_id === game.playerId && " (You)"}
                  </div>
                ))}
              </div>
            </div>

            <button className="play-again-btn" onClick={handlePlayAgain}>
              Play Again
            </button>
          </div>
        </div>
      )}

      {/* Message display for game actions - only show messages without player position */}
      <div className="messages-container">
        {game.messages
          .filter((message) => message.playerPosition === undefined)
          .map((message) => (
            <div
              key={message.id}
              className={`message ${message.isError ? "error" : ""}`}
            >
              {message.text}
            </div>
          ))}
      </div>
    </div>
  );
}

export default function GameScreen() {
  return (
    <GameProvider>
      <GameScreenContent />
    </GameProvider>
  );
}
