import AccessTimeIcon from "@mui/icons-material/AccessTime";
import { Box, Typography } from "@mui/material";

interface WaitingNoticeProps {
  subtitle: string;
}

export function WaitingNotice({ subtitle }: WaitingNoticeProps) {
  return (
    <Box
      sx={{
        position: "absolute",
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
        zIndex: 50,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 1.25,
        color: "rgba(255, 255, 255, 0.72)",
        textAlign: "center",
        pointerEvents: "none",
      }}
    >
      <AccessTimeIcon
        sx={{
          color: "rgba(255, 255, 255, 0.34)",
          fontSize: { xs: 46, sm: 54 },
        }}
      />
      <Typography
        variant="subtitle1"
        sx={{
          color: "rgba(255, 255, 255, 0.78)",
          fontWeight: 500,
          lineHeight: 1.25,
        }}
      >
        {subtitle}
      </Typography>
    </Box>
  );
}
