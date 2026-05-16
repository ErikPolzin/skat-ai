import { useNavigate } from "react-router-dom";
import {
  Box,
  Container,
  IconButton,
  Paper,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import HistoryIcon from "@mui/icons-material/History";
import PlayerHistory from "../components/PlayerHistory";

export default function HistoryScreen() {
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));

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
          <HistoryIcon color="primary" />
          <Typography
            variant={isMobile ? "h5" : "h4"}
            component="h1"
            sx={{ fontWeight: 700 }}
          >
            Game History
          </Typography>
        </Box>
        <PlayerHistory />
      </Paper>
    </Container>
  );
}
