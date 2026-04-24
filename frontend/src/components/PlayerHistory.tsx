import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  List,
  ListItem,
  ListItemText,
  CircularProgress,
} from "@mui/material";
import { getPlayerHistory, type PlayerResult } from "../api/games";

interface PlayerHistoryProps {
  playerId: string | null;
}

export default function PlayerHistory({ playerId }: PlayerHistoryProps) {
  const [history, setHistory] = useState<PlayerResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const fetchHistory = async () => {
    if (!playerId) return;

    try {
      setIsLoading(true);
      const data = await getPlayerHistory(playerId);
      setHistory(data);
    } catch (error) {
      console.error("Failed to fetch player history:", error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchHistory();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playerId]);

  return (
    <Box sx={{ minHeight: "200px", display: "flex", flexDirection: "column" }}>
      <Typography variant="subtitle1" gutterBottom>
        Recent Games (Last 50)
      </Typography>

      {isLoading ? (
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
