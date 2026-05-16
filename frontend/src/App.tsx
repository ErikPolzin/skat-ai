import { useState, useEffect } from "react";
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  Outlet,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { ThemeProvider, createTheme } from "@mui/material/styles";
import {
  CssBaseline,
  Box,
  Container,
  Typography,
  CircularProgress,
  Snackbar,
  Alert,
} from "@mui/material";
import {
  useProfileStore,
  selectUsername,
  selectPassword,
  selectPlayerId,
  selectSetUsername,
  selectSetPassword,
  selectSetPlayerId,
  selectSetProfileIcon,
  selectClearProfile,
} from "./stores/profileStore";
import { createOrRetrieveProfile } from "./api/games";
import { WebSocketProvider } from "./context/WebSocketContext";
import LoginScreen from "./screens/LoginScreen";
import LobbyScreen from "./screens/LobbyScreen";
import GameScreen from "./screens/GameScreen";
import HistoryScreen from "./screens/HistoryScreen";
import { useSnackbarStore } from "./stores/snackbarStore";

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
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </ThemeProvider>
  );
}

function AppRoutes() {
  const username = useProfileStore(selectUsername);
  const password = useProfileStore(selectPassword);
  const playerId = useProfileStore(selectPlayerId);
  const setUsername = useProfileStore(selectSetUsername);
  const setPassword = useProfileStore(selectSetPassword);
  const setPlayerId = useProfileStore(selectSetPlayerId);
  const setProfileIcon = useProfileStore(selectSetProfileIcon);
  const clearProfile = useProfileStore(selectClearProfile);
  const [isInitializing, setIsInitializing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { open, message, severity, hideSnackbar } = useSnackbarStore();

  // Initialize profile when username is set but no player ID exists
  useEffect(() => {
    if (username && password && !playerId && !isInitializing) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setIsInitializing(true);
      createOrRetrieveProfile(username, password)
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
          clearProfile();
          setError("We could not sign you in. Check your username and password.");
        })
        .finally(() => {
          setIsInitializing(false);
        });
    }
  }, [
    username,
    password,
    playerId,
    isInitializing,
    setPlayerId,
    setUsername,
    setPassword,
    setProfileIcon,
    clearProfile,
  ]);

  const handleLogin = async (
    newUsername: string,
    newPassword: string,
  ) => {
    setError(null);
    setIsInitializing(true);

    try {
      const profile = await createOrRetrieveProfile(newUsername, newPassword);
      setPlayerId(profile.player_id);
      setUsername(profile.player_name);
      setPassword(newPassword);
      if (profile.profile_icon) {
        setProfileIcon(profile.profile_icon);
      }
    } catch (err) {
      console.error("Failed to create profile:", err);
      clearProfile();
      setError("We could not sign you in. Check your username and password.");
      throw err;
    } finally {
      setIsInitializing(false);
    }
  };

  return (
    <>
      <Routes>
        <Route
          path="/login"
          element={
            <LoginRoute
              isAuthenticated={Boolean(username && password && playerId)}
              isSubmitting={isInitializing}
              error={error}
              onSubmit={handleLogin}
            />
          }
        />
        <Route
          element={
            <ProtectedRoutes
              hasCredentials={Boolean(username && password)}
              isAuthenticated={Boolean(username && password && playerId)}
              isInitializing={isInitializing}
              error={error}
            />
          }
        >
          <Route path="/" element={<LobbyScreen />} />
          <Route path="/history" element={<HistoryScreen />} />
          <Route path="/game/:gameId" element={<GameScreen />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <Snackbar
        open={open}
        autoHideDuration={6000}
        onClose={hideSnackbar}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert
          onClose={hideSnackbar}
          severity={severity}
          sx={{ width: "100%" }}
        >
          {message}
        </Alert>
      </Snackbar>
    </>
  );
}

function LoginRoute({
  isAuthenticated,
  isSubmitting,
  error,
  onSubmit,
}: {
  isAuthenticated: boolean;
  isSubmitting: boolean;
  error: string | null;
  onSubmit: (username: string, password: string) => Promise<void>;
}) {
  const location = useLocation();
  const navigate = useNavigate();
  const from = (location.state as { from?: { pathname?: string } } | null)?.from
    ?.pathname;

  if (isAuthenticated) {
    return <Navigate to={from || "/"} replace />;
  }

  return (
    <LoginScreen
      isSubmitting={isSubmitting}
      error={
        error ||
        (location.state as { error?: string } | null)?.error ||
        null
      }
      onSubmit={async (username, password) => {
        await onSubmit(username, password);
        navigate(from || "/", { replace: true });
      }}
    />
  );
}

function ProtectedRoutes({
  hasCredentials,
  isAuthenticated,
  isInitializing,
  error,
}: {
  hasCredentials: boolean;
  isAuthenticated: boolean;
  isInitializing: boolean;
  error: string | null;
}) {
  const location = useLocation();

  if (!hasCredentials) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  if (error && !isAuthenticated) {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: location, error }}
      />
    );
  }

  if (!isAuthenticated || isInitializing) {
    return <AuthLoadingScreen />;
  }

  return (
    <WebSocketProvider>
      <Outlet />
    </WebSocketProvider>
  );
}

function AuthLoadingScreen() {
  return (
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
          <Typography variant="h5">Signing you in...</Typography>
        </Box>
      </Container>
    </Box>
  );
}

export default App;
