import { useState } from "react";
import {
  Paper,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  IconButton,
  Box,
  useMediaQuery,
  useTheme,
  Backdrop,
} from "@mui/material";
import { ExpandMore, ExpandLess, ExitToApp } from "@mui/icons-material";
import { useNavigate } from "react-router-dom";
import type { SessionGameResult } from "../types";
import type { Player } from "../api/games";
import { leaveGame } from "../api/games";

interface SessionResultsProps {
  results: SessionGameResult[];
  playerId?: string;
  gameId?: string;
  gamesPlayed: number;
  maxGames: number;
  players?: (Player | null)[];
}

export function SessionResults({
  results,
  playerId,
  gameId,
  gamesPlayed,
  maxGames,
  players,
}: SessionResultsProps) {
  const theme = useTheme();
  const navigate = useNavigate();
  const isMobile = useMediaQuery(theme.breakpoints.down("lg"));
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [isLeaving, setIsLeaving] = useState(false);

  const handleLeaveSession = async () => {
    if (!gameId) return;

    try {
      setIsLeaving(true);
      await leaveGame(gameId);
      navigate("/");
    } catch (error) {
      console.error("Failed to leave session:", error);
      setIsLeaving(false);
    }
  };

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
    results.forEach((result) => {
      playerIds.forEach((id) => {
        runningTotals[id] += result.player_results[id] || 0;
      });
      cumulativeData.push({ ...runningTotals });
    });
  }

  const resultsContent = (
    <>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          px: 3,
          minHeight: "62px",
          bgcolor: theme.palette.primary.main,
        }}
      >
        <Typography variant="subtitle1" sx={{ color: "white" }}>
          Session Results ({gamesPlayed}/{maxGames})
        </Typography>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <IconButton
            onClick={handleLeaveSession}
            loading={isLeaving}
            size="small"
            sx={{ color: "rgba(255, 255, 255, 0.7)" }}
          >
            <ExitToApp />
          </IconButton>
          {isMobile && (
            <IconButton
              onClick={(e) => {
                e.stopPropagation();
                setDrawerOpen(!drawerOpen);
              }}
              size="small"
              sx={{ color: "rgba(255, 255, 255, 0.7)" }}
            >
              {drawerOpen ? <ExpandLess /> : <ExpandMore />}
            </IconButton>
          )}
        </Box>
      </Box>

      {/* Player Cumulative Scores Table */}
      <TableContainer sx={{ flexGrow: 1, overflowY: "auto" }}>
        <Table
          size="small"
          stickyHeader
          sx={{ height: cumulativeData.length === 0 ? "100%" : "auto" }}
        >
          <TableHead>
            <TableRow>
              <TableCell
                sx={{
                  color: "rgba(255, 255, 255, 0.9)",
                  fontWeight: "bold",
                  borderBottom: "2px solid rgba(255,255,255,0.1)",
                }}
              >
                #
              </TableCell>
              {playerIds.map((id) => (
                <TableCell
                  key={id}
                  align="center"
                  sx={{
                    color:
                      id === playerId ? "#bb86fc" : "rgba(255, 255, 255, 0.9)",
                    fontWeight: "bold",
                    borderBottom: "2px solid rgba(255,255,255,0.1)",
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
                <TableRow
                  key={index}
                  hover
                  sx={{ "&:hover": { bgcolor: "rgba(255, 255, 255, 0.05)" } }}
                >
                  <TableCell
                    sx={{
                      color: "rgba(255, 255, 255, 0.7)",
                      borderBottom: "1px solid rgba(255,255,255,0.05)",
                    }}
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
                            ? "#4caf50"
                            : scores[id] < 0
                              ? "#f44336"
                              : "rgba(255, 255, 255, 0.7)",
                        fontWeight: id === playerId ? "bold" : "normal",
                        borderBottom: "1px solid rgba(255,255,255,0.05)",
                      }}
                    >
                      {scores[id] > 0 && "+"}
                      {scores[id]}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow sx={{ height: "100%" }}>
                <TableCell
                  colSpan={playerIds.length + 1}
                  align="center"
                  sx={{
                    color: "rgba(255, 255, 255, 0.5)",
                    py: 4,
                    height: "100%",
                    verticalAlign: "middle",
                  }}
                >
                  No games completed yet
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  );

  if (isMobile) {
    return (
      <>
        <Backdrop
          open={drawerOpen}
          onClick={() => setDrawerOpen(false)}
          sx={{
            zIndex: 999,
            bgcolor: "rgba(0, 0, 0, 0.5)",
          }}
        />
        <Paper
          elevation={3}
          onClick={() => !drawerOpen && setDrawerOpen(true)}
          sx={{
            position: "fixed",
            top: 0,
            left: 0,
            right: 0,
            height: drawerOpen ? "80vh" : "64px",
            display: "flex",
            flexDirection: "column",
            borderRadius: 0,
            borderBottomLeftRadius: drawerOpen ? "20px" : 0,
            borderBottomRightRadius: drawerOpen ? "20px" : 0,
            zIndex: 1000,
            transition: "height 0.3s ease, border-radius 0.3s ease",
            cursor: !drawerOpen ? "pointer" : "default",
            overflow: "hidden",
          }}
        >
          {resultsContent}
        </Paper>
      </>
    );
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
        overflow: "hidden",
      }}
    >
      {resultsContent}
    </Paper>
  );
}
