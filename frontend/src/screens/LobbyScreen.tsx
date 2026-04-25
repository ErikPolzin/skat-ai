import React, { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Typography,
  TextField,
  Button,
  Paper,
  IconButton,
  CircularProgress,
  Avatar,
  Badge,
  Tabs,
  Tab,
  useMediaQuery,
  useTheme,
  Grid,
  Container,
} from "@mui/material";
import PhotoCameraIcon from "@mui/icons-material/PhotoCamera";
import { createGame, joinGame, uploadAvatar } from "../api/games";
import {
  selectPlayerId,
  selectProfileIcon,
  selectUsername,
  useProfileStore,
} from "../stores/profileStore";
import ActiveGames from "../components/ActiveGames";
import PlayerHistory from "../components/PlayerHistory";
import Leaderboard from "../components/Leaderboard";
import { getPlayerRating, type PlayerRating } from "../api/games";
import AvailableGames from "../components/AvailableGames";

const Header = () => {
  const profileId = useProfileStore(selectPlayerId);
  const profileIcon = useProfileStore(selectProfileIcon);
  const username = useProfileStore(selectUsername);
  const setProfileIcon = useProfileStore((state) => state.setProfileIcon);
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const [playerRating, setPlayerRating] = useState<PlayerRating | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (profileId) {
      fetchPlayerRating();
    }
  }, [profileId]);

  const fetchPlayerRating = async () => {
    if (!profileId) return;

    try {
      const rating = await getPlayerRating(profileId);
      setPlayerRating(rating);
    } catch (error) {
      console.error("Failed to fetch player rating:", error);
    }
  };

  const handleAvatarClick = () => {
    fileInputRef.current?.click();
  };

  const handleAvatarChange = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (!file || !profileId) return;
    // Validate file type
    if (!file.type.startsWith("image/")) {
      return;
    }
    // Validate file size (5MB max)
    if (file.size > 5 * 1024 * 1024) {
      return;
    }
    try {
      setIsUploadingAvatar(true);
      const result = await uploadAvatar(profileId, file);
      setProfileIcon(result.profile_icon);
    } catch (error) {
      console.error("Failed to upload avatar:", error);
    } finally {
      setIsUploadingAvatar(false);
    }
  };
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 2,
        px: 2,
        pt: 1,
        mb: { xs: 0, lg: 2 },
      }}
    >
      <Badge
        overlap="circular"
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        badgeContent={
          <IconButton
            size="small"
            onClick={handleAvatarClick}
            disabled={isUploadingAvatar || !profileId}
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
          alt={username ?? "No username"}
          sx={{
            width: 60,
            height: 60,
            fontSize: "2rem",
            bgcolor: "secondary.main",
          }}
        >
          {(username || "-").charAt(0).toUpperCase()}
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
        {playerRating && (
          <Typography variant="body2" color="text.secondary">
            Rating: {playerRating.rating} • Rank: #{playerRating.rank || "N/A"}
          </Typography>
        )}
      </Box>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        style={{ display: "none" }}
        onChange={handleAvatarChange}
      />
    </Box>
  );
};

const GamesTab = () => {
  const [gameCode, setGameCode] = useState<string>("");
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const username = useProfileStore(selectUsername);
  const profileId = useProfileStore(selectPlayerId);
  const navigate = useNavigate();

  const handleJoinOrCreate = async () => {
    let currentGameCode = gameCode.trim();
    try {
      setIsLoading(true);

      if (!currentGameCode) {
        // Create a new game and get the code
        const createData = await createGame(profileId || undefined);
        currentGameCode = createData.code;
      }

      // Join the game (either the newly created one or an existing one)
      const data = await joinGame(
        currentGameCode,
        username ?? "",
        profileId || undefined,
      );

      // Navigate to the game
      navigate(`/game/${data.game_id}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="subtitle1" gutterBottom>
        Join or Create Game
      </Typography>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
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
          {isLoading ? "Loading..." : gameCode ? "Join Game" : "Create Game"}
        </Button>
      </Box>
      <ActiveGames />
      <AvailableGames />
    </Box>
  );
};

export default function LobbyScreen() {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const [currentTab, setCurrentTab] = useState(0);

  if (isMobile) {
    return (
      <Box sx={{ bgcolor: "background.default", height: "100vh" }}>
        <Box sx={{ bgcolor: "background.paper" }}>
          <Header />
          <Tabs
            value={currentTab}
            onChange={(_, newValue) => setCurrentTab(newValue)}
            variant="fullWidth"
            sx={{ mb: 2 }}
          >
            <Tab label="Games" />
            <Tab label="History" />
            <Tab label="Leaderboard" />
          </Tabs>
        </Box>

        <Box sx={{ px: 1 }}>
          {currentTab === 0 && <GamesTab />}
          {currentTab === 1 && <PlayerHistory />}
          {currentTab === 2 && <Leaderboard />}
        </Box>
      </Box>
    );
  }

  return (
    <Container
      sx={{
        py: { xs: 0, sm: 3 },
      }}
    >
      <Paper
        elevation={3}
        sx={{
          p: { xs: 0, sm: 2, md: 3 },
          width: "100%",
          borderRadius: { xs: 0, sm: 1 },
          minHeight: { xs: "100vh", sm: "auto" },
        }}
      >
        <Header />
        <Grid container spacing={2}>
          <Grid size={{ sm: 12, md: 6, xl: 8 }} container>
            <Grid size={{ sm: 12, xl: 6 }}>
              <GamesTab />
            </Grid>
            <Grid size={{ sm: 12, xl: 6 }}>
              <PlayerHistory />
            </Grid>
          </Grid>
          <Grid size={{ sm: 12, md: 6, xl: 4 }}>
            <Leaderboard />
          </Grid>
        </Grid>
      </Paper>
    </Container>
  );
}
