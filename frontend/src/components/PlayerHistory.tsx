import { useState, useEffect } from "react";
import {
  Box,
  Typography,
  List,
  ListItem,
  ListItemText,
  IconButton,
  Skeleton,
} from "@mui/material";
import { getPlayerHistory, type PlayerResult } from "../api/games";
import { selectPlayerId, useProfileStore } from "../stores/profileStore";
import RefreshIcon from "@mui/icons-material/Refresh";

export default function PlayerHistory() {
  const [history, setHistory] = useState<PlayerResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const profileId = useProfileStore(selectPlayerId);

  const fetchHistory = async () => {
    if (!profileId) return;

    try {
      setIsLoading(true);
      const data = await getPlayerHistory(profileId);
      setHistory(data);
    } catch (error) {
      console.error("Failed to fetch player history:", error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchHistory();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileId]);

  return (
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
        <Typography variant="subtitle1">Game History</Typography>
        <IconButton onClick={fetchHistory} color="primary" disabled={isLoading}>
          <RefreshIcon />
        </IconButton>
      </Box>
      {isLoading && history.length === 0 ? (
        <List dense>
          {Array.from({ length: 5 }).map((_, index) => (
            <ListItem key={`skeleton-${index}`}>
              <ListItemText
                primary={
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Skeleton variant="text" width={30} height={20} />
                    <Skeleton variant="text" width={50} height={20} />
                    <Skeleton variant="text" width={60} height={20} />
                    <Skeleton variant="text" width={70} height={20} />
                  </Box>
                }
                secondary={<Skeleton variant="text" width={120} height={16} />}
              />
            </ListItem>
          ))}
        </List>
      ) : history.length === 0 ? (
        <Box
          sx={{
            flexGrow: 1,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          <Typography sx={{ py: 2 }} color="textDisabled">
            No game history
          </Typography>
        </Box>
      ) : (
        <List dense>
          {history.map((result, index) => (
            <ListItem key={result.game_id}>
              <ListItemText
                primary={
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                    }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      #{index + 1}
                    </Typography>
                    <Typography
                      variant="body2"
                      sx={{
                        fontWeight: "bold",
                        color: result.is_winner ? "#4caf50" : "#f44336",
                      }}
                    >
                      {result.is_winner ? "WIN" : "LOSS"}
                    </Typography>
                    <Typography
                      variant="body2"
                      sx={{
                        fontWeight: "bold",
                        color:
                          result.player_points > 0
                            ? "#4caf50"
                            : result.player_points < 0
                              ? "#f44336"
                              : "text.primary",
                      }}
                    >
                      {result.player_points > 0 && "+"}
                      {result.player_points} pts
                    </Typography>
                    {result.rating_change !== undefined &&
                      result.rating_change !== 0 && (
                        <Typography
                          variant="body2"
                          sx={{
                            fontWeight: "medium",
                            color:
                              result.rating_change > 0 ? "#2196f3" : "#ff5722",
                          }}
                        >
                          {result.rating_change > 0 && "+"}
                          {result.rating_change} ELO
                        </Typography>
                      )}
                  </Box>
                }
                secondary={
                  result.other_players && result.other_players.length > 0
                    ? `vs ${result.other_players.join(", ")}`
                    : undefined
                }
              />
            </ListItem>
          ))}
        </List>
      )}
    </Box>
  );
}
