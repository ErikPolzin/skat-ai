import React, { useState, useEffect, useRef } from "react";
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
  Avatar,
  Badge,
  Tabs,
  Tab,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import PhotoCameraIcon from "@mui/icons-material/PhotoCamera";
import {
  createGame,
  joinGame,
  getGames,
  uploadAvatar,
  type GameSession,
} from "../api/games";
import { useProfileStore } from "../stores/profileStore";
import ActiveGames from "../components/ActiveGames";
import PlayerHistory from "../components/PlayerHistory";

interface LobbyScreenProps {
  username: string;
}

export default function LobbyScreen({ username }: LobbyScreenProps) {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const navigate = useNavigate();
  const profilePlayerId = useProfileStore((state) => state.playerId);
  const profileIcon = useProfileStore((state) => state.profileIcon);
  const setProfileIcon = useProfileStore((state) => state.setProfileIcon);
  const [gameCode, setGameCode] = useState("");
  const [games, setGames] = useState<GameSession[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isFetching, setIsFetching] = useState(false);
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const [currentTab, setCurrentTab] = useState(0);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchGames = async () => {
    try {
      setIsFetching(true);
      const data = await getGames(profilePlayerId || undefined);
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
        const createData = await createGame(profilePlayerId || undefined);
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

  const handleAvatarClick = () => {
    fileInputRef.current?.click();
  };

  const handleAvatarChange = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (!file || !profilePlayerId) return;

    // Validate file type
    if (!file.type.startsWith("image/")) {
      setError("Please select an image file");
      return;
    }

    // Validate file size (5MB max)
    if (file.size > 5 * 1024 * 1024) {
      setError("Image must be smaller than 5MB");
      return;
    }

    try {
      setIsUploadingAvatar(true);
      setError(null);

      const result = await uploadAvatar(profilePlayerId, file);
      setProfileIcon(result.profile_icon);
    } catch (error) {
      console.error("Failed to upload avatar:", error);
      setError((error as Error).message);
    } finally {
      setIsUploadingAvatar(false);
    }
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
            width: "100%",
            borderRadius: { xs: 0, sm: 1 },
            minHeight: { xs: "100vh", sm: "auto" },
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              mb: 2,
            }}
          >
            <Badge
              overlap="circular"
              anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
              badgeContent={
                <IconButton
                  size="small"
                  onClick={handleAvatarClick}
                  disabled={isUploadingAvatar || !profilePlayerId}
                  sx={{
                    bgcolor: "primary.main",
                    color: "white",
                    "&:hover": { bgcolor: "primary.dark" },
                    width: 32,
                    height: 32,
                  }}
                >
                  {isUploadingAvatar ? (
                    <CircularProgress size={16} color="inherit" />
                  ) : (
                    <PhotoCameraIcon sx={{ fontSize: 16 }} />
                  )}
                </IconButton>
              }
            >
              <Avatar
                src={profileIcon || undefined}
                alt={username}
                sx={{
                  width: 60,
                  height: 60,
                  fontSize: "2rem",
                  bgcolor: "secondary.main",
                }}
              >
                {username.charAt(0).toUpperCase()}
              </Avatar>
            </Badge>
            <Box>
              <Typography
                variant="h4"
                component="h1"
                sx={{ fontSize: { xs: "1.5rem", sm: "2.125rem" } }}
              >
                Welcome, {username}!
              </Typography>
            </Box>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              style={{ display: "none" }}
              onChange={handleAvatarChange}
            />
          </Box>

          {error && (
            <Alert
              severity="error"
              sx={{ mb: 2 }}
              onClose={() => setError(null)}
            >
              {error}
            </Alert>
          )}

          {isMobile ? (
            <>
              <Tabs
                value={currentTab}
                onChange={(_, newValue) => setCurrentTab(newValue)}
                variant="fullWidth"
                sx={{ mb: 2 }}
              >
                <Tab label="Games" />
                <Tab label="History" />
              </Tabs>

              {currentTab === 0 && (
                <>
                  <Box sx={{ mb: 4 }}>
                    <Typography variant="subtitle1" gutterBottom>
                      Join or Create Game
                    </Typography>
                    <Box
                      sx={{ display: "flex", flexDirection: "column", gap: 2 }}
                    >
                      <TextField
                        placeholder="Enter game code"
                        value={gameCode}
                        onChange={(e) =>
                          setGameCode(e.target.value.toUpperCase())
                        }
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
                        startIcon={
                          isLoading ? <CircularProgress size={20} /> : null
                        }
                      >
                        {isLoading
                          ? "Loading..."
                          : gameCode
                            ? "Join Game"
                            : "Create Game"}
                      </Button>
                    </Box>
                  </Box>

                  <ActiveGames playerId={profilePlayerId} />

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
                      <Typography variant="subtitle1">
                        Available Games
                      </Typography>
                      <IconButton
                        onClick={fetchGames}
                        color="primary"
                        disabled={isFetching}
                      >
                        {isFetching ? (
                          <CircularProgress size={24} />
                        ) : (
                          <RefreshIcon />
                        )}
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
                          <ListItem key={game.id}>
                            <ListItemText
                              primary={game.code}
                              secondary={`${game.player_count}/3 players`}
                            />
                            <ListItemSecondaryAction>
                              <Button
                                variant="text"
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
                </>
              )}

              {currentTab === 1 && <PlayerHistory playerId={profilePlayerId} />}
            </>
          ) : (
            <Box sx={{ display: "flex", gap: 3, alignItems: "flex-start" }}>
              <Box sx={{ flex: 1, minWidth: 400 }}>
                <Box sx={{ mb: 4 }}>
                  <Typography variant="subtitle1" gutterBottom>
                    Join or Create Game
                  </Typography>
                  <Box
                    sx={{ display: "flex", flexDirection: "column", gap: 2 }}
                  >
                    <TextField
                      placeholder="Enter game code"
                      value={gameCode}
                      onChange={(e) =>
                        setGameCode(e.target.value.toUpperCase())
                      }
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
                      startIcon={
                        isLoading ? <CircularProgress size={20} /> : null
                      }
                    >
                      {isLoading
                        ? "Loading..."
                        : gameCode
                          ? "Join Game"
                          : "Create Game"}
                    </Button>
                  </Box>
                </Box>

                <ActiveGames playerId={profilePlayerId} />

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
                      {isFetching ? (
                        <CircularProgress size={24} />
                      ) : (
                        <RefreshIcon />
                      )}
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
                        <ListItem key={game.id}>
                          <ListItemText
                            primary={game.code}
                            secondary={`${game.player_count}/3 players`}
                          />
                          <ListItemSecondaryAction>
                            <Button
                              variant="text"
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
              </Box>

              <Box sx={{ flex: 1, minWidth: 400 }}>
                <PlayerHistory playerId={profilePlayerId} />
              </Box>
            </Box>
          )}
        </Paper>
      </Box>
    </Box>
  );
}
