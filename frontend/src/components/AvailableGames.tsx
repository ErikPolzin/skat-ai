import {
  Box,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Typography,
  Skeleton,
} from "@mui/material";
import { useEffect, useState } from "react";
import {
  selectPlayerId,
  useProfileStore,
} from "../stores/profileStore";
import { type GameSession, getGames, joinGame } from "../api/games";
import RefreshIcon from "@mui/icons-material/Refresh";
import { useNavigate } from "react-router-dom";
import { useSnackbarStore } from "../stores/snackbarStore";

const AvailableGames = () => {
  const profileId = useProfileStore(selectPlayerId);
  const [isFetching, setIsFetching] = useState(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [games, setGames] = useState<GameSession[]>([]);
  const navigate = useNavigate();
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);

  useEffect(() => {
    if (profileId) {
      // eslint-disable-next-line react-hooks/immutability
      fetchGames();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileId]);

  const fetchGames = async () => {
    try {
      setIsFetching(true);
      const data = await getGames(profileId || undefined);
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch games:", error);
      showSnackbar("Failed to fetch available games", "error");
    } finally {
      setIsFetching(false);
    }
  };

  const handleQuickJoin = async (code: string) => {
    try {
      setIsLoading(true);
      // Join the game (either the newly created one or an existing one)
      const data = await joinGame(code);
      // Navigate to the game
      navigate(`/${data.session_id}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
      showSnackbar("Failed to join game", "error");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box
      sx={{
        minHeight: "200px",
        display: "flex",
        flexDirection: "column",
        mt: 1,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle1">Available Games</Typography>
        <IconButton onClick={fetchGames} color="primary" disabled={isFetching}>
          <RefreshIcon />
        </IconButton>
      </Box>

      {isFetching && games.length === 0 ? (
        <List>
          {Array.from({ length: 3 }).map((_, index) => (
            <ListItem key={`skeleton-${index}`}>
              <ListItemText
                primary={<Skeleton variant="text" width={100} height={24} />}
                secondary={<Skeleton variant="text" width={80} height={20} />}
              />
            </ListItem>
          ))}
        </List>
      ) : games.length === 0 ? (
        <Box
          sx={{
            flexGrow: 1,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          <Typography sx={{ py: 2 }} color="textDisabled">
            No pending games
          </Typography>
        </Box>
      ) : (
        <List>
          {games.map((game) => (
            <ListItem key={game.id} disablePadding>
              <ListItemButton
                onClick={() => handleQuickJoin(game.code)}
                disabled={isLoading}
              >
                <ListItemText
                  primary={game.code}
                  secondary={`${game.player_count}/3 players`}
                />
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      )}
    </Box>
  );
};

export default AvailableGames;
