import {
  Avatar,
  Button,
  Card,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getSessionResults, type SessionPlayerResult } from "../api/games";
import { useGameContext } from "../context/GameContext";

interface TournamentResultsScreenProps {
  onBack: () => void;
}

export function TournamentResultsScreen({
  onBack,
}: TournamentResultsScreenProps) {
  const game = useGameContext();
  const navigate = useNavigate();
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
  const winner = standings[0];

  return (
    <Stack
      direction="column"
      sx={{
        position: "fixed",
        width: "100vw",
        height: "100vh",
        overflow: "auto",
        bgcolor: "background.default",
        zIndex: 2000,
        px: 2,
      }}
    >
      <Stack sx={{ my: 3, alignItems: "center" }}>
        <Typography variant="h3" sx={{ fontWeight: 900 }}>
          {winner ? `${winner.name} wins` : "Tournament Results"}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {game.sessionResults.length} games completed
        </Typography>
      </Stack>
      <Stack
        direction={"row"}
        aria-label="Final standings"
        spacing={4}
        sx={{ justifyContent: "center" }}
      >
        {standings.map((standing, index) => (
          <Card
            key={standing.id}
            elevation={index === 0 ? 8 : 3}
            variant="outlined"
          >
            <Stack direction={"column"}>
              <Typography
                variant="h3"
                color="primary"
                sx={{ fontWeight: "bold", textAlign: "center" }}
              >
                {standing.total > 0 && "+"}
                {standing.total}
              </Typography>
              {hasFinalRatings && (
                <Typography
                  variant="h6"
                  color={standing.ratingChange > 0 ? "success" : "error"}
                  sx={{ textAlign: "center" }}
                >
                  {standing.ratingChange > 0 && "+"}
                  {standing.ratingChange} Elo
                </Typography>
              )}
              <ListItem>
                <ListItemAvatar>
                  <Avatar
                    src={standing.profileIcon}
                    alt={standing.name}
                    className="tournament-avatar"
                  >
                    {standing.name.charAt(0).toUpperCase()}
                  </Avatar>
                </ListItemAvatar>
                <ListItemText
                  primary={standing.name}
                  secondary={hasFinalRatings && standing.ratingAfter}
                />
              </ListItem>
            </Stack>
          </Card>
        ))}
      </Stack>
      <TableContainer component={Paper} sx={{ flexGrow: 1, my: 2 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Game</TableCell>
              <TableCell>Contract</TableCell>
              {standings.map((standing) => (
                <TableCell key={standing.id} align="right">
                  {standing.name}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {game.sessionResults.map((sessionGame, index) => (
              <TableRow key={sessionGame.game_id}>
                <TableCell>{index + 1}</TableCell>
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
      <Stack
        direction="row"
        sx={{ justifyContent: "center", mb: 2 }}
        spacing={2}
      >
        <Button
          variant="outlined"
          color="primary"
          size="large"
          onClick={onBack}
        >
          Back to Game Summary
        </Button>
        <Button
          variant="contained"
          color="primary"
          size="large"
          onClick={() => navigate("/")}
        >
          Back to Lobby
        </Button>
      </Stack>
    </Stack>
  );
}
