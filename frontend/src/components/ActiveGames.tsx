import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Typography,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Button,
  IconButton,
  CircularProgress,
  Chip,
  Skeleton,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import CloseIcon from "@mui/icons-material/Close";
import { getActiveGames, leaveGame, type ActiveGame } from "../api/games";
import { selectPlayerId, useProfileStore } from "../stores/profileStore";
import { useSnackbarStore } from "../stores/snackbarStore";

export default function ActiveGames() {
  const navigate = useNavigate();
  const [games, setGames] = useState<ActiveGame[]>([]);
  const [isFetching, setIsFetching] = useState(false);
  const [leavingGameId, setLeavingGameId] = useState<string | null>(null);
  const profileId = useProfileStore(selectPlayerId);
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);

  const fetchActiveGames = async () => {
    if (!profileId) return;

    try {
      setIsFetching(true);
      const data = await getActiveGames(profileId);
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch active games:", error);
      showSnackbar("Failed to fetch active games", "error");
    } finally {
      setIsFetching(false);
    }
  };

  useEffect(() => {
    fetchActiveGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileId]);

  const handleRejoinGame = (gameId: string) => {
    navigate(`/game/${gameId}`);
  };

  const handleLeaveGame = async (gameId: string, event: React.MouseEvent) => {
    event.stopPropagation(); // Prevent triggering rejoin
    if (!profileId) return;

    try {
      setLeavingGameId(gameId);
      await leaveGame(gameId, profileId);
      // Refresh the list after leaving
      await fetchActiveGames();
      showSnackbar("Successfully left game", "success");
    } catch (error) {
      console.error("Failed to leave game:", error);
      showSnackbar("Failed to leave game", "error");
    } finally {
      setLeavingGameId(null);
    }
  };

  const getPhaseLabel = (phase: string): string => {
    const phaseMap: { [key: string]: string } = {
      waiting_for_players: "Waiting",
      dealing: "Ready to Deal",
      bidding: "Bidding",
      choosing_game: "Choosing Game",
      skat_decision: "Skat Decision",
      discard_cards: "Discarding",
      playing: "In Progress",
      complete: "Complete",
    };
    return phaseMap[phase] || phase;
  };

  const getPhaseColor = (
    phase: string,
  ): "default" | "primary" | "secondary" | "success" | "warning" => {
    if (phase === "waiting_for_players") return "warning";
    if (phase === "playing") return "success";
    if (phase === "complete") return "default";
    return "primary";
  };

  if (!profileId || (!isFetching && games.length === 0)) {
    return null;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", mt: 1 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle1">Your Active Games</Typography>
        <IconButton
          onClick={fetchActiveGames}
          color="primary"
          disabled={isFetching}
          size="small"
        >
          <RefreshIcon />
        </IconButton>
      </Box>

      <List>
        {isFetching && games.length === 0
          ? // Skeleton loader for initial load
            Array.from({ length: 2 }).map((_, index) => (
              <ListItem
                key={`skeleton-${index}`}
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
                      <Skeleton variant="text" width={80} height={24} />
                      <Skeleton variant="rounded" width={60} height={24} />
                    </Box>
                  }
                  secondary={
                    <Skeleton variant="text" width={150} height={20} />
                  }
                />
                <ListItemSecondaryAction>
                  <Box sx={{ display: "flex", gap: 1 }}>
                    <Skeleton variant="rounded" width={70} height={32} />
                    <Skeleton variant="circular" width={32} height={32} />
                  </Box>
                </ListItemSecondaryAction>
              </ListItem>
            ))
          : games.map((game) => (
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
                        flexWrap: "wrap",
                      }}
                    >
                      <Typography
                        variant="subtitle1"
                        sx={{ fontWeight: "bold" }}
                      >
                        {game.code}
                      </Typography>
                      <Chip
                        label={getPhaseLabel(game.phase)}
                        size="small"
                        color={getPhaseColor(game.phase)}
                      />
                      {game.game_number > 1 && (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ ml: "auto" }}
                        >
                          Game #{game.game_number}
                        </Typography>
                      )}
                    </Box>
                  }
                  secondary={
                    <Typography variant="body2" color="text.secondary">
                      {game.player_names.join(", ")}
                    </Typography>
                  }
                />
                <ListItemSecondaryAction>
                  <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                    <Button
                      variant="contained"
                      onClick={() => handleRejoinGame(game.id)}
                      size="small"
                    >
                      Rejoin
                    </Button>
                    <IconButton
                      onClick={(e) => handleLeaveGame(game.id, e)}
                      disabled={leavingGameId === game.id}
                      color="error"
                      size="small"
                    >
                      {leavingGameId === game.id ? (
                        <CircularProgress size={20} color="error" />
                      ) : (
                        <CloseIcon fontSize="small" />
                      )}
                    </IconButton>
                  </Box>
                </ListItemSecondaryAction>
              </ListItem>
            ))}
      </List>
    </Box>
  );
}
