import React, { useEffect, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Divider,
  Paper,
  TextField,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import CasinoIcon from "@mui/icons-material/Casino";
import LoginIcon from "@mui/icons-material/Login";
import cardSpade from "../assets/Card_spade.svg";
import cardHeart from "../assets/Card_heart.svg";

interface LoginScreenProps {
  isSubmitting?: boolean;
  error?: string | null;
  onSubmit: (username: string, password: string) => Promise<void>;
}

export default function LoginScreen({
  isSubmitting = false,
  error,
  onSubmit,
}: LoginScreenProps) {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
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
          "radial-gradient(circle at 18% 18%, rgba(214, 61, 84, 0.18), transparent 28%), linear-gradient(135deg, #0d2b24 0%, #18241f 46%, #221f26 100%)",
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
                "linear-gradient(160deg, rgba(10, 76, 58, 0.95), rgba(35, 37, 34, 0.96))",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.2 }}>
              <CasinoIcon sx={{ color: "#f3c96b" }} />
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
                height: { xs: 154, md: 224 },
                transform: "rotate(8deg)",
                borderRadius: 1.5,
                bgcolor: "#f6f0df",
                boxShadow: "0 24px 60px rgba(0, 0, 0, 0.35)",
                display: "grid",
                placeItems: "center",
              }}
            >
              <Box
                component="img"
                src={cardSpade}
                alt=""
                sx={{ width: "44%", opacity: 0.95 }}
              />
            </Box>
            {!isMobile && (
              <Box
                sx={{
                  position: "absolute",
                  right: 150,
                  top: 170,
                  width: 138,
                  height: 184,
                  transform: "rotate(-10deg)",
                  borderRadius: 1.5,
                  bgcolor: "#fff8ea",
                  boxShadow: "0 20px 50px rgba(0, 0, 0, 0.28)",
                  display: "grid",
                  placeItems: "center",
                }}
              >
                <Box
                  component="img"
                  src={cardHeart}
                  alt=""
                  sx={{ width: "42%", opacity: 0.95 }}
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
                Sign in to rejoin your tables, track your rating, and keep your
                games tied to your profile.
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
                Player Login
              </Typography>
              <Typography
                variant="h4"
                component="h2"
                sx={{ fontWeight: 750, mt: 0.5, mb: 1 }}
              >
                Welcome back
              </Typography>
              <Typography color="text.secondary" sx={{ mb: 3 }}>
                Use your player name and password to continue.
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
                  autoComplete="current-password"
                  sx={{ mb: 3 }}
                />
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
                      <LoginIcon />
                    )
                  }
                  sx={{ minHeight: 48 }}
                >
                  {isSubmitting ? "Signing in..." : "Sign in"}
                </Button>
              </Box>

              <Divider sx={{ my: 3 }} />
              <Typography variant="body2" color="text.secondary">
                New names are created automatically on first sign-in.
              </Typography>
            </Box>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
