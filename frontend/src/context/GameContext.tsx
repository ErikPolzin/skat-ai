import React, {
  createContext,
  useContext,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { Game, useGame } from "../hooks/useGame";
import { useNavigate, useParams } from "react-router-dom";
import { useProfileStore } from "../stores/profileStore";
import { useWebSocketContext } from "./WebSocketContext";
import { GameControls, useControls } from "../hooks/useControls";
import { Message } from "../types";
import { type GameInfo } from "../api/games";

const GameContext = createContext<
  | (Game & {
      controls: GameControls;
      trickWinnerRef: React.MutableRefObject<{ winner: number; declarer: number }>;
    })
  | null
>(null);

export function GameProvider({ children }: { children: ReactNode }) {
  const { gameId } = useParams<{ gameId: string }>();
  const playerId = useProfileStore((state) => state.playerId);

  const game = useGame(gameId, playerId || undefined);
  const socket = useWebSocketContext();
  const controls = useControls(game, socket);
  const navigate = useNavigate();

  // Store trick winner in a ref that persists across renders
  const trickWinnerRef = React.useRef<{ winner: number; declarer: number }>({
    winner: game.trickWinner,
    declarer: game.declarerPosition,
  });

  // Handle incoming WebSocket messages
  const handleGameMessage = useCallback((message: Message) => {
    const {
      setGameInfo,
      addMessage,
      setSessionResults,
      setGamesPlayed,
      markPlayerOffline,
    } = game;

    switch (message.type) {
      case "state_update":
        // Handle the new state diff format
        if (message.data.diff) {
          const diff = message.data.diff as GameInfo;

          // If trick is complete (3 cards) or being cleared, update the ref
          // This allows exit animations to access the correct winner
          if ((diff.state.trick?.length === 3 || diff.state.trick === null) && diff.state.trick_winner >= 0) {
            trickWinnerRef.current = {
              winner: diff.state.trick_winner,
              declarer: diff.state.declarer,
            };
          }

          // Apply all state changes at once first
          setGameInfo(diff);

          // Handle session results if included (from game complete or session_updated)
          if (message.data.session_results) {
            setSessionResults(message.data.session_results);
          }
          if (message.data.games_played !== undefined) {
            setGamesPlayed(message.data.games_played);
          }

          // Show the action description in the message log AFTER state is updated
          // (but not for session_updated events which are silent)
          if (
            message.data.description &&
            message.data.description.trim() !== "" &&
            message.data.action_type !== "session_updated"
          ) {
            const fromPlayer = message.data.from_player;
            addMessage(message.data.description, false, fromPlayer);
          }
        }
        break;
      case "start_next_game":
        navigate(`/game/${message.data.game_id}`);
        break;
      case "player_offline":
        // Mark player as offline and optionally show a message
        if (message.data.player_id) {
          markPlayerOffline(message.data.player_id);
          if (message.data.player_name) {
            addMessage(`${message.data.player_name} went offline`, false);
          }
        }
        break;
      case "error":
        addMessage(message.data.message, true);
        break;

      default:
        break;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Set up WebSocket message handler for game updates
  useEffect(() => {
    if (!gameId || !playerId) return;

    // Add handler for game messages
    const cleanup = socket.addMessageHandler("game", handleGameMessage);

    return cleanup;
    // Only re-run when gameId or playerId changes, not when game state changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId, playerId, socket.addMessageHandler]);

  return (
    <GameContext.Provider
      value={{
        ...game,
        controls,
        trickWinnerRef,
      }}
    >
      {children}
    </GameContext.Provider>
  );
}

export function useGameContext() {
  const game = useContext(GameContext);
  if (!game) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return game;
}
