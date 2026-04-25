import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemSecondaryAction,
  ListItemText,
  Typography,
} from "@mui/material";
import { useEffect, useState } from "react";
import {
  selectPlayerId,
  selectUsername,
  useProfileStore,
} from "../stores/profileStore";
import { GameSession, getGames, joinGame } from "../api/games";
import RefreshIcon from "@mui/icons-material/Refresh";
import { useNavigate } from "react-router-dom";

const AvailableGames = () => {
  const profileId = useProfileStore(selectPlayerId);
  const username = useProfileStore(selectUsername);
  const [isFetching, setIsFetching] = useState(false);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [games, setGames] = useState<GameSession[]>([]);
  const navigate = useNavigate();

  useEffect(() => {
    if (profileId) {
      fetchGames();
    }
  }, [profileId]);

  const fetchGames = async () => {
    try {
      setIsFetching(true);
      const data = await getGames(profileId || undefined);
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch games:", error);
    } finally {
      setIsFetching(false);
    }
  };

  const handleQuickJoin = async (code: string) => {
    try {
      setIsLoading(true);
      // Join the game (either the newly created one or an existing one)
      const data = await joinGame(code, username || "", profileId || undefined);
      // Navigate to the game
      navigate(`/game/${data.game_id}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
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
          {isFetching ? <CircularProgress size={24} /> : <RefreshIcon />}
        </IconButton>
      </Box>

      {games.length === 0 ? (
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
            <ListItem key={game.id}>
              <ListItemText
                primary={game.code}
                secondary={`${game.player_count}/3 players`}
              />
              <ListItemSecondaryAction>
                <Button
                  variant="text"
                  onClick={() => handleQuickJoin(game.code)}
                  disabled={isLoading}
                >
                  Join
                </Button>
              </ListItemSecondaryAction>
            </ListItem>
          ))}
        </List>
      )}
    </Box>
  );
};

export default AvailableGames;
