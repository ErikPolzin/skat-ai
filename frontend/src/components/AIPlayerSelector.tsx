import React from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  Card,
  CardActionArea,
  Avatar,
  Typography,
  Chip,
} from "@mui/material";
import type { AgentInfo } from "../api/games";

interface AIPlayerSelectorProps {
  open: boolean;
  onClose: () => void;
  agents: AgentInfo[];
  agentIdsInGame: Set<string | undefined>;
  onSelectAgent: (agentId: string) => void;
}

const getAgentTypeLabel = (agent: AgentInfo): string => {
  if (agent.card_play_type === "neural") return "Neural";
  if (agent.card_play_type === "mcts") return "MCTS";
  return "Heuristic";
};

const getAgentTypeColor = (
  agent: AgentInfo,
): "primary" | "secondary" | "default" => {
  if (agent.card_play_type === "neural") return "primary";
  if (agent.card_play_type === "mcts") return "secondary";
  return "default";
};

export function AIPlayerSelector({
  open,
  onClose,
  agents,
  agentIdsInGame,
  onSelectAgent,
}: AIPlayerSelectorProps) {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle sx={{ textAlign: "center", pb: 1 }}>
        Select AI Player
      </DialogTitle>
      <DialogContent sx={{ pt: 2, pb: 3 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "repeat(2, 1fr)",
              sm: "repeat(3, 1fr)",
            },
            gap: 2,
          }}
        >
          {agents.map((agent) => {
            const isInGame = agentIdsInGame.has(agent.id);
            return (
              <Card
                key={agent.id}
                elevation={2}
                sx={{
                  transition: "all 0.2s",
                  opacity: isInGame ? 0.5 : 1,
                  "&:hover": !isInGame
                    ? {
                        transform: "translateY(-4px)",
                        boxShadow: 4,
                      }
                    : {},
                }}
              >
                <CardActionArea
                  onClick={() => !isInGame && onSelectAgent(agent.id)}
                  disabled={isInGame}
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    p: 2,
                    position: "relative",
                  }}
                >
                  <Chip
                    label={
                      isInGame ? "In Game" : getAgentTypeLabel(agent)
                    }
                    size="small"
                    color={isInGame ? "default" : getAgentTypeColor(agent)}
                    sx={{
                      fontWeight: 500,
                      position: "absolute",
                      top: 8,
                      right: 8,
                      fontSize: "0.65rem",
                      height: 20,
                      zIndex: 1,
                    }}
                  />
                  <Avatar
                    src={agent.profile_icon || undefined}
                    sx={{
                      width: 80,
                      height: 80,
                      mb: 1.5,
                      fontSize: "2rem",
                      bgcolor: "primary.main",
                    }}
                  >
                    {agent.name.charAt(0)}
                  </Avatar>
                  <Typography
                    variant="subtitle1"
                    sx={{
                      fontWeight: 600,
                      textAlign: "center",
                    }}
                  >
                    {agent.name}
                  </Typography>
                </CardActionArea>
              </Card>
            );
          })}
        </Box>
      </DialogContent>
    </Dialog>
  );
}
