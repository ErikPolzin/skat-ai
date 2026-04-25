import React, { useState, useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ThemeProvider, createTheme } from "@mui/material/styles";
import {
  CssBaseline,
  Box,
  Container,
  Typography,
  Button,
  CircularProgress,
} from "@mui/material";
import {
  useProfileStore,
  selectUsername,
  selectPlayerId,
  selectSetUsername,
  selectSetPlayerId,
  selectSetProfileIcon,
} from "./stores/profileStore";
import { createOrRetrieveProfile } from "./api/games";
import { WebSocketProvider } from "./context/WebSocketContext";
import UsernameScreen from "./screens/UsernameScreen";
import LobbyScreen from "./screens/LobbyScreen";
import GameScreen from "./screens/GameScreen";

// Create MUI theme with dark mode
const theme = createTheme({
  palette: {
    mode: "dark",
    primary: {
      main: "#be47d6",
    },
    secondary: {
      main: "#7ca8d0",
    },
    error: {
      main: "#e74c3c",
    },
    background: {
      default: "#1a1a2e",
      paper: "#16213e",
    },
  },
});

function App() {
  const username = useProfileStore(selectUsername);
  const playerId = useProfileStore(selectPlayerId);
  const setUsername = useProfileStore(selectSetUsername);
  const setPlayerId = useProfileStore(selectSetPlayerId);
  const setProfileIcon = useProfileStore(selectSetProfileIcon);
  const [isInitializing, setIsInitializing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Initialize profile when username is set but no player ID exists
  useEffect(() => {
    if (username && !playerId && !isInitializing) {
      setIsInitializing(true);
      createOrRetrieveProfile(username)
        .then((profile) => {
          setPlayerId(profile.player_id);
          if (profile.player_name !== username) {
            setUsername(profile.player_name);
          }
          if (profile.profile_icon) {
            setProfileIcon(profile.profile_icon);
          }
          setError(null);
        })
        .catch((err) => {
          console.error("Failed to create profile:", err);
          setError("Failed to connect to server. Please try again.");
        })
        .finally(() => {
          setIsInitializing(false);
        });
    }
  }, [
    username,
    playerId,
    isInitializing,
    setPlayerId,
    setUsername,
    setProfileIcon,
  ]);

  // Handle username submission
  const handleUsernameSubmit = async (newUsername: string) => {
    setError(null);
    setIsInitializing(true);

    try {
      // Try to retrieve existing profile with current playerId (if any)
      const profile = await createOrRetrieveProfile(
        newUsername,
        playerId || undefined,
      );
      setPlayerId(profile.player_id);
      setUsername(profile.player_name);
      if (profile.profile_icon) {
        setProfileIcon(profile.profile_icon);
      }
    } catch (err) {
      console.error("Failed to create profile:", err);
      setError("Failed to connect to server. Please try again.");
      // Still set username locally so user can retry
      setUsername(newUsername);
    } finally {
      setIsInitializing(false);
    }
  };

  // Show username screen if no username or still initializing without a player ID
  if (!username || (username && !playerId && !error)) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box className="App">
          <BrowserRouter>
            {isInitializing ? (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  minHeight: "100vh",
                }}
              >
                <Container maxWidth="sm">
                  <Box sx={{ textAlign: "center" }}>
                    <CircularProgress sx={{ mb: 2 }} />
                    <Typography variant="h5">
                      Connecting to server...
                    </Typography>
                  </Box>
                </Container>
              </Box>
            ) : (
              <UsernameScreen onSubmit={handleUsernameSubmit} />
            )}
          </BrowserRouter>
        </Box>
      </ThemeProvider>
    );
  }

  // Show error screen if profile creation failed
  if (error && !playerId) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box className="App">
          <BrowserRouter>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                minHeight: "100vh",
              }}
            >
              <Container maxWidth="sm">
                <Box sx={{ textAlign: "center" }}>
                  <Typography variant="h4" gutterBottom>
                    Connection Error
                  </Typography>
                  <Typography color="error" sx={{ mb: 3 }}>
                    {error}
                  </Typography>
                  <Button
                    variant="contained"
                    color="primary"
                    onClick={() => {
                      setError(null);
                      if (username) {
                        handleUsernameSubmit(username);
                      }
                    }}
                  >
                    Retry
                  </Button>
                </Box>
              </Container>
            </Box>
          </BrowserRouter>
        </Box>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <WebSocketProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<LobbyScreen />} />
            <Route path="/game/:gameId" element={<GameScreen />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </WebSocketProvider>
    </ThemeProvider>
  );
}

export default App;
