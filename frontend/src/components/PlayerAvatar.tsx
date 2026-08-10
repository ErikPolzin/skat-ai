import WifiOffIcon from "@mui/icons-material/WifiOff";
import { Avatar, Box, Chip, Typography } from "@mui/material";
import type { Player } from "../api/games";
import { CircularTimer } from "./CircularTimer";

type AvatarPlacement = "top" | "left" | "player";

interface PlayerAvatarProps {
  player: Player;
  placement: AvatarPlacement;
  isCurrentPlayer: boolean;
  isDeclarer: boolean;
  isLoading?: boolean;
  isMobile: boolean;
  timerSize: number;
  deadline: string;
  countdown?: string | null;
  isCountdownUrgent?: boolean;
  role?: string | null;
  message?: { id: string | number; text: string };
}

const placementSx = {
  top: {
    top: { xs: 5, sm: 25 },
    left: "50%",
    transform: "translateX(-50%)",
    flexDirection: "row",
    gap: 1.25,
  },
  left: {
    left: { xs: 5, sm: 25 },
    top: "50%",
    transform: "translateY(-50%)",
    flexDirection: "column",
    gap: 0.5,
  },
  player: {
    bottom: { xs: 2, sm: 15 },
    left: "50%",
    transform: "translateX(-50%)",
    flexDirection: "row",
    gap: 1.25,
  },
} as const;

const bubblePlacementSx = {
  top: {
    top: "50%",
    left: "100%",
    transform: "translateY(-50%)",
    ml: "15px",
    "&::after": {
      top: "50%",
      left: -8,
      transform: "translateY(-50%)",
      borderColor: "transparent rgba(255,255,255,.95) transparent transparent",
      borderWidth: "8px 8px 8px 0",
    },
  },
  left: {
    top: "100%",
    left: "50%",
    transform: "translateX(-50%)",
    mt: "15px",
    "&::after": {
      top: -8,
      left: "50%",
      transform: "translateX(-50%)",
      borderColor: "transparent transparent rgba(255,255,255,.95) transparent",
      borderWidth: "0 8px 8px",
    },
  },
  player: {
    top: "50%",
    right: "100%",
    transform: "translateY(-50%)",
    mr: "15px",
    "&::after": {
      top: "50%",
      right: -8,
      transform: "translateY(-50%)",
      borderColor: "transparent transparent transparent rgba(255,255,255,.95)",
      borderWidth: "8px 0 8px 8px",
    },
  },
} as const;

export function PlayerAvatar({
  player,
  placement,
  isCurrentPlayer,
  isDeclarer,
  isLoading = false,
  isMobile,
  timerSize,
  deadline,
  countdown,
  isCountdownUrgent = false,
  role,
  message,
}: PlayerAvatarProps) {
  const size = isMobile ? 66 : timerSize;
  const {
    "&::after": bubbleArrowSx,
    ...bubblePositionSx
  } = bubblePlacementSx[placement];

  return (
    <Box
      sx={{
        position: "absolute",
        display: "flex",
        alignItems: "center",
        zIndex: 200,
        ...placementSx[placement],
      }}
    >
      <Box
        sx={{
          position: "relative",
          width: size,
          height: size,
          flexShrink: 0,
          border: "3px solid",
          borderColor: isLoading
            ? "#3498db"
            : isCurrentPlayer
              ? "#2ecc71"
              : "grey.500",
          borderRadius: "50%",
          bgcolor: "primary.main",
          color: "common.white",
          transition: "all .3s ease",
          animation: isLoading
            ? "avatarLoadingPulse 1.5s infinite"
            : isCurrentPlayer
              ? "avatarTurnPulse 2s infinite"
              : "none",
          "@keyframes avatarTurnPulse": {
            "0%, 100%": { boxShadow: "0 0 20px rgba(46,204,113,.5)" },
            "50%": { boxShadow: "0 0 30px rgba(46,204,113,.7)" },
          },
          "@keyframes avatarLoadingPulse": {
            "0%, 100%": { boxShadow: "0 0 20px rgba(52,152,219,.6)" },
            "50%": { boxShadow: "0 0 35px rgba(52,152,219,.9)" },
          },
        }}
      >
        <CircularTimer
          deadline={deadline}
          isCurrentPlayer={isCurrentPlayer}
          isAI={player.is_agent}
          size={timerSize}
        />
        <Avatar
          src={player.profile_icon || undefined}
          alt={player.name}
          sx={{
            position: "absolute",
            inset: 0,
            width: "100%",
            height: "100%",
            zIndex: 1,
            bgcolor: "primary.main",
            color: "common.white",
            fontSize: isMobile ? 26 : { sm: 28, md: 26 },
            fontWeight: "bold",
          }}
        >
          {player.name.charAt(0).toUpperCase()}
        </Avatar>
        {!player.is_online && (
          <Box
            title="Offline"
            sx={{
              position: "absolute",
              right: -5,
              bottom: -3,
              zIndex: 5,
              width: 24,
              height: 24,
              border: "2px solid white",
              borderRadius: "50%",
              bgcolor: "error.main",
              color: "common.white",
              display: "grid",
              placeItems: "center",
              pointerEvents: "none",
            }}
          >
            <WifiOffIcon aria-label="Offline" sx={{ fontSize: 14 }} />
          </Box>
        )}
        {countdown && (
          <Box
            sx={{
              position: "absolute",
              inset: 0,
              zIndex: 3,
              display: "grid",
              placeItems: "center",
              borderRadius: "50%",
              bgcolor: "rgba(20,8,8,.62)",
              color: "#ff4d4f",
              fontSize: isMobile ? 17 : 18,
              fontWeight: 800,
              lineHeight: 1,
              textShadow: "0 2px 4px rgba(0,0,0,.75)",
              pointerEvents: "none",
              animation: isCountdownUrgent ? "pulse 1s infinite" : "none",
            }}
          >
            {countdown}
          </Box>
        )}
      </Box>

      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: placement === "left" ? "center" : "flex-start",
          gap: "2px",
        }}
      >
        <Chip
          label={`${player.name}${isDeclarer ? " (D)" : ""}`}
          sx={{ bgcolor: "background.paper" }}
        />
        {role && (
          <Typography
            sx={{
              mt: { xs: "1px", sm: "3px" },
              fontSize: { xs: 11, sm: 12, md: 11 },
              fontWeight: 500,
              color: "common.white",
              textTransform: "uppercase",
              letterSpacing: ".5px",
              textShadow: "0 1px 3px rgba(0,0,0,.5)",
            }}
          >
            {role}
          </Typography>
        )}
      </Box>

      {message && (
        <Box
          key={message.id}
          sx={{
            position: "absolute",
            width: 100,
            maxWidth: { xs: 150, sm: 100 },
            p: { xs: "6px 10px", sm: "8px 12px" },
            borderRadius: "10px",
            bgcolor: "rgba(255,255,255,.95)",
            boxShadow: "0 4px 8px rgba(0,0,0,.2)",
            color: "#333",
            fontSize: { xs: 12, sm: 14 },
            textAlign: "center",
            whiteSpace: "normal",
            pointerEvents: "none",
            zIndex: 10000,
            animation: "avatarMessageFade 5s ease-in-out",
            "@keyframes avatarMessageFade": {
              "0%, 100%": { opacity: 0 },
              "10%, 90%": { opacity: 1 },
            },
            "&::after": {
              content: '""',
              position: "absolute",
              width: 0,
              height: 0,
              borderStyle: "solid",
              ...bubbleArrowSx,
            },
            ...bubblePositionSx,
          }}
        >
          {message.text}
        </Box>
      )}
    </Box>
  );
}
