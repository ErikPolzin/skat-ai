import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Typography,
  TextField,
  Button,
  Paper,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Alert,
  IconButton,
  CircularProgress,
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
  const [isLoading, setIsLoading] = useState(false);
  const [isFetching, setIsFetching] = useState(false);

  useEffect(() => {
    fetchGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchGames = async () => {
    try {
      setIsFetching(true);
      const data = await getGames();
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch games:", error);
    } finally {
      setIsFetching(false);
    }
  };

  const handleJoinOrCreate = async () => {
    let currentGameCode = gameCode.trim();
    try {
      setError(null);
      setIsLoading(true);

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
    } finally {
      setIsLoading(false);
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
        alignItems: { xs: "stretch", sm: "center" },
        justifyContent: { xs: "stretch", sm: "center" },
        minHeight: "100vh",
        py: { xs: 0, sm: 3 },
      }}
    >
      <Box
        sx={{
          width: { xs: "100%", sm: "auto" },
          maxWidth: { xs: "100%", sm: "900px" },
          mx: { xs: 0, sm: "auto" },
        }}
      >
        <Paper
          elevation={3}
          sx={{
            p: { xs: 2, sm: 3, md: 4 },
            minWidth: { xs: "auto", sm: "500px" },
            width: "100%",
            borderRadius: { xs: 0, sm: 1 },
            minHeight: { xs: "100vh", sm: "auto" },
          }}
        >
          <Typography
            variant="h4"
            component="h1"
            gutterBottom
            sx={{ fontSize: { xs: "1.5rem", sm: "2.125rem" } }}
          >
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
            <Typography variant="subtitle1" gutterBottom>
              Join or Create Game
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <TextField
                placeholder="Enter game code"
                value={gameCode}
                onChange={(e) => setGameCode(e.target.value.toUpperCase())}
                disabled={isLoading}
                sx={{
                  "& input": {
                    textTransform: "uppercase",
                    textAlign: "center",
                    letterSpacing: "2px",
                  },
                }}
                fullWidth
              />
              <Button
                variant="contained"
                color="primary"
                onClick={handleJoinOrCreate}
                disabled={isLoading}
                size="large"
                fullWidth={!gameCode}
                startIcon={isLoading ? <CircularProgress size={20} /> : null}
              >
                {isLoading
                  ? "Loading..."
                  : gameCode
                    ? "Join Game"
                    : "Create Game"}
              </Button>
            </Box>
          </Box>

          <Box
            sx={{
              minHeight: "200px",
              display: "flex",
              flexDirection: "column",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Typography variant="subtitle1">Available Games</Typography>
              <IconButton
                onClick={fetchGames}
                color="primary"
                disabled={isFetching}
              >
                {isFetching ? <CircularProgress size={24} /> : <RefreshIcon />}
              </IconButton>
            </Box>

            {isFetching ? (
              <Box
                sx={{
                  flexGrow: 1,
                  display: "flex",
                  justifyContent: "center",
                  alignItems: "center",
                }}
              >
                <CircularProgress />
              </Box>
            ) : games.length === 0 ? (
              <Box
                sx={{
                  flexGrow: 1,
                  display: "flex",
                  justifyContent: "center",
                  alignItems: "center",
                }}
              >
                <Typography sx={{ py: 2 }} color="textDisabled">
                  No active games
                </Typography>
              </Box>
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
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1,
                          }}
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
                        disabled={isLoading}
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
      </Box>
    </Box>
  );
}
