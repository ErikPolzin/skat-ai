import React from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  Typography,
  Paper,
  CircularProgress,
  Alert,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import WarningIcon from "@mui/icons-material/Warning";
import { GameProvider, useGameContext } from "../context/GameContext";
import { MotionCardTable } from "../components/MotionCardTable";
import { SessionResults } from "../components/SessionResults";

function GameScreenContent() {
  const game = useGameContext();
  const navigate = useNavigate();
  const theme = useTheme();
  const isWideScreen = useMediaQuery(theme.breakpoints.up("md")); // 900px+

  const handleRetryLoad = () => {
    window.location.reload();
  };

  // Show loading state
  if (game.isLoading) {
    return (
      <Box
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
            playerId={game.player?.id}
            gameId={game.gameId}
            gamesPlayed={game.gamesPlayed}
            maxGames={10}
            players={game.players}
          />
        </Box>
      )}

      {/* Session results FAB for mobile */}
      {!isWideScreen && game.playerCount === 3 && (
        <SessionResults
          results={game.sessionResults}
          playerId={game.player?.id}
          gameId={game.gameId}
          gamesPlayed={game.gamesPlayed}
          maxGames={10}
          players={game.players}
        />
      )}

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
