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
  Container,
  MenuItem,
  FormControlLabel,
  Checkbox,
  Card,
  CardContent,
  CardActions,
  Stack,
} from "@mui/material";
import { SparkLineChart } from "@mui/x-charts/SparkLineChart";
import { lineClasses } from "@mui/x-charts/LineChart";
import PhotoCameraIcon from "@mui/icons-material/PhotoCamera";
import HistoryIcon from "@mui/icons-material/History";
import LoginIcon from "@mui/icons-material/Login";
import {
  createGame,
  joinGame,
  uploadAvatar,
  type CompletionPolicy,
  type PassPolicy,
} from "../api/games";
import {
  selectPlayerId,
  selectProfileIcon,
  selectRating,
  selectSetProfileIcon,
  selectSetRating,
  selectUsername,
  useProfileStore,
} from "../stores/profileStore";
import { useSnackbarStore } from "../stores/snackbarStore";
import Leaderboard from "../components/Leaderboard";
import { getPlayerRating } from "../api/games";
import GameLists from "../components/GameLists";
import JoinGameDialog from "../components/JoinGameDialog";

const Header = () => {
  const navigate = useNavigate();
  const theme = useTheme();
  const profileId = useProfileStore(selectPlayerId);
  const profileIcon = useProfileStore(selectProfileIcon);
  const username = useProfileStore(selectUsername);
  const rating = useProfileStore(selectRating);
  const setRating = useProfileStore(selectSetRating);
  const setProfileIcon = useProfileStore(selectSetProfileIcon);
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));

  useEffect(() => {
    if (profileId) {
      // eslint-disable-next-line react-hooks/immutability
      fetchPlayerRating();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileId]);

  const fetchPlayerRating = async () => {
    if (!profileId) return;

    try {
      setRating(await getPlayerRating(profileId));
    } catch (error) {
      console.error("Failed to fetch player rating:", error);
      showSnackbar("Failed to fetch player rating", "error");
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
      showSnackbar("Please select an image file", "error");
      return;
    }
    // Validate file size (5MB max)
    if (file.size > 5 * 1024 * 1024) {
      showSnackbar("Image size must be less than 5MB", "error");
      return;
    }
    try {
      setIsUploadingAvatar(true);
      const result = await uploadAvatar(profileId, file);
      setProfileIcon(result.profile_icon);
      showSnackbar("Avatar updated successfully", "success");
    } catch (error) {
      console.error("Failed to upload avatar:", error);
      showSnackbar("Failed to upload avatar", "error");
    } finally {
      setIsUploadingAvatar(false);
    }
  };
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "row",
        gap: 2,
        flexGrow: 1,
        py: isMobile ? 1 : 0,
        pl: isMobile ? 2 : 0,
        pr: isMobile ? 1 : 2,
        mb: isMobile ? 0 : 2,
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
      <Box sx={{ flex: "1 1 auto", minWidth: 0 }}>
        <Typography
          variant="h4"
          component="h1"
          noWrap
          sx={{
            fontSize: { xs: "1.5rem", sm: "2.125rem" },
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
        >
          Welcome, {username}!
        </Typography>
        {rating && (
          <Typography variant="body2" color="text.secondary">
            Rating: {rating.rating} • Rank: #{rating.rank || "N/A"}
          </Typography>
        )}
      </Box>
      <IconButton
        color="primary"
        onClick={() => navigate("/history")}
        aria-label="Game history"
        sx={{ ml: "auto", alignSelf: "center" }}
      >
        <HistoryIcon />
      </IconButton>
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
  const [maxGames, setMaxGames] = useState<number>(10);
  const [passPolicy, setPassPolicy] = useState<PassPolicy>("ramsch");
  const [timerEnabled, setTimerEnabled] = useState<boolean>(false);
  const [completionPolicy, setCompletionPolicy] =
    useState<CompletionPolicy>("flexible");
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [isJoinDialogOpen, setIsJoinDialogOpen] = useState<boolean>(false);
  const navigate = useNavigate();
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);

  const handleCreateGame = async () => {
    try {
      setIsLoading(true);
      const createData = await createGame({
        max_games: maxGames,
        pass_policy: passPolicy,
        timer_enabled: timerEnabled,
        completion_policy: maxGames === 0 ? "flexible" : completionPolicy,
      });
      const data = await joinGame(createData.code);
      navigate(`/${data.session_id}`);
    } catch (error) {
      console.error("Error creating game:", error);
      showSnackbar("Failed to create game", "error");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box sx={{ mx: 1 }}>
      <Card variant="outlined" elevation={0}>
        <CardContent>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
              gap: 2,
            }}
          >
            <TextField
              label="Games"
              type="number"
              value={maxGames}
              onChange={(e) =>
                setMaxGames(
                  Math.max(0, Math.min(100, Number(e.target.value) || 0)),
                )
              }
              disabled={isLoading}
              helperText="0 means unlimited"
              slotProps={{ htmlInput: { min: 0, max: 100 } }}
            />
            <TextField
              select
              label="All pass"
              value={passPolicy}
              onChange={(e) => setPassPolicy(e.target.value as PassPolicy)}
              disabled={isLoading}
            >
              <MenuItem value="reshuffle">Re-shuffle</MenuItem>
              <MenuItem value="force_listener">Force forehand</MenuItem>
              <MenuItem value="ramsch">Play Ramsch</MenuItem>
            </TextField>
            <TextField
              select
              label="Tournament commitment"
              value={maxGames === 0 ? "flexible" : completionPolicy}
              onChange={(e) =>
                setCompletionPolicy(e.target.value as CompletionPolicy)
              }
              disabled={isLoading}
            >
              <MenuItem value="flexible">Can end or leave between games</MenuItem>
              <MenuItem value="strict" disabled={maxGames === 0}>
                Play all games; early leave forfeits
              </MenuItem>
            </TextField>
            <FormControlLabel
              sx={{ gridColumn: { xs: "auto", sm: "1 / -1" } }}
              control={
                <Checkbox
                  checked={timerEnabled}
                  onChange={(e) => setTimerEnabled(e.target.checked)}
                  disabled={isLoading}
                />
              }
              label="Two minute move timeout"
            />
          </Box>
        </CardContent>
        <CardActions>
          <Stack direction="column" sx={{ flexGrow: 1 }} spacing={1}>
            <Button
              color="primary"
              startIcon={<LoginIcon />}
              onClick={() => setIsJoinDialogOpen(true)}
              disabled={isLoading}
            >
              Join with Code
            </Button>
            <Button
              sx={{ flexGrow: 1 }}
              variant="contained"
              color="primary"
              onClick={handleCreateGame}
              loading={isLoading}
            >
              Create Game
            </Button>
          </Stack>
        </CardActions>
      </Card>
      <JoinGameDialog
        open={isJoinDialogOpen}
        onClose={() => setIsJoinDialogOpen(false)}
      />
      <GameLists />
    </Box>
  );
};

const LeaderboardTab = () => {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const rating = useProfileStore(selectRating);
  const minY = Math.min(...(rating?.timeline ?? []));
  const maxY = Math.max(...(rating?.timeline ?? []));
  return (
    <Box>
      <SparkLineChart
        sx={{
          [`& .${lineClasses.area}`]: { opacity: 0.2 },
          [`& .${lineClasses.line}`]: { strokeWidth: 3 },
        }}
        area
        color="rgb(224, 101, 255)"
        height={isMobile ? 50 : 100}
        showHighlight
        showTooltip
        data={rating?.timeline || []}
        yAxis={{
          min: minY,
          max: maxY,
        }}
      />
      <Leaderboard />
    </Box>
  );
};

export default function LobbyScreen() {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const [currentTab, setCurrentTab] = useState(0);

  if (isMobile) {
    return (
      <Box
        sx={{
          bgcolor: "background.default",
          height: "100vh",
          overflowY: "auto",
        }}
      >
        <Box sx={{ bgcolor: "background.paper" }}>
          <Header />
          <Tabs
            value={currentTab}
            onChange={(_, newValue) => setCurrentTab(newValue)}
            variant="fullWidth"
            sx={{ mb: 2 }}
          >
            <Tab label="Games" />
            <Tab label="Leaderboard" />
          </Tabs>
        </Box>

        <Box>
          {currentTab === 0 && <GamesTab />}
          {currentTab === 1 && <LeaderboardTab />}
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
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { sm: "1fr", md: "2fr 1fr" },
            gap: 2,
          }}
        >
          <GamesTab />
          <LeaderboardTab />
        </Box>
      </Paper>
    </Container>
  );
}
