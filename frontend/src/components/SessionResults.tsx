import React from "react";
import {
  Paper,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material";
import type { SessionGameResult } from "../types";
import type { Player } from "../api/games";

interface SessionResultsProps {
  results: SessionGameResult[];
  playerId?: string;
  gamesPlayed: number;
  maxGames: number;
  players?: (Player | null)[];
}

export function SessionResults({
  results,
  playerId,
  gamesPlayed,
  maxGames,
  players,
}: SessionResultsProps) {
  // Get player IDs from results or from players prop
  let playerIds: string[] = [];
  let playerNames: { [id: string]: string } = {};

  if (results && results.length > 0) {
    // Get from results
    playerIds =
      results[0] && results[0].player_results
        ? Object.keys(results[0].player_results)
        : [];
    playerNames = results[0]?.player_names || {};
  } else if (players) {
    // Get from players prop when no results yet
    playerIds = players.filter((p): p is Player => p !== null).map((p) => p.id);
    playerNames = Object.fromEntries(
      players.filter((p): p is Player => p !== null).map((p) => [p.id, p.name]),
    );
  }

  if (playerIds.length === 0) {
    return null;
  }

  // Calculate cumulative scores for each game
  const cumulativeData: Array<{ [id: string]: number }> = [];
  const runningTotals: { [id: string]: number } = {};
  playerIds.forEach((id) => (runningTotals[id] = 0));

  if (results && results.length > 0) {
    results.forEach((result, index) => {
      playerIds.forEach((id) => {
        runningTotals[id] += result.player_results[id] || 0;
      });
      cumulativeData.push({ ...runningTotals });
    });
  }

  return (
    <Paper
      elevation={3}
      sx={{
        my: "12px",
        marginRight: "8px",
        height: "calc(100% - 24px)",
        display: "flex",
        flexDirection: "column",
        borderRadius: "20px",
        bgcolor: "white",
      }}
    >
      <Typography variant="h6" gutterBottom sx={{ my: 1, mx: 3 }}>
        Session Results ({gamesPlayed}/{maxGames})
      </Typography>

      {/* Player Cumulative Scores Table */}
      <TableContainer sx={{ flexGrow: 1, overflowY: "auto" }}>
        <Table size="small" stickyHeader>
          <TableHead>
            <TableRow>
              <TableCell
                sx={{
                  bgcolor: "white",
                  fontWeight: "bold",
                  borderBottom: "2px solid rgba(0,0,0,0.1)",
                }}
              >
                #
              </TableCell>
              {playerIds.map((id) => (
                <TableCell
                  key={id}
                  align="center"
                  sx={{
                    bgcolor: "white",
                    color: id === playerId ? "#764ba2" : "text.primary",
                    fontWeight: "bold",
                    borderBottom: "2px solid rgba(0,0,0,0.1)",
                  }}
                >
                  {playerNames[id] || id.substring(0, 8)}
                  {id === playerId && " (You)"}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {cumulativeData.length > 0 ? (
              cumulativeData.map((scores, index) => (
                <TableRow key={index} hover>
                  <TableCell
                    sx={{ borderBottom: "1px solid rgba(0,0,0,0.05)" }}
                  >
                    {index + 1}
                  </TableCell>
                  {playerIds.map((id) => (
                    <TableCell
                      key={id}
                      align="center"
                      sx={{
                        color:
                          scores[id] > 0
                            ? "success.main"
                            : scores[id] < 0
                              ? "error.main"
                              : "text.primary",
                        fontWeight: id === playerId ? "bold" : "normal",
                        borderBottom: "1px solid rgba(0,0,0,0.05)",
                      }}
                    >
                      {scores[id] > 0 && "+"}
                      {scores[id]}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={playerIds.length + 1}
                  align="center"
                  sx={{ color: "text.secondary", py: 4 }}
                >
                  No games completed yet
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Paper>
  );
}
