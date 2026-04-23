import React from "react";
import { Box, Typography, Button, Paper } from "@mui/material";
import { useGameContext } from "../context/GameContext";

export function GameLobbyWaiting() {
  const game = useGameContext();
  const playersNeeded = 3 - game.playerCount;

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
          <Button variant="contained" onClick={() => game.addAgent()}>
            Add AI Player
          </Button>
        </>
      )}
    </Box>
  );
}
