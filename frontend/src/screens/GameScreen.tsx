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
  useMediaQuery,
  useTheme,
} from "@mui/material";
import EmojiEventsIcon from "@mui/icons-material/EmojiEvents";
import HeartBrokenIcon from "@mui/icons-material/HeartBroken";
import WarningIcon from "@mui/icons-material/Warning";
import { GameProvider, useGameContext } from "../context/GameContext";
import { MotionCardTable } from "../components/MotionCardTable";
import { SessionResults } from "../components/SessionResults";
import "./GameScreen.css";

function GameScreenContent() {
  const game = useGameContext();
  const navigate = useNavigate();
  const theme = useTheme();
  const isWideScreen = useMediaQuery(theme.breakpoints.up("md")); // 900px+

  const handlePlayAgain = () => {
    navigate("/");
  };

  const handlePlayNextGame = async () => {
    try {
      await game.controls.playNextGame();
    } catch (error) {
      console.error("Failed to start next game:", error);
    }
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
          minHeight: "100vh",
        }}
      >
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
          minHeight: "100vh",
        }}
      >
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
    <Box
      className="game-screen"
      sx={{
        display: "flex",
        flexDirection: "row",
        height: "100vh",
        width: "100vw",
      }}
    >
      {/* Main game area - 2/3 width when sidebar is visible */}
      <Box
        sx={{
          flex: isWideScreen && game.playerCount === 3 ? 2 : 1,
          position: "relative",
          height: "100vh",
          overflow: "hidden",
          minWidth: 0, // Allow flex item to shrink below content size
        }}
      >
        <MotionCardTable />
      </Box>

      {/* Sidebar for session info on wide screens - 1/3 width */}
      {isWideScreen && game.playerCount === 3 && (
        <Box
          sx={{
            flex: 1,
            flexShrink: 0,
            minWidth: 300,
            maxWidth: 450,
            height: "100vh",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <SessionResults
            results={game.sessionResults}
            playerId={game.playerId}
            gamesPlayed={game.gamesPlayed}
            maxGames={10}
            players={game.players}
          />
        </Box>
      )}

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
              <EmojiEventsIcon sx={{ fontSize: 80, color: "gold", mb: 2 }} />
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
                {game.declarerScore === 0
                  ? "Perfect null game - no tricks taken!"
                  : `Null game failed - ${game.declarerScore} trick${
                      game.declarerScore > 1 ? "s" : ""
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
                  {game.declarer.id === game.playerId && " (You)"}
                </>
              ) : (
                "Unknown"
              )}
            </Typography>

            <Typography variant="subtitle2" sx={{ mt: 2, fontWeight: "bold" }}>
              Opponents
            </Typography>
            {game.opponents.map((player) => (
              <Typography key={player.id}>
                {player.name}
                {player.is_agent && " (AI)"}
                {player.id === game.playerId && " (You)"}
              </Typography>
            ))}
          </Box>

          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {game.canPlayNext && (
              <Button
                variant="contained"
                color="primary"
                size="large"
                fullWidth
                onClick={handlePlayNextGame}
              >
                Play Another Game ({game.gamesPlayed + 1}/10)
              </Button>
            )}
            <Button
              variant={game.canPlayNext ? "outlined" : "contained"}
              color="primary"
              size="large"
              fullWidth
              onClick={handlePlayAgain}
            >
              Back to Lobby
            </Button>
          </Box>
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
