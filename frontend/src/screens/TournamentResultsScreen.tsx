import {
  Avatar,
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getSessionResults, type SessionPlayerResult } from "../api/games";
import { GameProvider, useGameContext } from "../context/GameContext";

function TournamentResultsScreenContent() {
  const game = useGameContext();
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const [sessionPlayerResults, setSessionPlayerResults] = useState<
    SessionPlayerResult[]
  >(game.sessionPlayerResults);
  const hasFinalRatings = sessionPlayerResults.length > 0;

  useEffect(() => {
    let cancelled = false;

    const refreshResults = async () => {
      if (!game.sessionId || sessionPlayerResults.length > 0) {
        return;
      }
      for (let attempt = 0; attempt < 6 && !cancelled; attempt++) {
        const sessionData = await getSessionResults(game.sessionId);
        if (cancelled) return;
        if (
          sessionData.player_results &&
          sessionData.player_results.length > 0
        ) {
          setSessionPlayerResults(sessionData.player_results);
          game.setSessionPlayerResults(sessionData.player_results);
          if (sessionData.results?.length) {
            game.setSessionResults(sessionData.results);
            game.setGamesPlayed(sessionData.results.length);
          }
          return;
        }
        await new Promise((resolve) => setTimeout(resolve, 800));
      }
    };

    refreshResults().catch((error) => {
      console.error("Failed to refresh tournament results:", error);
    });

    return () => {
      cancelled = true;
    };
  }, [game, sessionPlayerResults.length]);

  const playerNames = useMemo(
    () =>
      game.sessionResults[0]?.player_names ||
      Object.fromEntries(
        game.players
          .filter((player) => player !== null)
          .map((player) => [player.id, player.name]),
      ),
    [game.players, game.sessionResults],
  );
  const standings = useMemo(() => {
    const playerIds = Object.keys(playerNames);
    const completedTotals = playerIds.reduce<Record<string, number>>(
      (totals, id) => {
        totals[id] = game.sessionResults.reduce(
          (sum, sessionGame) => sum + (sessionGame.player_results[id] || 0),
          0,
        );
        return totals;
      },
      {},
    );
    const finalResultsByPlayer = Object.fromEntries(
      sessionPlayerResults.map((playerResult) => [
        playerResult.player_id,
        playerResult,
      ]),
    );

    return playerIds
      .map((id) => ({
        id,
        name: playerNames[id],
        profileIcon:
          game.players.find((player) => player?.id === id)?.profile_icon || "",
        total:
          finalResultsByPlayer[id]?.player_points ?? completedTotals[id] ?? 0,
        ratingBefore: finalResultsByPlayer[id]?.rating_before,
        ratingAfter: finalResultsByPlayer[id]?.rating_after,
        ratingChange: finalResultsByPlayer[id]?.rating_change ?? 0,
        isWinner: finalResultsByPlayer[id]?.is_winner ?? false,
        isForfeit: finalResultsByPlayer[id]?.is_forfeit ?? false,
      }))
      .sort((a, b) => {
        if (a.isForfeit !== b.isForfeit) return a.isForfeit ? 1 : -1;
        if (b.total !== a.total) return b.total - a.total;
        return b.ratingChange - a.ratingChange;
      });
  }, [game.players, game.sessionResults, playerNames, sessionPlayerResults]);
  const featuredStandings = isMobile
    ? standings
    : [standings[1], standings[0], standings[2]].filter(
        (standing): standing is (typeof standings)[number] => Boolean(standing),
      );
  const rankByPlayer = new Map(
    standings.map((standing, index) => [standing.id, index + 1]),
  );
  const ratingColor = (ratingChange: number) =>
    ratingChange > 0
      ? "success.main"
      : ratingChange < 0
        ? "error.main"
        : "text.secondary";
  const rankBadgeColor = (rank: number) => {
    switch (rank) {
      case 1:
        return "#fdd835";
      case 2:
        return "#cfd8dc";
      case 3:
        return "#d7a36a";
      default:
        return "#607d8b";
    }
  };

  return (
    <Dialog open fullScreen>
      <DialogContent
        sx={{
          bgcolor: "background.default",
          display: "flex",
          flexDirection: "column",
          overflow: "auto",
          px: { xs: 1, sm: 2 },
          pb: { xs: 1, sm: 0 },
          pt: { xs: 1, sm: 2 },
        }}
      >
        <Stack
          direction={{ xs: "column", sm: "row" }}
          aria-label="Final standings"
          spacing={{ xs: 1, sm: 2, md: 3 }}
          sx={{
            justifyContent: "center",
            alignItems: { xs: "stretch", sm: "center" },
            mx: "auto",
            width: "100%",
            maxWidth: 1040,
            mt: { xs: 0, sm: 2.5 },
          }}
        >
          {featuredStandings.map((standing) => {
            const rank = rankByPlayer.get(standing.id) ?? 0;
            const isWinner = rank === 1;
            const isCurrentPlayer = standing.id === game.player?.id;
            return (
              <Card
                key={standing.id}
                elevation={isWinner ? 8 : 2}
                variant="outlined"
                sx={{
                  flex: {
                    xs: "0 0 auto",
                    sm: isWinner ? "1.2 1 0" : "0.82 1 0",
                  },
                  minWidth: { xs: "100%", sm: 0 },
                  opacity: isWinner ? 1 : 0.9,
                  position: "relative",
                }}
              >
                <Box
                  sx={{
                    alignItems: "center",
                    bgcolor: rankBadgeColor(rank),
                    borderRadius: "50%",
                    color: "#111827",
                    display: "flex",
                    fontSize: isWinner ? { xs: 14, sm: 18 } : 13,
                    fontWeight: 900,
                    height: isWinner ? { xs: 30, sm: 38 } : 28,
                    justifyContent: "center",
                    position: "absolute",
                    right: { xs: 10, sm: 12 },
                    top: { xs: 10, sm: 12 },
                    width: isWinner ? { xs: 30, sm: 38 } : 28,
                    zIndex: 1,
                  }}
                >
                  {rank}
                </Box>
                <Stack
                  direction={{ xs: "row", sm: "column" }}
                  sx={{
                    alignItems: "center",
                    gap: isWinner ? { xs: 1.5, sm: 1.5 } : 1,
                    justifyContent: { xs: "space-between", sm: "center" },
                    minHeight: { xs: "auto", sm: isWinner ? 300 : 230 },
                    p: isWinner ? { xs: 1.25, sm: 3 } : { xs: 1, sm: 2 },
                    position: "relative",
                    textAlign: { xs: "left", sm: "center" },
                  }}
                >
                  <Box
                    sx={{
                      position: "relative",
                      flexShrink: 0,
                    }}
                  >
                    <Avatar
                      src={standing.profileIcon}
                      alt={standing.name}
                      sx={{
                        width: isWinner
                          ? { xs: 64, sm: 140 }
                          : { xs: 48, sm: 76 },
                        height: isWinner
                          ? { xs: 64, sm: 140 }
                          : { xs: 48, sm: 76 },
                        fontSize: isWinner
                          ? { xs: 28, sm: 56 }
                          : { xs: 20, sm: 30 },
                      }}
                    >
                      {standing.name.charAt(0).toUpperCase()}
                    </Avatar>
                  </Box>
                  <Stack
                    spacing={{ xs: 0.25, sm: 1 }}
                    sx={{ flex: 1, minWidth: 0, alignItems: { sm: "center" } }}
                  >
                    <Typography
                      variant={isWinner ? "h6" : "body1"}
                      sx={{ fontWeight: 700 }}
                      noWrap
                    >
                      {standing.name}
                      {isCurrentPlayer ? " (You)" : ""}
                    </Typography>
                    <Typography color="text.secondary" noWrap>
                      {standing.total > 0 && "+"}
                      {standing.total} pts
                      {hasFinalRatings && standing.ratingAfter
                        ? ` · ${standing.ratingAfter}`
                        : ""}
                    </Typography>
                  </Stack>
                  <Typography
                    variant={isWinner ? (isMobile ? "h3" : "h2") : "h4"}
                    sx={{
                      color: ratingColor(standing.ratingChange),
                      fontWeight: 900,
                      lineHeight: 1,
                      minWidth: { xs: isWinner ? 112 : 92, sm: "auto" },
                      pr: { xs: 3.5, sm: 0 },
                      textAlign: { xs: "right", sm: "center" },
                    }}
                  >
                    {standing.ratingChange > 0 && "+"}
                    {standing.ratingChange}
                    <Typography
                      component="span"
                      sx={{
                        color: "inherit",
                        fontSize: isWinner
                          ? { xs: 18, sm: 34 }
                          : { xs: 14, sm: 22 },
                        fontWeight: 700,
                        ml: 0.75,
                      }}
                    >
                      Elo
                    </Typography>
                  </Typography>
                </Stack>
              </Card>
            );
          })}
        </Stack>
        <TableContainer
          component={Paper}
          sx={{
            flexGrow: { xs: 0, sm: 1 },
            flexShrink: 0,
            my: { xs: 1, sm: 2 },
            mx: "auto",
            width: "100%",
            maxWidth: 1100,
            overflowX: "auto",
            minHeight: { xs: "auto", sm: 0 },
          }}
        >
          <Table size="small" sx={{ minWidth: { xs: 560, sm: 0 } }}>
            <TableHead>
              <TableRow>
                <TableCell
                  sx={{
                    position: { xs: "sticky", sm: "static" },
                    left: 0,
                    bgcolor: "background.paper",
                    zIndex: 1,
                  }}
                >
                  Game
                </TableCell>
                <TableCell sx={{ minWidth: 160 }}>Contract</TableCell>
                {standings.map((standing) => {
                  return (
                    <TableCell
                      key={standing.id}
                      align="right"
                      sx={{ minWidth: 88 }}
                    >
                      {standing.name}
                    </TableCell>
                  );
                })}
              </TableRow>
            </TableHead>
            <TableBody>
              {game.sessionResults.map((sessionGame, index) => (
                <TableRow key={sessionGame.game_number}>
                  <TableCell
                    sx={{
                      position: { xs: "sticky", sm: "static" },
                      left: 0,
                      bgcolor: "background.paper",
                      zIndex: 1,
                    }}
                  >
                    {index + 1}
                  </TableCell>
                  <TableCell>
                    {sessionGame.game_mode === "ramsch"
                      ? "Ramsch"
                      : `${sessionGame.declarer_name} ${sessionGame.declarer_won ? "won" : "lost"}`}
                  </TableCell>
                  {standings.map((standing) => {
                    const points = sessionGame.player_results[standing.id] || 0;
                    return (
                      <TableCell key={standing.id} align="right">
                        {points > 0 && "+"}
                        {points}
                      </TableCell>
                    );
                  })}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </DialogContent>
      <DialogActions
        sx={{
          flexDirection: "row",
          justifyContent: "center",
          alignItems: "center",
          bgcolor: "background.default",
          borderTop: "1px solid rgba(255, 255, 255, 0.12)",
          gap: { xs: 1, sm: 1.5 },
          p: { xs: 1, sm: 2 },
          width: "100%",
          "& > :not(style) ~ :not(style)": {
            ml: 0,
          },
        }}
      >
        <IconButton
          aria-label="Back to game summary"
          color="primary"
          onClick={() => navigate(`/${game.sessionId}`)}
          sx={{
            border: "1px solid",
            borderColor: "primary.main",
            flexShrink: 0,
            height: 40,
            width: 40,
          }}
        >
          <ArrowBackIcon />
        </IconButton>
        <Button
          variant="contained"
          color="primary"
          size="large"
          fullWidth
          onClick={() => navigate("/")}
          sx={{ maxWidth: { xs: "none", sm: 520 } }}
        >
          Back to Lobby
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export default function TournamentResultsScreen() {
  return (
    <GameProvider>
      <TournamentResultsScreenContent />
    </GameProvider>
  );
}
