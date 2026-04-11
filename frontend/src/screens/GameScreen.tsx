import React from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  Typography,
  Paper,
  CircularProgress,
  Modal,
  Chip,
  Alert,
} from "@mui/material";
import EmojiEventsIcon from "@mui/icons-material/EmojiEvents";
import HeartBrokenIcon from "@mui/icons-material/HeartBroken";
import WarningIcon from "@mui/icons-material/Warning";
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
      <Box
        className="game-screen"
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "100vh"
        }}>
        <Box sx={{ textAlign: "center" }}>
          <CircularProgress size={60} sx={{ mb: 2 }} />
          <Typography variant="h6">Loading game...</Typography>
        </Box>
      </Box>
    );
  }

  // Show error state
  if (game.error) {
    return (
      <Box
        className="game-screen"
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "100vh"
        }}>
        <Paper elevation={3} sx={{ p: 4, textAlign: "center", maxWidth: 400 }}>
          <WarningIcon color="warning" sx={{ fontSize: 60, mb: 2 }} />
          <Typography variant="h5" gutterBottom>
            Unable to Load Game
          </Typography>
          <Typography color="text.secondary" sx={{ mb: 3 }}>
            {game.error}
          </Typography>
          <Box sx={{ display: "flex", gap: 2, justifyContent: "center" }}>
            <Button variant="contained" onClick={handleRetryLoad}>
              Try Again
            </Button>
            <Button variant="outlined" onClick={() => navigate("/")}>
              Back to Lobby
            </Button>
          </Box>
        </Paper>
      </Box>
    );
  }

  // Main game screen - always use MotionCardTable for consistency
  return (
    <Box className="game-screen">
      <MotionCardTable />

      {/* Game Over Modal */}
      <Modal
        open={game.gameOver}
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Paper
          elevation={6}
          sx={{
            p: 4,
            maxWidth: 500,
            width: "90%",
            textAlign: "center",
            borderRadius: 2,
          }}
        >
          <Box sx={{ mb: 3 }}>
            {game.playerWon ? (
              <EmojiEventsIcon
                sx={{ fontSize: 80, color: "gold", mb: 2 }}
              />
            ) : (
              <HeartBrokenIcon
                sx={{ fontSize: 80, color: "error.main", mb: 2 }}
              />
            )}
            <Typography variant="h3" sx={{ fontWeight: "bold" }}>
              {game.playerWon ? "YOU WON!" : "YOU LOST"}
            </Typography>
          </Box>

          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              DECLARER: {game.declarerScore}
            </Typography>
            <Typography variant="h6" gutterBottom>
              OPPONENTS: {game.opponentScore}
            </Typography>

            {game.isSchneider && !game.isNull && (
              <Chip
                label={game.isSchwarz ? "SCHWARZ!" : "SCHNEIDER!"}
                color="secondary"
                sx={{ mt: 2 }}
              />
            )}

            {game.isNull && (
              <Typography variant="body1" sx={{ mt: 2 }}>
                {game.declarerTricks === 0
                  ? "Perfect null game - no tricks taken!"
                  : `Null game failed - ${game.declarerTricks} trick${
                      game.declarerTricks > 1 ? "s" : ""
                    } taken`}
              </Typography>
            )}
          </Box>

          <Box sx={{ mb: 3 }}>
            <Typography variant="body2" color="text.secondary">
              GAME MODE: {game.gameMode || "1"}
              {game.trumpSuit && ` (${game.trumpSuit})`}
            </Typography>
          </Box>

          <Box sx={{ mb: 3 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
              Declarer
            </Typography>
            <Typography>
              {game.declarer ? (
                <>
                  {game.declarer.name}
                  {game.declarer.is_agent && " (AI)"}
                  {game.declarer.player_id === game.playerId && " (You)"}
                </>
              ) : (
                "Unknown"
              )}
            </Typography>

            <Typography
              variant="subtitle2"
              sx={{ mt: 2, fontWeight: "bold" }}
            >
              Opponents
            </Typography>
            {game.opponents.map((player) => (
              <Typography key={player.player_id}>
                {player.name}
                {player.is_agent && " (AI)"}
                {player.player_id === game.playerId && " (You)"}
              </Typography>
            ))}
          </Box>

          <Button
            variant="contained"
            color="primary"
            size="large"
            fullWidth
            onClick={handlePlayAgain}
          >
            Play Again
          </Button>
        </Paper>
      </Modal>

      {/* Message display for game actions */}
      <Box
        sx={{
          position: "fixed",
          bottom: 20,
          right: 20,
          maxWidth: 400,
          zIndex: 1000,
        }}
      >
        {game.messages
          .filter((message) => message.playerPosition === undefined)
          .map((message) => (
            <Alert
              key={message.id}
              severity={message.isError ? "error" : "info"}
              sx={{ mb: 1 }}
            >
              {message.text}
            </Alert>
          ))}
      </Box>
    </Box>
  );
}

export default function GameScreen() {
  return (
    <GameProvider>
      <GameScreenContent />
    </GameProvider>
  );
}