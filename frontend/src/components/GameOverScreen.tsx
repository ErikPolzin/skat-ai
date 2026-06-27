import {
  Box,
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
import CheckIcon from "@mui/icons-material/Check";
import HomeIcon from "@mui/icons-material/Home";
import { useNavigate } from "react-router-dom";
import { useGameContext } from "../context/GameContext";
import type { Player } from "../api/games";

function ResultPlayerAvatar({
  player,
  points,
}: {
  player: Player;
  points?: number;
}) {
  const showReadyCheck = player.ready_for_next || player.is_agent;

  return (
    <Box
      sx={{
        alignItems: "center",
        display: "flex",
        flexDirection: "column",
        gap: 0.5,
        minWidth: 64,
      }}
    >
      <Box
        className="avatar-circle"
        sx={{
          bgcolor: "rgba(255, 255, 255, 0.12)",
          border: "2px solid rgba(255, 255, 255, 0.24)",
          borderRadius: "50%",
          boxShadow: "0 8px 18px rgba(0, 0, 0, 0.24)",
          height: { xs: 48, sm: 56 },
          overflow: "visible",
          position: "relative",
          width: { xs: 48, sm: 56 },
        }}
      >
        <Box
          className="avatar-content"
          sx={{
            alignItems: "center",
            borderRadius: "50%",
            display: "flex",
            fontSize: { xs: 20, sm: 24 },
            fontWeight: 800,
            height: "100%",
            justifyContent: "center",
            left: 0,
            overflow: "hidden",
            position: "absolute",
            top: 0,
            width: "100%",
            zIndex: 1,
          }}
        >
          {player.profile_icon ? (
            <Box
              component="img"
              src={player.profile_icon}
              alt={player.name}
              sx={{ height: "100%", objectFit: "cover", width: "100%" }}
            />
          ) : (
            <span>{player.name.charAt(0).toUpperCase()}</span>
          )}
        </Box>
        {showReadyCheck && (
          <Box
            className="avatar-ready-check"
            aria-label="Accepted new game"
            sx={{ fontSize: { xs: 34, sm: 40 } }}
          >
            <CheckIcon fontSize="inherit" />
          </Box>
        )}
      </Box>
      <Typography
        component="span"
        sx={{
          color: "text.secondary",
          fontSize: 12,
          fontWeight: 700,
          lineHeight: 1.1,
          maxWidth: 82,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
        title={player.name}
      >
        {player.name}
      </Typography>
      {points !== undefined && (
        <Typography
          component="span"
          sx={{
            color: "text.primary",
            fontSize: 12,
            fontWeight: 800,
            lineHeight: 1,
          }}
        >
          {points} points
        </Typography>
      )}
    </Box>
  );
}

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
  const playerReadyForNext = game.player?.ready_for_next ?? false;
  const readyPlayerCount = game.players.filter(
    (p) => p && !p.is_agent && p.ready_for_next,
  ).length;
  const humanPlayerCount = game.players.filter((p) => p && !p.is_agent).length;
  const maxGamesLabel = game.maxGames === 0 ? "∞" : game.maxGames;
  const activePlayers = game.players.filter((p): p is Player => p !== null);
  const declarerPlayers =
    game.declarerPosition === null
      ? []
      : activePlayers.filter((p) => p.position === game.declarerPosition);
  const defenderPlayers =
    game.declarerPosition === null
      ? activePlayers
      : activePlayers.filter((p) => p.position !== game.declarerPosition);
  const totalCardPoints = 120;
  const scoreBarSegments = isRamsch
    ? activePlayers.map((player, index) => ({
        key: player.id,
        label: player.name,
        score: game.playerScores[player.position],
        color: index === 0 ? "#ffb300" : index === 1 ? "#ab6cff" : "#4f8cff",
      }))
    : [
        {
          key: "declarer",
          label: "Declarer",
          score: game.declarerScore,
          color: "#4f8cff",
        },
        {
          key: "defenders",
          label: "Defenders",
          score: game.opponentScore,
          color: "#ffb300",
        },
      ];
  const claimedScore = scoreBarSegments.reduce(
    (sum, segment) => sum + segment.score,
    0,
  );
  const unclaimedScore = Math.max(0, totalCardPoints - claimedScore);
  const scoreBarTitle = scoreBarSegments
    .map((segment) => `${segment.label}: ${segment.score}`)
    .concat(unclaimedScore > 0 ? [`Unclaimed: ${unclaimedScore}`] : [])
    .join(" · ");
  const runningTotals = activePlayers.map((player) => ({
    player,
    total: game.sessionResults.reduce(
      (sum, sessionGame) => sum + (sessionGame.player_results[player.id] || 0),
      0,
    ),
  }));

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
              sm: "min(640px, calc(100vw - 180px))",
            },
            height: { xs: "auto", sm: "min(560px, calc(100vh - 120px))" },
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
          justifyContent: "flex-start",
          gap: { xs: 1.25, sm: 1.5 },
          overflowY: "auto",
          p: { xs: 1.25, sm: 2.25 },
          pb: { xs: 1, sm: 1.5 },
          textAlign: "center",
          "& > *": {
            flexShrink: 0,
          },
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
        <Box
          sx={{
            alignItems: "stretch",
            display: "grid",
            gap: { xs: 1.25, sm: 1.75 },
            gridTemplateColumns: "1fr",
            width: "100%",
          }}
        >
          <Box
            sx={{
              border: "1px solid rgba(255, 255, 255, 0.14)",
              borderRadius: 1.5,
              display: "grid",
              gap: 1.25,
              gridTemplateColumns: {
                xs: game.declarerPosition === null ? "1fr" : "1fr 1fr",
                sm: game.declarerPosition === null ? "1fr" : "0.8fr 1.2fr",
              },
              p: { xs: 1, sm: 1.25 },
            }}
          >
            {game.declarerPosition !== null && (
              <Box sx={{ minWidth: 0 }}>
                <Typography
                  component="span"
                  sx={{
                    color: "#fdd835",
                    display: "block",
                    fontSize: 12,
                    fontWeight: 800,
                    mb: 1,
                    textTransform: "uppercase",
                  }}
                >
                  Declarer
                </Typography>
                <Box sx={{ display: "flex", justifyContent: "center" }}>
                  {declarerPlayers.map((player) => (
                    <ResultPlayerAvatar key={player.id} player={player} />
                  ))}
                </Box>
                <Typography
                  component="span"
                  sx={{
                    display: "block",
                    fontSize: 13,
                    fontWeight: 800,
                    mt: 1,
                  }}
                >
                  {game.declarerScore} points
                </Typography>
              </Box>
            )}
            <Box sx={{ minWidth: 0 }}>
              <Typography
                component="span"
                sx={{
                  color: "#fdd835",
                  display: "block",
                  fontSize: 12,
                  fontWeight: 800,
                  mb: 1,
                  textTransform: "uppercase",
                }}
              >
                {game.declarerPosition === null ? "Players" : "Defenders"}
              </Typography>
              <Box
                sx={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: 1,
                  justifyContent: "center",
                }}
              >
                {defenderPlayers.map((player) => (
                  <ResultPlayerAvatar
                    key={player.id}
                    player={player}
                    points={
                      game.declarerPosition === null
                        ? game.playerScores[player.position]
                        : undefined
                    }
                  />
                ))}
              </Box>
              {game.declarerPosition !== null && (
                <Typography
                  component="span"
                  sx={{
                    display: "block",
                    fontSize: 13,
                    fontWeight: 800,
                    mt: 1,
                  }}
                >
                  {game.opponentScore} points
                </Typography>
              )}
            </Box>
          </Box>
          <Box
            title={scoreBarTitle}
            aria-label={`Final score: ${scoreBarTitle}`}
            sx={{
              bgcolor: "rgba(0, 0, 0, 0.26)",
              borderRadius: 999,
              display: "flex",
              flexDirection: "row",
              height: 14,
              justifyContent: "flex-start",
              justifySelf: "center",
              maxWidth: 440,
              overflow: "hidden",
              position: "relative",
              width: "100%",
            }}
          >
            {!isRamsch && (
              <>
                <Box
                  sx={{
                    bgcolor: "#ff4d4f",
                    borderRadius: 999,
                    bottom: -4,
                    left: "25%",
                    position: "absolute",
                    top: -4,
                    transform: "translateX(-50%)",
                    width: 2,
                    zIndex: 2,
                  }}
                />
                <Box
                  sx={{
                    bgcolor: "rgba(255, 255, 255, 0.82)",
                    borderRadius: 999,
                    bottom: -4,
                    left: "50%",
                    position: "absolute",
                    top: -4,
                    transform: "translateX(-50%)",
                    width: 2,
                    zIndex: 2,
                  }}
                />
                <Box
                  sx={{
                    bgcolor: "#ff4d4f",
                    borderRadius: 999,
                    bottom: -4,
                    left: "75%",
                    position: "absolute",
                    top: -4,
                    transform: "translateX(-50%)",
                    width: 2,
                    zIndex: 2,
                  }}
                />
              </>
            )}
            {scoreBarSegments.map((segment) => (
              <Box
                key={segment.key}
                sx={{
                  bgcolor: segment.color,
                  flexBasis: `${Math.max(0, segment.score / totalCardPoints) * 100}%`,
                  flexGrow: 0,
                  flexShrink: 0,
                  minHeight: 0,
                  minWidth: 0,
                }}
              />
            ))}
            {unclaimedScore > 0 && (
              <Box
                sx={{
                  bgcolor: "rgba(255, 255, 255, 0.12)",
                  flexBasis: `${(unclaimedScore / totalCardPoints) * 100}%`,
                  flexGrow: 0,
                  flexShrink: 0,
                  minHeight: 0,
                  minWidth: 0,
                }}
              />
            )}
          </Box>
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
                          {result.is_schwarz
                            ? "Schwarz Made"
                            : "Schneider Made"}
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
          <Box
            sx={{
              alignItems: "center",
              borderBottom: "1px solid rgba(255, 255, 255, 0.1)",
              borderTop: "1px solid rgba(255, 255, 255, 0.1)",
              display: "grid",
              gap: 1,
              gridTemplateColumns: {
                xs: "1fr",
                sm: `auto repeat(${runningTotals.length}, minmax(0, 1fr))`,
              },
              py: 1,
            }}
          >
            <Typography
              component="span"
              sx={{
                color: "text.secondary",
                fontSize: 12,
                fontWeight: 800,
                textTransform: "uppercase",
              }}
            >
              Running totals
            </Typography>
            <Box
              sx={{
                display: { xs: "grid", sm: "contents" },
                gap: 1,
                gridTemplateColumns: `repeat(${runningTotals.length}, minmax(0, 1fr))`,
              }}
            >
              {runningTotals.map(({ player, total }) => (
                <Box key={player.id} sx={{ minWidth: 0 }}>
                  <Typography
                    component="span"
                    sx={{
                      color:
                        player.id === game.player?.id
                          ? "primary.light"
                          : "text.secondary",
                      display: "block",
                      fontSize: 12,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                    title={player.name}
                  >
                    {player.name}
                  </Typography>
                  <Typography
                    component="span"
                    sx={{
                      color:
                        total > 0
                          ? "success.main"
                          : total < 0
                            ? "error.main"
                            : "text.primary",
                      display: "block",
                      fontSize: 14,
                      fontWeight: 800,
                    }}
                  >
                    {total > 0 ? "+" : ""}
                    {total}
                  </Typography>
                </Box>
              ))}
            </Box>
          </Box>
        </Box>
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
        {game.canPlayNext &&
          game.completionPolicy !== "strict" &&
          !playerReadyForNext && (
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
            {!playerReadyForNext ? (
              <Button
                variant="contained"
                color="primary"
                size="large"
                onClick={() => game.controls.playNextGame()}
                loading={game.controls.isLoading}
                disabled={!game.controls.isConnected || playerReadyForNext}
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
