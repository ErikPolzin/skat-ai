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
import type { CompletionPolicy, PassPolicy, Player } from "../api/games";
import { leaveGame } from "../api/games";

interface SessionResultsProps {
  results: SessionGameResult[];
  playerId?: string;
  gameId?: string;
  gamesPlayed: number;
  maxGames: number;
  players?: (Player | null)[];
  passPolicy?: PassPolicy;
  completionPolicy?: CompletionPolicy;
}

export function SessionResults({
  results,
  playerId,
  gameId,
  gamesPlayed,
  maxGames,
  players,
  passPolicy,
  completionPolicy,
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

  const passPolicyLabel = {
    reshuffle: "All pass: Re-shuffle",
    force_listener: "All pass: Force forehand",
    ramsch: "All pass: Play Ramsch",
  }[passPolicy || "reshuffle"];
  const commitmentLabel =
    completionPolicy === "strict"
      ? "Play all games; early leave forfeits"
      : "Flexible tournament";
  const maxGamesLabel = maxGames === 0 ? "∞" : maxGames;

  if (playerIds.length === 0) {
    return null;
  }

  const totals: { [id: string]: number } = {};
  playerIds.forEach((id) => (totals[id] = 0));

  if (results && results.length > 0) {
    results.forEach((result) => {
      playerIds.forEach((id) => {
        totals[id] += result.player_results[id] || 0;
      });
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
        <Box sx={{ minWidth: 0 }}>
          <Typography
            variant="subtitle1"
            sx={{ color: "white", lineHeight: 1.2 }}
          >
            Session Results ({gamesPlayed}/{maxGamesLabel})
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: "rgba(255, 255, 255, 0.75)",
              display: "block",
              lineHeight: 1.2,
            }}
          >
            {passPolicyLabel} · {commitmentLabel}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
          <IconButton
            onClick={handleLeaveSession}
            loading={isLeaving}
            size="small"
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

      {/* Player Scores Table */}
      <TableContainer sx={{ flexGrow: 1, minHeight: 0, overflowY: "auto" }}>
        <Table
          size="small"
          stickyHeader
          sx={{ height: "100%", tableLayout: "fixed" }}
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
            {results.length > 0 ? (
              <>
                {results.map((result, index) => (
                  <TableRow
                    key={result.game_number || index}
                    hover
                    sx={{
                      "&:hover": { bgcolor: "rgba(255, 255, 255, 0.05)" },
                    }}
                  >
                    <TableCell
                      sx={{
                        color: "rgba(255, 255, 255, 0.7)",
                        borderBottom: "1px solid rgba(255,255,255,0.05)",
                      }}
                    >
                      {index + 1}
                    </TableCell>
                    {playerIds.map((id) => {
                      const points = result.player_results[id] || 0;
                      return (
                        <TableCell
                          key={id}
                          align="center"
                          sx={{
                            color:
                              points > 0
                                ? "#4caf50"
                                : points < 0
                                  ? "#f44336"
                                  : "rgba(255, 255, 255, 0.7)",
                            fontWeight: id === playerId ? "bold" : "normal",
                            borderBottom: "1px solid rgba(255,255,255,0.05)",
                          }}
                        >
                          {points > 0 && "+"}
                          {points}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))}
                <TableRow sx={{ height: "100%" }}>
                  <TableCell
                    colSpan={playerIds.length + 1}
                    sx={{ borderBottom: 0, p: 0 }}
                  />
                </TableRow>
                <TableRow
                  sx={{
                    bgcolor: "rgba(255, 255, 255, 0.04)",
                    "&:last-child td": { borderBottom: 0 },
                    ...(!isMobile && {
                      "& > td": {
                        position: "sticky",
                        bottom: 0,
                        zIndex: 1,
                        bgcolor: theme.palette.background.paper,
                      },
                    }),
                  }}
                >
                  <TableCell
                    sx={{
                      color: "rgba(255, 255, 255, 0.9)",
                      fontWeight: "bold",
                      borderTop: "2px solid rgba(255,255,255,0.12)",
                    }}
                  >
                    Total
                  </TableCell>
                  {playerIds.map((id) => {
                    const total = totals[id];
                    return (
                      <TableCell
                        key={id}
                        align="center"
                        sx={{
                          color:
                            total > 0
                              ? "#4caf50"
                              : total < 0
                                ? "#f44336"
                                : "rgba(255, 255, 255, 0.7)",
                          fontWeight: "bold",
                          borderTop: "2px solid rgba(255,255,255,0.12)",
                        }}
                      >
                        {total > 0 && "+"}
                        {total}
                      </TableCell>
                    );
                  })}
                </TableRow>
              </>
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
