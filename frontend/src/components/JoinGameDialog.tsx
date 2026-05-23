import {
  Dialog,
  Typography,
  Box,
  Stack,
  TextField,
  Button,
  CircularProgress,
  IconButton,
  DialogContent,
} from "@mui/material";
import { useState, useRef, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { joinGame } from "../api/games";
import { useSnackbarStore } from "../stores/snackbarStore";
import ArrowBack from "@mui/icons-material/ArrowBack";

const GAME_CODE_LENGTH = 4;
const GAME_CODE_PATTERN = /^[0-9A-F]$/;

interface JoinGameDialogProps {
  open: boolean;
  onClose: () => void;
}

const JoinGameDialog = ({ open, onClose }: JoinGameDialogProps) => {
  const [codeChars, setCodeChars] = useState<string[]>(
    Array(GAME_CODE_LENGTH).fill(""),
  );
  const [isJoining, setIsJoining] = useState(false);
  const inputRefs = useRef<Array<HTMLInputElement | null>>([]);
  const navigate = useNavigate();
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);
  const code = codeChars.join("");
  const canJoin = code.length === GAME_CODE_LENGTH;

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setCodeChars(Array(GAME_CODE_LENGTH).fill(""));
    window.setTimeout(() => inputRefs.current[0]?.focus(), 0);
  }, [open]);

  const fillCodeFrom = (value: string, startIndex: number) => {
    const nextCharacters = value
      .toUpperCase()
      .replace(/[^0-9A-F]/g, "")
      .slice(0, GAME_CODE_LENGTH - startIndex)
      .split("");

    if (nextCharacters.length === 0) return;

    setCodeChars((current) => {
      const next = [...current];
      nextCharacters.forEach((character, offset) => {
        next[startIndex + offset] = character;
      });
      return next;
    });

    const nextFocusIndex = Math.min(
      startIndex + nextCharacters.length,
      GAME_CODE_LENGTH - 1,
    );
    inputRefs.current[nextFocusIndex]?.focus();
  };

  const handleCodeChange = (index: number, value: string) => {
    if (value.length > 1) {
      fillCodeFrom(value, index);
      return;
    }

    const nextCharacter = value.toUpperCase();
    if (nextCharacter && !GAME_CODE_PATTERN.test(nextCharacter)) return;

    setCodeChars((current) => {
      const next = [...current];
      next[index] = nextCharacter;
      return next;
    });

    if (nextCharacter && index < GAME_CODE_LENGTH - 1) {
      inputRefs.current[index + 1]?.focus();
    }
  };

  const handleKeyDown = (
    index: number,
    event: React.KeyboardEvent<HTMLInputElement>,
  ) => {
    if (event.key === "Backspace" && !codeChars[index] && index > 0) {
      inputRefs.current[index - 1]?.focus();
      return;
    }

    if (event.key === "Enter" && canJoin) {
      handleJoinGame();
    }
  };

  const handleJoinGame = async () => {
    if (!canJoin) return;

    try {
      setIsJoining(true);
      const data = await joinGame(code);
      onClose();
      navigate(`/${data.session_id}`);
    } catch (error) {
      console.error("Failed to join game:", error);
      showSnackbar("Failed to join game", "error");
    } finally {
      setIsJoining(false);
    }
  };

  return (
    <Dialog fullScreen open={open} onClose={onClose}>
      <DialogContent
        sx={{
          minHeight: "calc(100vh - 64px)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          px: 2,
          py: 4,
          bgcolor: "primary.main",
        }}
      >
        <Stack
          spacing={3}
          sx={{
            width: "100%",
            maxWidth: 420,
            alignItems: "stretch",
          }}
        >
          <Box>
            <IconButton onClick={onClose}>
              <ArrowBack />
            </IconButton>
            <Typography
              variant="h4"
              component="h1"
              sx={{ textAlign: "center" }}
            >
              Enter Game Code
            </Typography>
          </Box>

          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: `repeat(${GAME_CODE_LENGTH}, minmax(0, 1fr))`,
              gap: { xs: 1, sm: 1.5 },
            }}
          >
            {codeChars.map((character, index) => (
              <TextField
                key={index}
                autoFocus={index === 0}
                variant="outlined"
                value={character}
                inputRef={(element) => {
                  inputRefs.current[index] = element;
                }}
                onChange={(event) =>
                  handleCodeChange(index, event.target.value)
                }
                onKeyDown={(event) =>
                  handleKeyDown(
                    index,
                    event as React.KeyboardEvent<HTMLInputElement>,
                  )
                }
                onPaste={(event) => {
                  event.preventDefault();
                  fillCodeFrom(event.clipboardData.getData("text"), index);
                }}
                disabled={isJoining}
                slotProps={{
                  htmlInput: {
                    maxLength: 1,
                    inputMode: "text",
                    autoComplete: index === 0 ? "one-time-code" : "off",
                    "aria-label": `Game code character ${index + 1}`,
                  },
                }}
                sx={{
                  "& fieldset": { border: "none" },
                  "& input": {
                    borderRadius: "16px",
                    color: "primary.main",
                    bgcolor: "#ffffff",
                    textAlign: "center",
                    fontSize: { xs: "2rem", sm: "2.5rem" },
                    fontWeight: 700,
                    textTransform: "uppercase",
                    py: { xs: 1.25, sm: 1.75 },
                  },
                }}
              />
            ))}
          </Box>
          <Button
            variant="contained"
            color="secondary"
            size="large"
            onClick={handleJoinGame}
            disabled={!canJoin || isJoining}
            startIcon={isJoining ? <CircularProgress size={20} /> : null}
            fullWidth
          >
            {isJoining ? "Joining..." : "Join Game"}
          </Button>
        </Stack>
      </DialogContent>
    </Dialog>
  );
};

export default JoinGameDialog;
