import React, {
  createContext,
  useContext,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { Game, useGame } from "../hooks/useGame";
import { useParams } from "react-router-dom";
import { useProfileStore } from "../stores/profileStore";
import { useWebSocketContext } from "./WebSocketContext";
import { GameControls, useControls } from "../hooks/useControls";
import { Message, Player } from "../types";

const GameContext = createContext<
  | (Game & {
      controls: GameControls;
    })
  | null
>(null);

export function GameProvider({ children }: { children: ReactNode }) {
  const { gameId } = useParams<{ gameId: string }>();
  const playerId = useProfileStore((state) => state.playerId);

  const game = useGame(gameId, playerId || undefined);
  const socket = useWebSocketContext();
  const controls = useControls(game, socket);

  // Handle incoming WebSocket messages
  const handleGameMessage = useCallback((message: Message) => {
    const { setGameState, addMessage } = game;

    switch (message.type) {
      case "state_update":
        // Handle the new state diff format
        if (message.data.diff) {
          const diff = message.data.diff;

          // Apply all state changes at once first
          setGameState(diff.changes || {});

          // Show the action description in the message log AFTER state is updated
          if (diff.description && diff.description.trim() !== "") {
            // Find player position from player_id in the diff
            let playerPosition: number | undefined;
            if (diff.player_id) {
              // Get the updated players from the game state
              const updatedPlayers = diff.changes?.players || game.players;
              const player = updatedPlayers.find(
                (p: Player | undefined) => p?.player_id === diff.player_id,
              );
              if (player) {
                playerPosition = player.position;
              }
            }
            addMessage(diff.description, false, playerPosition);
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
