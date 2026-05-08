import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  Paper,
  CircularProgress,
} from "@mui/material";
import { useNavigate } from "react-router-dom";
import { useGameContext } from "../context/GameContext";
import { leaveGame, getAvailableAgents, type AgentInfo } from "../api/games";
import { AIPlayerSelector } from "./AIPlayerSelector";

export function GameLobbyWaiting() {
  const game = useGameContext();
  const navigate = useNavigate();
  const [isLeaving, setIsLeaving] = useState(false);
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [dialogOpen, setDialogOpen] = useState(false);
  const playersNeeded = 3 - game.playerCount;

  useEffect(() => {
    const loadAgents = async () => {
      const availableAgents = await getAvailableAgents();
      setAgents(availableAgents);
    };
    loadAgents();
  }, []);

  const handleLeaveGame = async () => {
    if (!game.player?.id || !game.gameId) return;

    try {
      setIsLeaving(true);
      await leaveGame(game.gameId, game.player?.id);
      navigate("/");
    } catch (error) {
      console.error("Failed to leave game:", error);
      setIsLeaving(false);
    }
  };

  const handleOpenDialog = () => {
    setDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setDialogOpen(false);
  };

  const handleSelectAgent = (agentId: string) => {
    game.addAgent(agentId);
    handleCloseDialog();
  };

  // Get agent IDs that are already in the game
  const agentIdsInGame = new Set(
    game.players.map((player) => player?.id).filter(Boolean),
  );

  return (
    <Box
      sx={{
        position: "absolute",
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
        textAlign: "center",
        zIndex: 50,
        width: { xs: "90%", sm: "auto" },
        maxWidth: { xs: "350px", sm: "500px" },
        px: { xs: 2, sm: 0 },
      }}
    >
      <Typography
        variant="h4"
        sx={{
          mb: 2,
          color: "#b1c4d7",
          fontWeight: 600,
          fontSize: { xs: "20px", sm: "24px" },
        }}
      >
        Waiting for Players
      </Typography>

      {game.gameCode && (
        <Paper
          elevation={3}
          sx={{
            background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
            p: { xs: "12px 15px", sm: "15px 20px" },
            borderRadius: "12px",
            my: { xs: 2, sm: 3 },
            boxShadow: "0 4px 15px rgba(102, 126, 234, 0.3)",
          }}
        >
          <Typography
            sx={{
              color: "rgba(255, 255, 255, 0.9)",
              fontSize: { xs: "11px", sm: "12px" },
              textTransform: "uppercase",
              letterSpacing: "1px",
              mb: 0.5,
            }}
          >
            Game Code
          </Typography>
          <Typography
            sx={{
              color: "white",
              fontSize: { xs: "24px", sm: "28px" },
              fontWeight: "bold",
              letterSpacing: { xs: "4px", sm: "6px" },
              mb: 0.5,
              fontFamily: "'Courier New', monospace",
            }}
          >
            {game.gameCode}
          </Typography>
          <Typography
            sx={{
              color: "rgba(255, 255, 255, 0.8)",
              fontSize: { xs: "10px", sm: "11px" },
              fontStyle: "italic",
            }}
          >
            Share this code with friends to join
          </Typography>
        </Paper>
      )}

      <Typography
        sx={{
          fontSize: { xs: "16px", sm: "18px" },
          color: "#667eea",
          fontWeight: 600,
          mb: 2,
        }}
      >
        {game.playerCount} / 3 players joined
      </Typography>

      {playersNeeded > 0 && (
        <>
          <Typography
            sx={{
              color: "#939393",
              fontSize: { xs: "13px", sm: "14px" },
              my: 2,
              fontStyle: "italic",
            }}
          >
            Waiting for {playersNeeded} more player
            {playersNeeded > 1 ? "s" : ""}...
          </Typography>
          <Box sx={{ display: "flex", gap: 2, justifyContent: "center" }}>
            <Button variant="contained" onClick={handleOpenDialog}>
              Add AI Player
            </Button>
            <Button
              variant="outlined"
              color="error"
              onClick={handleLeaveGame}
              disabled={isLeaving}
              startIcon={isLeaving ? <CircularProgress size={16} /> : null}
            >
              {isLeaving ? "Leaving..." : "Leave Game"}
            </Button>
          </Box>

          <AIPlayerSelector
            open={dialogOpen}
            onClose={handleCloseDialog}
            agents={agents}
            agentIdsInGame={agentIdsInGame}
            onSelectAgent={handleSelectAgent}
          />
        </>
      )}
    </Box>
  );
}
