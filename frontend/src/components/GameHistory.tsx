import React, { useEffect, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  FormControlLabel,
  Checkbox,
  Chip,
  Paper,
} from "@mui/material";
import { getPlayerGameHistory, type GameHistoryEntry } from "../api/games";

interface GameHistoryProps {
  playerId: string | null;
}

export function GameHistory({ playerId }: GameHistoryProps) {
  const [history, setHistory] = useState<GameHistoryEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAIGames, setShowAIGames] = useState(true);

  useEffect(() => {
    if (!playerId) {
      setHistory([]);
      return;
    }

    const fetchHistory = async () => {
      setLoading(true);
      try {
        const data = await getPlayerGameHistory(playerId, 20);
        setHistory(data || []);  // Ensure we always set an array, never null
      } catch (error) {
        console.error("Failed to load game history:", error);
        setHistory([]);
      } finally {
        setLoading(false);
      }
    };

    fetchHistory();
  }, [playerId]);

  // Filter history based on AI games toggle
  const filteredHistory = showAIGames
    ? history
    : history?.filter(entry => !entry.vs_ai) || [];

  if (!playerId) {
    return null;
  }

  if (loading) {
    return (
      <Box sx={{ textAlign: "center", py: 2 }}>
        <CircularProgress size={30} />
      </Box>
    );
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? "s" : ""} ago`;
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;

    return date.toLocaleDateString();
  };

  const formatOpponents = (names: string[]) => {
    if (!names || names.length === 0) return "Unknown";
    if (names.length === 1) return names[0];
    return names.join(" & ");
  };

  if (!history || history.length === 0) {
    return (
      <Typography color="text.secondary" align="center" sx={{ py: 2 }}>
        No games played yet
      </Typography>
    );
  }

  return (
    <Box>
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 2 }}>
        <FormControlLabel
          control={
            <Checkbox
              checked={showAIGames}
              onChange={(e) => setShowAIGames(e.target.checked)}
            />
          }
          label="Show AI Games"
        />
      </Box>

      {filteredHistory.length === 0 ? (
        <Typography color="text.secondary" align="center" sx={{ py: 2 }}>
          No {!showAIGames ? "human" : ""} games found
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {filteredHistory.map((entry, index) => (
            <Paper
              key={`${entry.game_id}-${index}`}
              sx={{
                p: 2,
                border: 2,
                borderColor: entry.is_winner ? "success.light" : "error.light",
                backgroundColor: entry.is_winner
                  ? "rgba(76, 175, 80, 0.05)"
                  : "rgba(244, 67, 54, 0.05)",
              }}
            >
              <Box sx={{ display: "flex", justifyContent: "space-between", mb: 1 }}>
                <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                  <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
                    {entry.game_code || entry.game_id.slice(0, 8)}
                  </Typography>
                  {entry.vs_ai && (
                    <Chip label="vs AI" size="small" color="info" />
                  )}
                </Box>
                <Typography variant="caption" color="text.secondary">
                  {formatDate(entry.finished_at)}
                </Typography>
              </Box>

              <Typography variant="body2" sx={{ mb: 1 }}>
                vs {formatOpponents(entry.opponent_names || [])}
              </Typography>

              <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <Box sx={{ display: "flex", gap: 2 }}>
                  <Typography variant="caption" color="text.secondary">
                    {entry.game_mode}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {entry.is_declarer ? "Declarer" : "Defender"}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Score: {entry.final_score}
                  </Typography>
                </Box>
                <Chip
                  label={entry.is_winner ? "Victory" : "Defeat"}
                  color={entry.is_winner ? "success" : "error"}
                  size="small"
                />
              </Box>
            </Paper>
          ))}
        </Box>
      )}
    </Box>
  );
}