import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemSecondaryAction,
  ListItemText,
  Pagination,
  Skeleton,
  Typography,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";
import RefreshIcon from "@mui/icons-material/Refresh";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
  getGameLists,
  joinGame,
  leaveGame,
  type ActiveGame,
  type GameLists as GameListsResponse,
  type GameSession,
  type PaginatedList,
} from "../api/games";
import { useSnackbarStore } from "../stores/snackbarStore";

const PAGE_SIZE = 5;

const emptyPage = <T,>(): PaginatedList<T> => ({
  items: [],
  page: {
    limit: PAGE_SIZE,
    offset: 0,
    total: 0,
    has_more: false,
  },
});

const emptyLists: GameListsResponse = {
  active: emptyPage<ActiveGame>(),
  available: emptyPage<GameSession>(),
  spectatable: emptyPage<ActiveGame>(),
};

const getPhaseLabel = (phase: string): string => {
  const phaseMap: { [key: string]: string } = {
    waiting_for_players: "Waiting",
    dealing: "Ready to Deal",
    bidding: "Bidding",
    declarer_choice: "Choosing Game",
    skat_exchange: "Skat Exchange",
    playing: "In Progress",
    complete: "Complete",
  };
  return phaseMap[phase] || phase;
};

const getPhaseColor = (
  phase: string,
): "default" | "primary" | "secondary" | "success" | "warning" => {
  if (phase === "waiting_for_players") return "warning";
  if (phase === "playing") return "success";
  if (phase === "complete") return "default";
  return "primary";
};

interface ListPagerProps {
  page: PaginatedList<unknown>["page"];
  onChange: (page: number) => void;
}

function ListPager({ page, onChange }: ListPagerProps) {
  const count = Math.ceil(page.total / page.limit);
  if (count <= 1) return null;

  return (
    <Box sx={{ display: "flex", justifyContent: "center", py: 1 }}>
      <Pagination
        count={count}
        page={Math.floor(page.offset / page.limit) + 1}
        onChange={(_, nextPage) => onChange(nextPage)}
        size="small"
      />
    </Box>
  );
}

function GameListSkeleton({ rows = 2 }: { rows?: number }) {
  return (
    <List>
      {Array.from({ length: rows }).map((_, index) => (
        <ListItem key={`skeleton-${index}`}>
          <ListItemText
            primary={<Skeleton variant="text" width={100} height={24} />}
            secondary={<Skeleton variant="text" width={160} height={20} />}
          />
        </ListItem>
      ))}
    </List>
  );
}

export default function GameLists() {
  const navigate = useNavigate();
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);
  const [lists, setLists] = useState<GameListsResponse>(emptyLists);
  const [isFetching, setIsFetching] = useState(false);
  const [isJoining, setIsJoining] = useState(false);
  const [leavingGameId, setLeavingGameId] = useState<string | null>(null);
  const [activePage, setActivePage] = useState(1);
  const [availablePage, setAvailablePage] = useState(1);
  const [spectatablePage, setSpectatablePage] = useState(1);

  const offsets = useMemo(
    () => ({
      activeOffset: (activePage - 1) * PAGE_SIZE,
      availableOffset: (availablePage - 1) * PAGE_SIZE,
      spectatableOffset: (spectatablePage - 1) * PAGE_SIZE,
    }),
    [activePage, availablePage, spectatablePage],
  );

  const fetchLists = async () => {
    try {
      setIsFetching(true);
      setLists(
        await getGameLists({
          activeLimit: PAGE_SIZE,
          availableLimit: PAGE_SIZE,
          spectatableLimit: PAGE_SIZE,
          ...offsets,
        }),
      );
    } catch (error) {
      console.error("Failed to fetch game lists:", error);
      showSnackbar("Failed to fetch games", "error");
    } finally {
      setIsFetching(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchLists();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offsets]);

  const handleJoinGame = async (code: string) => {
    try {
      setIsJoining(true);
      const data = await joinGame(code);
      navigate(`/${data.session_id}`);
    } catch (error) {
      console.error("Failed to join game:", error);
      showSnackbar("Failed to join game", "error");
    } finally {
      setIsJoining(false);
    }
  };

  const handleLeaveGame = async (gameId: string, event: React.MouseEvent) => {
    event.stopPropagation();

    try {
      setLeavingGameId(gameId);
      await leaveGame(gameId);
      await fetchLists();
      showSnackbar("Successfully left game", "success");
    } catch (error) {
      console.error("Failed to leave game:", error);
      showSnackbar("Failed to leave game", "error");
    } finally {
      setLeavingGameId(null);
    }
  };

  const renderActiveGames = (
    games: ActiveGame[],
    mode: "active" | "spectatable",
  ) => (
    <List>
      {games.map((game) => (
        <ListItem
          key={game.id}
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            mb: 1,
          }}
        >
          <ListItemText
            primary={
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  flexWrap: "wrap",
                }}
              >
                <Typography variant="subtitle1" sx={{ fontWeight: "bold" }}>
                  {game.code}
                </Typography>
                <Chip
                  label={getPhaseLabel(game.phase)}
                  size="small"
                  color={getPhaseColor(game.phase)}
                />
              </Box>
            }
            secondary={
              <Typography variant="body2" color="text.secondary">
                {game.player_names.join(", ")}
              </Typography>
            }
          />
          <ListItemSecondaryAction>
            <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
              {mode === "active" ? (
                <>
                  <Button
                    variant="contained"
                    onClick={() => navigate(`/${game.session_id}`)}
                    size="small"
                  >
                    Rejoin
                  </Button>
                  <IconButton
                    onClick={(e) => handleLeaveGame(game.id, e)}
                    disabled={leavingGameId === game.id}
                    color="error"
                    size="small"
                  >
                    {leavingGameId === game.id ? (
                      <CircularProgress size={20} color="error" />
                    ) : (
                      <CloseIcon fontSize="small" />
                    )}
                  </IconButton>
                </>
              ) : (
                <Button
                  variant="outlined"
                  size="small"
                  startIcon={<VisibilityIcon />}
                  disabled
                >
                  Spectate
                </Button>
              )}
            </Box>
          </ListItemSecondaryAction>
        </ListItem>
      ))}
    </List>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", mt: 1 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle1">Games</Typography>
        <IconButton
          onClick={fetchLists}
          color="primary"
          disabled={isFetching}
          size="small"
        >
          <RefreshIcon />
        </IconButton>
      </Box>

      {isFetching && lists.active.items.length === 0 ? (
        <GameListSkeleton />
      ) : lists.active.items.length > 0 ? (
        <>
          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Your Active Games
          </Typography>
          {renderActiveGames(lists.active.items, "active")}
          <ListPager page={lists.active.page} onChange={setActivePage} />
        </>
      ) : null}

      <Typography variant="subtitle2" sx={{ mt: 1 }}>
        Available Games
      </Typography>
      {isFetching && lists.available.items.length === 0 ? (
        <GameListSkeleton rows={3} />
      ) : lists.available.items.length === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center" }}>
          <Typography sx={{ py: 2 }} color="textDisabled">
            No pending games
          </Typography>
        </Box>
      ) : (
        <>
          <List>
            {lists.available.items.map((game) => (
              <ListItem key={game.id} disablePadding>
                <ListItemButton
                  onClick={() => handleJoinGame(game.code)}
                  disabled={isJoining}
                >
                  <ListItemText
                    primary={game.code}
                    secondary={`${game.player_count}/3 players`}
                  />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
          <ListPager page={lists.available.page} onChange={setAvailablePage} />
        </>
      )}

      {lists.spectatable.items.length > 0 && (
        <>
          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Other Games
          </Typography>
          {renderActiveGames(lists.spectatable.items, "spectatable")}
          <ListPager
            page={lists.spectatable.page}
            onChange={setSpectatablePage}
          />
        </>
      )}
    </Box>
  );
}
