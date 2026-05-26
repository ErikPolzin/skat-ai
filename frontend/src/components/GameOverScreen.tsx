import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Typography,
} from "@mui/material";
import HomeIcon from "@mui/icons-material/Home";
import { useNavigate } from "react-router-dom";
import { useGameContext } from "../context/GameContext";

export function GameOverScreen() {
  const game = useGameContext();
  const navigate = useNavigate();

  if (!game.gameOver || !game.result) return null;

  const { result } = game;
  const absMatadors = Math.abs(result.matadors);
  const isRamsch = game.gameMode === "ramsch";
  const ramschScores = game.players
    .flatMap((player) =>
      player
        ? [
            {
              id: player.id,
              name:
                player.id === game.player?.id
                  ? `${player.name} (You)`
                  : player.name,
              points: game.playerScores[player.position],
            },
          ]
        : [],
    )
    .sort((a, b) => a.points - b.points);
  const lowestRamschScore = ramschScores[0]?.points ?? 0;

  // Map suit symbols to full names
  const suitNames: { [key: string]: string } = {
    "♣": "Clubs",
    "♠": "Spades",
    "♥": "Hearts",
    "♦": "Diamonds",
  };
  const isTournamentComplete = !game.canPlayNext;
  const readyPlayerCount = game.players.filter(
    (p) => p && !p.is_agent && p.ready_for_next,
  ).length;
  const humanPlayerCount = game.players.filter((p) => p && !p.is_agent).length;
  const maxGamesLabel = game.maxGames === 0 ? "∞" : game.maxGames;

  return (
    <Dialog
      open
      maxWidth={false}
      sx={{
        "& .MuiDialog-container": {
          alignItems: "center",
          justifyContent: "center",
        },
      }}
      slotProps={{
        paper: {
          elevation: 4,
          sx: {
            width: {
              xs: "calc(100vw - 12px)",
              sm: "min(560px, calc(100vw - 180px))",
            },
            height: { xs: "auto", sm: "min(450px, calc(100vh - 180px))" },
            maxHeight: { xs: "calc(100vh - 96px)", sm: "none" },
            minWidth: { xs: 0, sm: 280 },
            minHeight: { xs: 0, sm: 320 },
            m: 0,
            bgcolor: "background.paper",
            borderRadius: { xs: 1.5, sm: 2 },
            position: "relative",
          },
        },
      }}
    >
      {isTournamentComplete && (
        <IconButton
          aria-label="Back to lobby"
          onClick={() => navigate("/")}
          sx={{
            height: 40,
            left: { xs: 8, sm: 12 },
            position: "absolute",
            top: { xs: 8, sm: 12 },
            width: 40,
          }}
        >
          <HomeIcon />
        </IconButton>
      )}
      <DialogContent
        sx={{
          display: "flex",
          flex: 1,
          flexDirection: "column",
          alignItems: "center",
          justifyContent: { xs: "flex-start", sm: "center" },
          gap: { xs: 1.25, sm: 1.5 },
          overflowY: "auto",
          p: { xs: 1.25, sm: 2.25 },
          pb: { xs: 1, sm: 1.5 },
          textAlign: "center",
        }}
      >
        <Typography
          component="span"
          sx={{
            color: game.playerWon ? "#4caf50" : "#f44336",
            fontSize: { xs: 24, sm: 42 },
            fontWeight: 700,
            lineHeight: 1.1,
            textShadow: "0 2px 4px rgba(0, 0, 0, 0.3)",
          }}
        >
          {game.playerWon ? "YOU WON" : "YOU LOST"}
        </Typography>
        {result.is_forfeit ? (
          <Typography
            component="span"
            sx={{
              color: "#fdd835",
              fontSize: { xs: 16, sm: 18 },
              fontWeight: 700,
              mt: 1.5,
            }}
          >
            Game forfeited due to inactivity
          </Typography>
        ) : (
          <Typography
            component="span"
            sx={{
              color: "#fdd835",
              fontSize: { xs: 16, sm: 20 },
              fontWeight: 700,
            }}
          >
            {isRamsch
              ? "Lowest score wins"
              : `${game.declarer?.name}: ${
                  game.playerWon === game.isDeclarer ? "+" : ""
                }${result.value}`}
          </Typography>
        )}
        {!result.is_forfeit && isRamsch && (
          <TableContainer
            component={Paper}
            sx={{
              width: "100%",
              bgcolor: "background.paper",
              border: "1px solid rgba(255, 255, 255, 0.14)",
            }}
          >
            <Table size="small">
              <TableBody>
                {ramschScores.map((score) => (
                  <TableRow key={score.id}>
                    <TableCell
                      sx={{
                        fontWeight:
                          score.points === lowestRamschScore
                            ? "bold"
                            : undefined,
                      }}
                    >
                      {score.name}
                      {score.points === lowestRamschScore ? " won" : ""}
                    </TableCell>
                    <TableCell
                      align="right"
                      sx={{
                        fontWeight:
                          score.points === lowestRamschScore
                            ? "bold"
                            : undefined,
                      }}
                    >
                      {score.points}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
        {!result.is_forfeit && game.isNull && result.base_value > 0 && (
          <TableContainer
            component={Paper}
            sx={{
              width: "100%",
              bgcolor: "background.paper",
              border: "1px solid rgba(255, 255, 255, 0.14)",
            }}
          >
            <Table size="small">
              <TableBody>
                <TableRow>
                  <TableCell>Null contract</TableCell>
                  <TableCell align="right">
                    {result.declarer_won ? "Won" : "Lost"}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={{ fontWeight: "bold" }}>Total</TableCell>
                  <TableCell align="right" sx={{ fontWeight: "bold" }}>
                    {result.value}
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>
        )}
        {!result.is_forfeit &&
          !game.isNull &&
          !isRamsch &&
          result.base_value > 0 && (
            <TableContainer
              component={Paper}
              sx={{
                width: "100%",
                bgcolor: "background.paper",
                border: "1px solid rgba(255, 255, 255, 0.14)",
              }}
            >
              <Table size="small">
                <TableBody>
                  <TableRow>
                    <TableCell>
                      Game, {result.matadors > 0 ? "With" : "Without"}{" "}
                      {absMatadors}
                    </TableCell>
                    <TableCell align="right">
                      {1 + absMatadors} (+{1 + absMatadors})
                    </TableCell>
                  </TableRow>
                  {result.is_schneider && (
                    <TableRow>
                      <TableCell>
                        {result.is_schwarz ? "Schwarz Made" : "Schneider Made"}
                      </TableCell>
                      <TableCell align="right">
                        {result.is_schwarz ? 2 : 1} (
                        {result.declarer_won ? "+" : "-"}
                        {result.is_schwarz ? 2 : 1})
                      </TableCell>
                    </TableRow>
                  )}
                  <TableRow>
                    <TableCell>
                      {game.gameMode === "grand"
                        ? "Grand"
                        : `${suitNames[game.trumpSuit]} contract`}
                      {result.declarer_won ? ", Won" : ", Lost"}
                    </TableCell>
                    <TableCell align="right">
                      {!result.declarer_won && `-2×(`}
                      {result.multiplier}×{result.base_value}
                      {!result.declarer_won && `)`}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={{ fontWeight: "bold" }}>Total</TableCell>
                    <TableCell align="right" sx={{ fontWeight: "bold" }}>
                      {result.value}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          )}
      </DialogContent>
      <DialogActions
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          flexWrap: { xs: "nowrap", sm: "nowrap" },
          justifyContent: "center",
          gap: 1,
          p: { xs: 1.25, sm: 2 },
          pt: 0,
          width: "100%",
          "& > :not(style) ~ :not(style)": {
            ml: 0,
          },
          "& .MuiButton-root": {
            flex: { xs: "1 1 auto", sm: "1 1 0" },
            minWidth: 0,
            width: { xs: "100%", sm: "auto" },
          },
        }}
      >
        {game.canPlayNext && game.completionPolicy !== "strict" && (
          <Button
            variant="outlined"
            color="primary"
            size="large"
            onClick={async () => {
              await game.controls.endTournament();
              navigate(`/${game.sessionId}/results`);
            }}
            loading={game.controls.isLoading}
            disabled={!game.controls.isConnected || game.controls.isLoading}
          >
            End Tournament
          </Button>
        )}
        {!game.canPlayNext && (
          <Button
            variant="contained"
            color="primary"
            size="large"
            onClick={() => navigate(`/${game.sessionId}/results`)}
          >
            Tournament Results
          </Button>
        )}
        {game.canPlayNext && (
          <>
            {!game.player?.ready_for_next ? (
              <Button
                variant="contained"
                color="primary"
                size="large"
                onClick={() => game.controls.playNextGame()}
                loading={game.controls.isLoading}
                disabled={
                  !game.controls.isConnected || game.player?.ready_for_next
                }
              >
                {`Play Next (${game.gamesPlayed + 1}/${maxGamesLabel})`}
              </Button>
            ) : (
              <Typography
                component="span"
                sx={{
                  alignSelf: "center",
                  color: "text.secondary",
                  flex: { xs: "1 1 auto", sm: "1 1 0" },
                  fontSize: 14,
                  order: 2,
                  textAlign: "center",
                  width: { xs: "100%", sm: "auto" },
                }}
              >
                {readyPlayerCount} / {humanPlayerCount} players ready
              </Typography>
            )}
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
