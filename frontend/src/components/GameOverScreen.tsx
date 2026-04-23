import React from "react";
import { Button } from "@mui/material";
import { useNavigate } from "react-router-dom";
import { useGameContext } from "../context/GameContext";
import "./GameOverScreen.css";

export function GameOverScreen() {
  const game = useGameContext();
  const navigate = useNavigate();

  if (!game.gameOver) return null;

  return (
    <div className="game-over-screen">
      <span
        className="game-over-title"
        style={{ color: game.playerWon ? "#4caf50" : "#f44336" }}
      >
        {game.playerWon ? "YOU WON" : "YOU LOST"}
      </span>
      <span className="game-over-score">
        {game.declarer?.name}: {game.playerWon === game.isDeclarer ? "+" : ""}
        {game.gameValue}
      </span>
      {game.isSchneider && !game.isNull && (
        <span className="game-over-bonus">
          {game.isSchwarz ? "SCHWARZ!" : "SCHNEIDER!"}
        </span>
      )}
      <div className="game-over-buttons">
        {game.canPlayNext && (
          <Button
            variant="contained"
            color="primary"
            size="large"
            fullWidth
            onClick={() => game.controls.playNextGame()}
            disabled={!game.controls.isConnected || game.controls.isLoading}
          >
            {game.controls.isLoading
              ? "Loading..."
              : `Play Next (${game.gamesPlayed + 1}/10)`}
          </Button>
        )}
        <Button
          variant={game.canPlayNext ? "outlined" : "contained"}
          color="primary"
          size="large"
          fullWidth
          onClick={() => navigate("/")}
        >
          Back to Lobby
        </Button>
      </div>
    </div>
  );
}
