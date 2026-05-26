import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  Container,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Paper,
  Skeleton,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import HistoryIcon from "@mui/icons-material/History";
import { Fragment, useEffect, useMemo, useState } from "react";
import { getPlayerHistory, type PlayerResult } from "../api/games";
import { selectPlayerId, useProfileStore } from "../stores/profileStore";
import RefreshIcon from "@mui/icons-material/Refresh";
import EmojiEventsIcon from "@mui/icons-material/EmojiEvents";

const formatHistoryDate = (value?: string) => {
  if (!value) return "Unknown date";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown date";

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
  }).format(date);
};

export default function HistoryScreen() {
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const [history, setHistory] = useState<PlayerResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const profileId = useProfileStore(selectPlayerId);
  const groupedHistory = useMemo(() => {
    const groups: Array<{
      date: string;
      items: Array<{ result: PlayerResult; index: number }>;
    }> = [];
    const groupByDate = new Map<string, (typeof groups)[number]>();

    history.forEach((result, index) => {
      const date = formatHistoryDate(result.finished_at);
      let group = groupByDate.get(date);
      if (!group) {
        group = { date, items: [] };
        groupByDate.set(date, group);
        groups.push(group);
      }
      group.items.push({ result, index });
    });

    return groups;
  }, [history]);

  const fetchHistory = async () => {
    if (!profileId) return;

    try {
      setIsLoading(true);
      const data = await getPlayerHistory(profileId);
      setHistory(data);
    } catch (error) {
      console.error("Failed to fetch player history:", error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchHistory();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileId]);

  return (
    <Container sx={{ py: { xs: 0, sm: 3 }, px: { xs: 0, sm: 3 } }}>
      <Paper
        elevation={3}
        sx={{
          p: { xs: 2, sm: 3 },
          width: "100%",
          borderRadius: { xs: 0, sm: 1 },
          minHeight: { xs: "100vh", sm: "auto" },
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            mb: 2,
          }}
        >
          <IconButton
            color="primary"
            onClick={() => navigate("/")}
            aria-label="Back to lobby"
            edge="start"
          >
            <ArrowBackIcon />
          </IconButton>
          <HistoryIcon />
          <Typography variant={isMobile ? "h6" : "h5"} sx={{ flexGrow: 1 }}>
            Game History
          </Typography>
          <IconButton
            onClick={fetchHistory}
            color="primary"
            disabled={isLoading}
          >
            <RefreshIcon />
          </IconButton>
        </Box>
        <Box
          sx={{
            minHeight: "200px",
            display: "flex",
            flexDirection: "column",
          }}
        >
          {isLoading && history.length === 0 ? (
            <List dense>
              {Array.from({ length: 5 }).map((_, index) => (
                <ListItem key={`skeleton-${index}`}>
                  <ListItemText
                    primary={
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1 }}
                      >
                        <Skeleton variant="text" width={30} height={20} />
                        <Skeleton variant="text" width={50} height={20} />
                        <Skeleton variant="text" width={60} height={20} />
                        <Skeleton variant="text" width={70} height={20} />
                      </Box>
                    }
                    secondary={
                      <Skeleton variant="text" width={120} height={16} />
                    }
                  />
                </ListItem>
              ))}
            </List>
          ) : history.length === 0 ? (
            <Box
              sx={{
                flexGrow: 1,
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
              }}
            >
              <Typography sx={{ py: 2 }} color="textDisabled">
                No game history
              </Typography>
            </Box>
          ) : (
            <List dense>
              {groupedHistory.map((group) => (
                <Fragment key={group.date}>
                  <ListItem disableGutters sx={{ pb: 0.25, pt: 1.25 }}>
                    <Typography
                      variant="overline"
                      color="text.secondary"
                      sx={{ fontWeight: 500, lineHeight: 1.2, opacity: 0.65 }}
                    >
                      {group.date}
                    </Typography>
                  </ListItem>
                  {group.items.map(({ result, index }) => (
                    <ListItem
                      key={result.session_id}
                      secondaryAction={
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<EmojiEventsIcon />}
                          onClick={() =>
                            navigate(`/${result.session_id}/results`)
                          }
                        >
                          Results
                        </Button>
                      }
                      sx={{ pr: 14 }}
                    >
                      <ListItemText
                        primary={
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              gap: 1,
                            }}
                          >
                            <Typography variant="body2" color="text.secondary">
                              {index + 1}.
                            </Typography>
                            <Typography
                              variant="body2"
                              sx={{
                                fontWeight: "bold",
                                color: "text.primary",
                              }}
                            >
                              {result.position
                                ? `#${result.position}`
                                : "Placed"}
                            </Typography>
                            <Typography
                              variant="body2"
                              sx={{
                                fontWeight: "bold",
                                color:
                                  result.player_points > 0
                                    ? "#4caf50"
                                    : result.player_points < 0
                                      ? "#f44336"
                                      : "text.primary",
                              }}
                            >
                              {result.player_points > 0 && "+"}
                              {result.player_points} pts
                            </Typography>
                            {result.rating_change !== undefined &&
                              result.rating_change !== 0 && (
                                <Typography
                                  variant="body2"
                                  sx={{
                                    fontWeight: "medium",
                                    color:
                                      result.rating_change > 0
                                        ? "#2196f3"
                                        : "#ff5722",
                                  }}
                                >
                                  {result.rating_change > 0 && "+"}
                                  {result.rating_change} ELO
                                </Typography>
                              )}
                          </Box>
                        }
                        secondary={
                          result.other_players &&
                          result.other_players.length > 0
                            ? `vs ${result.other_players.join(", ")}`
                            : undefined
                        }
                      />
                    </ListItem>
                  ))}
                </Fragment>
              ))}
            </List>
          )}
        </Box>
      </Paper>
    </Container>
  );
}
