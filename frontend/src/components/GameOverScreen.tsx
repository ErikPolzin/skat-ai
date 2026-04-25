import React from "react";
import {
  Button,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import { useNavigate } from "react-router-dom";
import { useGameContext } from "../context/GameContext";
import "./GameOverScreen.css";

export function GameOverScreen() {
  const game = useGameContext();
  const navigate = useNavigate();

  if (!game.gameOver || !game.result) return null;

  const { result } = game;
  const absMatadors = Math.abs(result.matadors);

  // Map suit symbols to full names
  const suitNames: { [key: string]: string } = {
    "♣": "Clubs",
    "♠": "Spades",
    "♥": "Hearts",
    "♦": "Diamonds",
  };

  return (
    <div className="game-over-screen">
      <span
        className="game-over-title"
        style={{ color: game.playerWon ? "#4caf50" : "#f44336" }}
      >
        {game.playerWon ? "YOU WON" : "YOU LOST"}
      </span>
      {result.is_forfeit ? (
        <span className="game-over-score" style={{ fontSize: "18px", marginTop: "12px" }}>
          Game forfeited due to inactivity
        </span>
      ) : (
        <span className="game-over-score">
          {game.declarer?.name}: {game.playerWon === game.isDeclarer ? "+" : ""}
          {result.value}
        </span>
      )}
      {!result.is_forfeit && !game.isNull && result.base_value > 0 && (
        <TableContainer component={Paper}>
          <Table size="small">
            <TableBody>
              <TableRow>
                <TableCell>
                  Game, {result.matadors > 0 ? "With" : "Without"} {absMatadors}
                </TableCell>
                <TableCell align="right">
                  {1 + absMatadors} (+{1 + absMatadors})
                </TableCell>
              </TableRow>
              {result.is_schneider && (
                <TableRow>
                  <TableCell>
                    {result.is_schwarz ? "Schwarz Made" : "Schneider Made"}
                  </TableCell>
                  <TableCell align="right">
                    {result.is_schwarz ? 2 : 1} (
                    {result.declarer_won ? "+" : "-"}
                    {result.is_schwarz ? 2 : 1})
                  </TableCell>
                </TableRow>
              )}
              <TableRow>
                <TableCell>
                  {game.gameMode === "grand"
                    ? "Grand"
                    : `${suitNames[game.trumpSuit]} contract`}
                  {result.declarer_won ? ", Won" : ", Lost"}
                </TableCell>
                <TableCell align="right">
                  {!result.declarer_won && `-2×(`}
                  {result.multiplier}×{result.base_value}
                  {!result.declarer_won && `)`}
                </TableCell>
              </TableRow>
              <TableRow className="breakdown-total">
                <TableCell sx={{ fontWeight: "bold" }}>Total</TableCell>
                <TableCell align="right" sx={{ fontWeight: "bold" }}>
                  {result.value}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </TableContainer>
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
