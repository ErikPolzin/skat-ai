import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Container,
  Typography,
  TextField,
  Button,
  Paper,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Alert,
  Divider,
  IconButton,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import { createGame, joinGame, getGames, type GameSession } from "../api/games";
import { useProfileStore } from "../stores/profileStore";

interface LobbyScreenProps {
  username: string;
}

export default function LobbyScreen({ username }: LobbyScreenProps) {
  const navigate = useNavigate();
  const profilePlayerId = useProfileStore((state) => state.playerId);
  const [gameCode, setGameCode] = useState("");
  const [games, setGames] = useState<GameSession[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchGames = async () => {
    try {
      const data = await getGames();
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch games:", error);
    }
  };

  const handleJoinOrCreate = async () => {
    let currentGameCode = gameCode.trim();
    try {
      setError(null);

      if (!currentGameCode) {
        // Create a new game and get the code
        const createData = await createGame();
        currentGameCode = createData.code;
      }

      // Join the game (either the newly created one or an existing one)
      const data = await joinGame(
        currentGameCode,
        username,
        profilePlayerId || undefined,
      );

      // Navigate to the game
      navigate(`/game/${data.game_id}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
      setError((error as Error).message);
    }
  };

  const handleQuickJoin = (code: string) => {
    setGameCode(code);
    setTimeout(() => handleJoinOrCreate(), 0);
  };

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        py: 3,
      }}
    >
      <Container maxWidth="md">
        <Paper elevation={3} sx={{ p: 4, minWidth: "500px" }}>
          <Typography variant="h4" component="h1" gutterBottom>
            Welcome, {username}!
          </Typography>

          {error && (
            <Alert
              severity="error"
              sx={{ mb: 2 }}
              onClose={() => setError(null)}
            >
              {error}
            </Alert>
          )}

          <Box sx={{ mb: 4 }}>
            <Typography variant="h6" gutterBottom>
              Join or Create Game
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <TextField
                placeholder="Enter game code or leave empty to create"
                value={gameCode}
                onChange={(e) => setGameCode(e.target.value.toUpperCase())}
                sx={{
                  "& input": {
                    textTransform: "uppercase",
                    letterSpacing: "2px",
                  },
                }}
                fullWidth
              />
              <Button
                variant="contained"
                color="primary"
                onClick={handleJoinOrCreate}
                size="large"
                fullWidth={!gameCode}
              >
                {gameCode ? "Join Game" : "Create New Game"}
              </Button>
            </Box>
          </Box>

          <Divider sx={{ my: 3 }} />

          <Box sx={{ mb: 3 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Typography variant="h6">Available Games</Typography>
              <IconButton onClick={fetchGames} color="primary">
                <RefreshIcon />
              </IconButton>
            </Box>

            {games.length === 0 ? (
              <Typography color="text.secondary" align="center" sx={{ py: 2 }}>
                No active games
              </Typography>
            ) : (
              <List>
                {games.map((game) => (
                  <ListItem
                    key={game.id}
                    sx={{
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 1,
                      mb: 1,
                    }}
                  >
                    <ListItemText
                      primary={
                        <Box
                          sx={{ display: "flex", alignItems: "center", gap: 1 }}
                        >
                          <Typography
                            variant="subtitle1"
                            sx={{ fontWeight: "bold" }}
                          >
                            {game.code}
                          </Typography>
                          <Typography color="text.secondary">
                            {game.player_count}/3 players
                          </Typography>
                        </Box>
                      }
                    />
                    <ListItemSecondaryAction>
                      <Button
                        variant="outlined"
                        onClick={() => handleQuickJoin(game.code)}
                      >
                        Join
                      </Button>
                    </ListItemSecondaryAction>
                  </ListItem>
                ))}
              </List>
            )}
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
