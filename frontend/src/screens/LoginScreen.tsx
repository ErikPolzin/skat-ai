import React, { useEffect, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Divider,
  Link,
  Paper,
  TextField,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import CasinoIcon from "@mui/icons-material/Casino";
import LoginIcon from "@mui/icons-material/Login";
import PersonAddIcon from "@mui/icons-material/PersonAdd";
import { Link as RouterLink } from "react-router-dom";

const cardSpade = "/res/cards/A♠.svg";
const cardHeart = "/res/cards/J♥.svg";

interface LoginScreenProps {
  isSubmitting?: boolean;
  error?: string | null;
  mode?: "sign-in" | "sign-up";
  onSwitchMode?: () => void;
  onSubmit: (username: string, password: string) => Promise<void>;
}

export default function LoginScreen({
  isSubmitting = false,
  error,
  mode = "sign-in",
  onSwitchMode,
  onSubmit,
}: LoginScreenProps) {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);

  useEffect(() => {
    const savedUsername = localStorage.getItem("skat-username");
    if (savedUsername) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setUsername(savedUsername);
    }
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const name = username.trim();
    if (!name || !password) {
      setLocalError("Enter your username and password.");
      return;
    }
    if (mode === "sign-up" && password !== confirmPassword) {
      setLocalError("Passwords do not match.");
      return;
    }

    setLocalError(null);
    localStorage.setItem("skat-username", name);
    await onSubmit(name, password);
  };

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        py: { xs: 3, md: 6 },
        background:
          "radial-gradient(circle at 18% 18%, rgba(190, 71, 214, 0.28), transparent 32%), linear-gradient(135deg, #25102c 0%, #1a1a2e 48%, #211627 100%)",
      }}
    >
      <Container maxWidth="lg">
        <Paper
          elevation={8}
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "minmax(320px, 0.9fr) 1fr" },
            overflow: "hidden",
            borderRadius: 2,
            bgcolor: "rgba(18, 25, 25, 0.94)",
            border: "1px solid rgba(255, 255, 255, 0.08)",
          }}
        >
          <Box
            sx={{
              position: "relative",
              minHeight: { xs: 190, md: 560 },
              p: { xs: 3, md: 5 },
              display: "flex",
              flexDirection: "column",
              justifyContent: "space-between",
              background:
                "linear-gradient(160deg, rgba(190, 71, 214, 0.96), rgba(70, 24, 82, 0.98))",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.2 }}>
              <CasinoIcon sx={{ color: "primary.contrastText" }} />
              <Typography variant="h6" component="p" sx={{ fontWeight: 700 }}>
                Skat
              </Typography>
            </Box>

            <Box
              sx={{
                position: "absolute",
                right: { xs: 24, md: 48 },
                top: { xs: 30, md: 82 },
                width: { xs: 116, md: 168 },
                aspectRatio: "167 / 243",
                transform: "rotate(8deg)",
                borderRadius: 1.5,
                boxShadow: "0 24px 60px rgba(0, 0, 0, 0.35)",
                overflow: "hidden",
              }}
            >
              <Box
                component="img"
                src={cardSpade}
                alt="Ace of spades"
                sx={{ width: "100%", height: "100%", display: "block" }}
              />
            </Box>
            {!isMobile && (
              <Box
                sx={{
                  position: "absolute",
                  right: 150,
                  top: 170,
                  width: 138,
                  aspectRatio: "167 / 243",
                  transform: "rotate(-10deg)",
                  borderRadius: 1.5,
                  boxShadow: "0 20px 50px rgba(0, 0, 0, 0.28)",
                  overflow: "hidden",
                }}
              >
                <Box
                  component="img"
                  src={cardHeart}
                  alt="Jack of hearts"
                  sx={{ width: "100%", height: "100%", display: "block" }}
                />
              </Box>
            )}

            <Box sx={{ maxWidth: 360, position: "relative", zIndex: 1 }}>
              <Typography
                variant="h3"
                component="h1"
                sx={{
                  fontWeight: 800,
                  fontSize: { xs: "2rem", md: "3.25rem" },
                  lineHeight: 1.05,
                  mb: 2,
                }}
              >
                Take your seat.
              </Typography>
              <Typography color="rgba(255,255,255,0.72)" sx={{ maxWidth: 320 }}>
                {mode === "sign-up"
                  ? "Create your player profile, take a seat, and start building your rating."
                  : "Sign in to rejoin your tables, track your rating, and keep your games tied to your profile."}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              p: { xs: 3, sm: 5, md: 7 },
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
            }}
          >
            <Box sx={{ maxWidth: 430, width: "100%", mx: "auto" }}>
              <Typography variant="overline" color="text.secondary">
                {mode === "sign-up" ? "Create account" : "Player Login"}
              </Typography>
              <Typography
                variant="h4"
                component="h2"
                sx={{ fontWeight: 750, mt: 0.5, mb: 1 }}
              >
                {mode === "sign-up" ? "Join the table" : "Welcome back"}
              </Typography>
              <Typography color="text.secondary" sx={{ mb: 3 }}>
                {mode === "sign-up"
                  ? "Choose a player name and password to get started."
                  : "Use your player name and password to continue."}
              </Typography>

              {(error || localError) && (
                <Alert severity="error" sx={{ mb: 3 }}>
                  {localError || error}
                </Alert>
              )}

              <Box component="form" onSubmit={handleSubmit}>
                <TextField
                  id="username"
                  label="Username"
                  placeholder="Player name"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  disabled={isSubmitting}
                  fullWidth
                  autoFocus
                  autoComplete="username"
                  sx={{ mb: 2.5 }}
                />
                <TextField
                  id="password"
                  label="Password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isSubmitting}
                  fullWidth
                  required
                  autoComplete={mode === "sign-up" ? "new-password" : "current-password"}
                  sx={{ mb: 3 }}
                />
                {mode === "sign-up" && (
                  <TextField
                    id="confirm-password"
                    label="Confirm password"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    disabled={isSubmitting}
                    fullWidth
                    required
                    autoComplete="new-password"
                    sx={{ mb: 3 }}
                  />
                )}
                <Button
                  type="submit"
                  variant="contained"
                  color="primary"
                  fullWidth
                  size="large"
                  disabled={isSubmitting}
                  startIcon={
                    isSubmitting ? (
                      <CircularProgress size={18} color="inherit" />
                    ) : (
                      mode === "sign-up" ? <PersonAddIcon /> : <LoginIcon />
                    )
                  }
                  sx={{ minHeight: 48 }}
                >
                  {isSubmitting
                    ? mode === "sign-up"
                      ? "Creating account..."
                      : "Signing in..."
                    : mode === "sign-up"
                      ? "Create account"
                      : "Sign in"}
                </Button>
              </Box>

              <Divider sx={{ my: 3 }} />
              <Typography variant="body2" color="text.secondary">
                {mode === "sign-up" ? "Already have an account? " : "New to Skat? "}
                <Link
                  component={RouterLink}
                  to={mode === "sign-up" ? "/login" : "/signup"}
                  onClick={onSwitchMode}
                  color="primary"
                  underline="hover"
                >
                  {mode === "sign-up" ? "Sign in" : "Create an account"}
                </Link>
              </Typography>
            </Box>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
