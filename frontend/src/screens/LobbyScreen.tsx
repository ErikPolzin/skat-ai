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
  Chip,
  IconButton,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import { createGame, joinGame, getGames, type GameState } from "../api/games";
import { useProfileStore } from "../stores/profileStore";
import { GameHistory } from "../components/GameHistory";

interface LobbyScreenProps {
  username: string;
}

export default function LobbyScreen({ username }: LobbyScreenProps) {
  const navigate = useNavigate();
  const profilePlayerId = useProfileStore((state) => state.playerId);
  const setUsername = useProfileStore((state) => state.setUsername);
  const setPlayerId = useProfileStore((state) => state.setPlayerId);
  const [gameId, setGameId] = useState("");
  const [games, setGames] = useState<GameState[]>([]);
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
    let currentGameId = gameId.trim();

    try {
      setError(null);

      // Create game if no ID provided
      if (!currentGameId) {
        const data = await createGame();
        currentGameId = data.game_id;
      }

      // Get player credentials - send existing ID if we have one
      const data = await joinGame(
        currentGameId,
        username,
        profilePlayerId || undefined,
      );

      // Store player ID and name from server
      setPlayerId(data.player_id);
      if (data.player_name !== username) {
        setUsername(data.player_name);
      }

      // Navigate to the game
      navigate(`/game/${currentGameId}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
      setError((error as Error).message);
    }
  };

  const handleQuickJoin = (id: string) => {
    setGameId(id);
    setTimeout(() => handleJoinOrCreate(), 0);
  };

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        py: 3
      }}>
      <Container maxWidth="md">
        <Paper elevation={3} sx={{ p: 4 }}>
          <Typography variant="h4" component="h1" gutterBottom>
            Welcome, {username}!
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
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
                value={gameId}
                onChange={(e) => setGameId(e.target.value.toUpperCase())}
                sx={{
                  "& input": {
                    textTransform: "uppercase",
                    letterSpacing: "2px",
                  }
                }}
                fullWidth
              />
              <Button
                variant="contained"
                color="primary"
                onClick={handleJoinOrCreate}
                size="large"
                fullWidth={!gameId}
              >
                {gameId ? "Join Game" : "Create New Game"}
              </Button>
            </Box>
          </Box>

          <Divider sx={{ my: 3 }} />

          <Box sx={{ mb: 3 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
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
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                          <Typography variant="subtitle1" sx={{ fontWeight: "bold" }}>
                            {game.id}
                          </Typography>
                          <Typography color="text.secondary">
                            {game.players.length}/3 players
                          </Typography>
                          {game.phase !== "waiting" && (
                            <Chip label="In Progress" size="small" color="warning" />
                          )}
                        </Box>
                      }
                    />
                    <ListItemSecondaryAction>
                      <Button
                        variant="outlined"
                        onClick={() => handleQuickJoin(game.id)}
                        disabled={
                          game.phase !== "waiting" || game.players.length >= 3
                        }
                      >
                        Join
                      </Button>
                    </ListItemSecondaryAction>
                  </ListItem>
                ))}
              </List>
            )}
          </Box>

          <Divider sx={{ my: 3 }} />

          <Box>
            <Typography variant="h6" gutterBottom>
              Recent Games
            </Typography>
            <GameHistory playerId={profilePlayerId} />
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}