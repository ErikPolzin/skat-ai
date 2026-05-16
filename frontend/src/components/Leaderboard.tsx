import { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Avatar,
  Skeleton,
} from "@mui/material";
import { getLeaderboard, type LeaderboardEntry } from "../api/games";

export default function Leaderboard() {
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const fetchLeaderboard = async () => {
    try {
      setIsLoading(true);
      const data = await getLeaderboard(100);
      setLeaderboard(data);
    } catch (error) {
      console.error("Failed to fetch leaderboard:", error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchLeaderboard();
  }, []);

  const getRatingColor = (rating: number) => {
    if (rating >= 2000) return "#FFD700"; // Gold
    if (rating >= 1800) return "#C0C0C0"; // Silver
    if (rating >= 1600) return "#CD7F32"; // Bronze
    return "#90A4AE"; // Gray
  };

  if (isLoading) {
    return (
      <TableContainer
        component={Paper}
        sx={{
          borderRadius: { xs: 0, sm: 1 },
        }}
      >
        <Table stickyHeader size="small">
          <TableHead>
            <TableRow>
              <TableCell>Rank</TableCell>
              <TableCell>Player</TableCell>
              <TableCell align="right">Rating</TableCell>
              <TableCell align="right">Games</TableCell>
              <TableCell align="right">Win Rate</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {Array.from({ length: 10 }).map((_, index) => (
              <TableRow key={`skeleton-${index}`}>
                <TableCell>
                  <Skeleton variant="text" width={40} height={32} />
                </TableCell>
                <TableCell>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Skeleton variant="circular" width={32} height={32} />
                    <Box>
                      <Skeleton variant="text" width={100} height={20} />
                    </Box>
                  </Box>
                </TableCell>
                <TableCell align="right">
                  <Skeleton variant="text" width={50} height={24} />
                </TableCell>
                <TableCell align="right">
                  <Skeleton variant="text" width={30} height={20} />
                </TableCell>
                <TableCell align="right">
                  <Skeleton variant="text" width={50} height={20} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    );
  }

  if (leaderboard.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 4 }}>
        <Typography color="text.secondary">
          No players ranked yet. Play some games to get on the leaderboard!
        </Typography>
      </Box>
    );
  }

  return (
    <TableContainer
      component={Paper}
      sx={{
        borderRadius: { xs: 0, sm: 1 },
      }}
    >
      <Table stickyHeader size="small">
        <TableHead>
          <TableRow>
            <TableCell>Rank</TableCell>
            <TableCell>Player</TableCell>
            <TableCell align="right">Rating</TableCell>
            <TableCell align="right">Games</TableCell>
            <TableCell align="right">Win Rate</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {leaderboard.map((entry) => (
            <TableRow
              key={entry.profile_id}
              sx={{
                "&:hover": { bgcolor: "action.hover" },
                bgcolor: entry.rank <= 3 ? "action.selected" : "transparent",
              }}
            >
              <TableCell>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Typography
                    sx={{
                      color:
                        entry.rank === 1
                          ? "#FFD700"
                          : entry.rank === 2
                            ? "#C0C0C0"
                            : entry.rank === 3
                              ? "#CD7F32"
                              : "text.primary",
                      fontWeight: entry.rank <= 3 ? "bold" : "normal",
                    }}
                  >
                    #{entry.rank}
                  </Typography>
                </Box>
              </TableCell>
              <TableCell>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Avatar
                    src={entry.profile_icon || undefined}
                    alt={entry.name}
                    sx={{ width: 32, height: 32 }}
                  >
                    {entry.name.charAt(0).toUpperCase()}
                  </Avatar>
                  <Typography variant="body2">{entry.name}</Typography>
                </Box>
              </TableCell>
              <TableCell align="right">
                <Typography
                  variant="body1"
                  sx={{
                    fontWeight: "bold",
                    color: getRatingColor(entry.rating),
                  }}
                >
                  {entry.rating}
                </Typography>
              </TableCell>
              <TableCell align="right">{entry.games_played}</TableCell>
              <TableCell align="right">
                <Typography
                  variant="body2"
                  sx={{
                    color:
                      entry.win_rate >= 60
                        ? "#4caf50"
                        : entry.win_rate >= 40
                          ? "#ff9800"
                          : "#f44336",
                  }}
                >
                  {entry.win_rate.toFixed(1)}%
                </Typography>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
