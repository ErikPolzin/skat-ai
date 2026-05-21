import React, {
  createContext,
  useContext,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import { type Game, useGame } from "../hooks/useGame";
import { useNavigate, useParams } from "react-router-dom";
import { useProfileStore } from "../stores/profileStore";
import { useWebSocketContext } from "./WebSocketContext";
import { type GameControls, useControls } from "../hooks/useControls";
import {
  type ErrorMessage,
  type Message,
  type PlayerForfeitMessage,
  type PlayerLeftMessage,
  type PlayerOfflineMessage,
  type StartNextGameMessage,
  type StateUpdateMessage,
} from "../types";
import { type GamePosition, type GameInfo } from "../api/games";

const GameContext = createContext<
  | (Game & {
      controls: GameControls;
      trickWinnerRef: React.MutableRefObject<{
        winner: GamePosition | null;
        declarer: GamePosition | null;
      }>;
    })
  | null
>(null);

export function GameProvider({ children }: { children: ReactNode }) {
  const { sessionId } = useParams<{ sessionId: string }>();
  const playerId = useProfileStore((state) => state.playerId);

  const game = useGame(sessionId, playerId || undefined);
  const socket = useWebSocketContext();
  const controls = useControls(game, socket);
  const navigate = useNavigate();

  // Store trick winner in a ref that persists across renders
  const trickWinnerRef = React.useRef<{
    winner: GamePosition | null;
    declarer: GamePosition | null;
  }>({
    winner: game.trickWinner,
    declarer: game.declarerPosition,
  });

  // Handle incoming WebSocket messages
  const handleGameMessage = useCallback((message: Message<unknown>) => {
    const {
      setGameInfo,
      addMessage,
      setSessionResults,
      setGamesPlayed,
      updatePlayerOnlineStatus,
    } = game;

    switch (message.type) {
      case "state_update":
        {
          const data = (message as StateUpdateMessage).data;
          // Handle the new state diff format
          const diff = data.diff as GameInfo;

          // If trick is complete (3 cards) or being cleared, update the ref
          // This allows exit animations to access the correct winner
          if (
            (diff.state.trick?.length === 3 || diff.state.trick === null) &&
            diff.state.trick_winner != null
          ) {
            trickWinnerRef.current = {
              winner: diff.state.trick_winner,
              declarer: diff.state.declarer,
            };
          }

          // Apply all state changes at once first
          setGameInfo(diff);

          // Handle session results if included (from game complete or session_updated)
          if (data.session_results) {
            setSessionResults(data.session_results);
          }
          if (data.session_player_results) {
            game.setSessionPlayerResults(data.session_player_results);
          }
          if (data.games_played !== undefined) {
            setGamesPlayed(data.games_played);
          }

          // Show the action description in the message log AFTER state is updated
          // (but not for session_updated events which are silent)
          if (
            data.description &&
            data.description.trim() !== "" &&
            data.action_type !== "session_updated"
          ) {
            const fromPlayer = data.from_player;
            const fromPlayerId = fromPlayer
              ? game.players[fromPlayer]?.id
              : undefined;
            addMessage(data.description, false, fromPlayer);
            if (fromPlayerId) {
              updatePlayerOnlineStatus(fromPlayerId, true);
            }
          }
        }

        break;
      case "start_next_game":
        {
          const data = (message as StartNextGameMessage).data;
          navigate(`/${data.session_id || game.sessionId}`, { replace: true });
        }
        break;
      case "player_offline":
        {
          const data = (message as PlayerOfflineMessage).data;
          // Update player's online status in state
          if (data.player_id) {
            updatePlayerOnlineStatus(data.player_id, false);
          }
        }
        break;
      case "player_left":
        {
          const data = (message as PlayerLeftMessage).data;
          // Player left the game lobby
          if (data.player_name) {
            addMessage(`${data.player_name} left the game`, false);
          } else {
            addMessage(`A player left the game`, false);
          }
        }
        // The server will send a state_update with the new player list
        break;
      case "player_forfeit":
        {
          const data = (message as PlayerForfeitMessage).data;
          // Player forfeited an active game
          if (data.player_name) {
            addMessage(`${data.player_name} has forfeited. Game ended.`, true);
          } else {
            addMessage(`A player has forfeited. Game ended.`, true);
          }
        }
        // The server will send a state_update with game complete status
        break;
      case "error":
        {
          const data = (message as ErrorMessage).data;
          addMessage(data.message, true);
        }
        break;

      default:
        break;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Set up WebSocket message handler for game updates
  useEffect(() => {
    if (!sessionId || !playerId) return;

    // Add handler for game messages
    const cleanup = socket.addMessageHandler("game", handleGameMessage);

    return cleanup;
    // Only re-run when sessionId or playerId changes, not when game state changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, playerId, socket.addMessageHandler]);

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

// eslint-disable-next-line react-refresh/only-export-components
export function useGameContext() {
  const game = useContext(GameContext);
  if (!game) {
    throw new Error("useGameContext must be used within a GameProvider");
  }
  return game;
}
