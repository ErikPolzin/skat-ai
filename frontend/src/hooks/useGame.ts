import { useState, useCallback, useRef, useEffect, useMemo } from "react";
import type { Card, SessionGameResult } from "../types";
import {
  addAIAgent,
  fetchGameState,
  getSessionResults,
  type GameInfo,
  type Player,
  type ServerPlayer,
} from "../api/games";

interface GameMessage {
  id: number;
  text: string;
  isError: boolean;
  playerPosition?: number; // Position of the player who triggered this message
}

export function useGame(
  gameId: string | undefined,
  playerId: string | undefined,
) {
  // Server state - matches API response
  const [gameInfo, setGameInfo] = useState<GameInfo>({
    state: {
      id: "",
      code: "",
      session_id: "",
      game_number: 0,
      players: [null, null, null],
      current_player: 0,
      phase: "waiting_for_players",
      trick: null,
      declarer_score: 0,
      opponent_score: 0,
      declarer: null,
      mode: "grand",
      trump_suit: "♣",
      trick_starter: 0,
      trick_winner: null,
      bid_value: 0,
      listener_passed: false,
      speaker_passed: false,
      dealer_passed: false,
      matadors: 0,
      played_hand: false,
      announced_schneider: false,
      announced_schwarz: false,
      current_player_deadline: "",
      forfeited_player: null,
    },
    player_id: undefined,
    hand: [],
    skat: undefined,
    can_play_next: false,
  });

  const state = gameInfo.state;
  const hand = gameInfo.hand ?? [];
  const skatCards = gameInfo.skat ?? undefined;
  const canPlayNextFromState = gameInfo.can_play_next ?? false;
  const result = gameInfo.result;

  // Transform server players (with position as index) to client players (with position field)
  const players = useMemo<(Player | null)[]>(
    () =>
      state.players.map((p, index) => (p ? { ...p, position: index } : null)),
    [state.players],
  );

  // Derive player info from players array
  const player = useMemo(
    () => players.find((p) => p?.id === playerId),
    [players, playerId],
  );

  const playerPosition = player?.position ?? null;
  const isBiddingPhase = state.phase === "bidding";
  const isSkatExchange = state.phase === "skat_exchange";
  const isDeclarerChoice = state.phase === "declarer_choice";
  const isInLobby = state.phase === "waiting_for_players";
  const isDeclarer =
    playerPosition !== null &&
    state.declarer !== null &&
    playerPosition === state.declarer;
  const isDealer = playerPosition === 0;
  const isNull = state.mode === "null";
  const gameOver = state.phase === "complete";

  // Check if it's my turn by comparing position
  const isMyTurn = state.current_player === playerPosition;

  // UI-only state
  const [messages, setMessages] = useState<GameMessage[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sessionResults, setSessionResults] = useState<SessionGameResult[]>([]);
  const [gamesPlayed, setGamesPlayed] = useState(0);
  const messageIdCounter = useRef(0);

  const opponents = useMemo(
    () =>
      players.filter(
        (p) => p && (state.declarer === null || p.position !== state.declarer),
      ) as Player[],
    [state.declarer, players],
  );

  const declarer = useMemo(
    () =>
      state.declarer !== null
        ? players.find((p) => p && p.position === state.declarer)
        : undefined,
    [state.declarer, players],
  );

  const leftPlayer = useMemo(
    () =>
      playerPosition != null
        ? players[(playerPosition + 1) % players.length]
        : undefined,
    [playerPosition, players],
  );

  const topPlayer = useMemo(
    () =>
      playerPosition != null
        ? players[(playerPosition + 2) % players.length]
        : undefined,
    [playerPosition, players],
  );

  const playerCount = useMemo(
    () => players.filter((p) => !!p).length,
    [players],
  );

  // Helper function to get bidding role
  const getRole = useCallback(
    (position: number | null): string | null => {
      if (state.phase === "bidding") {
        switch (position) {
          case 0:
            return "Dealer";
          case 1:
            return "Listener";
          case 2:
            return "Speaker";
          default:
            return null;
        }
      }
      return null;
    },
    [state.phase],
  );

  // Fetch game state from server only when gameId changes
  useEffect(() => {
    if (!gameId) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setIsLoading(false);
      setError("No game ID provided");
      return;
    }

    // Skip if we've already fetched this game

    const loadGameState = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const data = await fetchGameState(gameId, playerId);
        setGameInfo(data);

        // Fetch session results if we have a session ID
        if (data.state.session_id) {
          try {
            const sessionData = await getSessionResults(data.state.session_id);
            if (sessionData.results && sessionData.results.length > 0) {
              setSessionResults(sessionData.results);
              setGamesPlayed(sessionData.results.length);
            }
          } catch (error) {
            console.error("Failed to fetch session results:", error);
            // Don't fail the whole load if session results fail
          }
        }

        setIsLoading(false);
      } catch (error) {
        console.error("Failed to fetch game state:", error);
        setError(
          error instanceof Error
            ? error.message
            : "Failed to load game. Please try again.",
        );
        setIsLoading(false);
      }
    };

    loadGameState();
  }, [gameId, playerId]);

  const removeCardFromHand = useCallback((card: Card) => {
    setGameInfo((prev) => ({
      ...prev,
      hand: (prev.hand || []).filter(
        (c) => !(c.rank === card.rank && c.suit === card.suit),
      ),
    }));
  }, []);

  const optimisticallyPlayCard = useCallback((card: Card) => {
    setGameInfo((prev) => ({
      ...prev,
      hand: (prev.hand || []).filter(
        (c) => !(c.rank === card.rank && c.suit === card.suit),
      ),
      state: {
        ...prev.state,
        trick: [...(prev.state.trick || []), card],
      },
    }));
  }, []);

  const undoOptimisticPlayCard = useCallback((card: Card) => {
    setGameInfo((prev) => ({
      ...prev,
      hand: [...(prev.hand || []), card],
      state: {
        ...prev.state,
        trick: (prev.state.trick || []).filter(
          (c) => !(c.rank === card.rank && c.suit === card.suit),
        ),
      },
    }));
  }, []);

  const addMessage = useCallback(
    (text: string, isError = false, playerPosition?: number) => {
      messageIdCounter.current += 1;
      const id = messageIdCounter.current;

      setMessages((prev) => [...prev, { id, text, isError, playerPosition }]);

      setTimeout(() => {
        setMessages((prev) => prev.filter((m) => m.id !== id));
      }, 5000);
    },
    [],
  );

  const reset = useCallback(() => {
    setGameInfo({
      state: {
        id: "",
        code: "",
        session_id: "",
        game_number: 0,
        players: [null, null, null],
        current_player: 0,
        phase: "waiting_for_players",
        trick: null,
        declarer_score: 0,
        opponent_score: 0,
        declarer: null,
        mode: "grand",
        trump_suit: "♣",
        trick_starter: 0,
        trick_winner: null,
        bid_value: 0,
        listener_passed: false,
        speaker_passed: false,
        dealer_passed: false,
        matadors: 0,
        played_hand: false,
        announced_schneider: false,
        announced_schwarz: false,
        current_player_deadline: "",
        forfeited_player: null,
      },
      player_id: undefined,
      hand: [],
      skat: undefined,
      can_play_next: false,
    });
    setMessages([]);
  }, []);

  const addAgent = async (agentId?: string) => {
    if (!state.id) return;
    try {
      await addAIAgent(state.id, agentId);
    } catch (error) {
      console.error("Failed to add AI agent:", error);
    }
  };

  const updatePlayerOnlineStatus = useCallback(
    (playerId: string, isOnline: boolean) => {
      setGameInfo((prev) => ({
        ...prev,
        state: {
          ...prev.state,
          players: prev.state.players.map((p) =>
            p && p.id === playerId ? { ...p, is_online: isOnline } : p,
          ) as [ServerPlayer | null, ServerPlayer | null, ServerPlayer | null],
        },
      }));
    },
    [],
  );

  return {
    // Loading and error states
    isLoading,
    error,
    // State - derived from server state
    gameId: state.id,
    gameCode: state.code,
    sessionId: state.session_id,
    gameNumber: state.game_number,
    player,
    playerPosition,
    playerHand: hand,
    leftPlayer,
    topPlayer,
    players,
    playerCount,
    opponents,
    declarer,
    hand,
    trick: state.trick ?? [],
    trickStarter: state.trick_starter,
    trickWinner: state.trick_winner,
    currentPlayer: state.current_player,
    declarerScore: state.declarer_score,
    opponentScore: state.opponent_score,
    declarerPosition: state.declarer,
    bidValue: state.bid_value,
    listenerPassed: state.listener_passed,
    speakerPassed: state.speaker_passed,
    dealerPassed: state.dealer_passed,
    phase: state.phase,
    playedHand: state.played_hand,
    schneiderAnnounced: state.announced_schneider,
    schwarzAnnounced: state.announced_schwarz,
    gameOver,
    gameMode: state.mode,
    trumpSuit: state.trump_suit,
    currentPlayerDeadline: state.current_player_deadline,
    messages,
    skatCards: skatCards ? [skatCards[0], skatCards[1]] : [],
    hasPickedUpSkat: isSkatExchange && hand.length === 12,
    isNull,
    isBiddingPhase,
    isSkatExchange,
    isDealer,
    isDeclarer,
    isDeclarerChoice,
    isMyTurn,
    isInLobby,
    // Use result data when available
    playerWon: result
      ? result.is_forfeit
        ? // In forfeit games, use forfeited_player to determine winner
          state.forfeited_player !== null &&
          playerPosition !== state.forfeited_player
        : // In normal games, check if player was declarer and if declarer won
          playerPosition !== null && state.declarer !== null
          ? playerPosition === state.declarer
            ? result.declarer_won
            : !result.declarer_won
          : false
      : false,
    isSchneider: result?.is_schneider ?? false,
    isSchwarz: result?.is_schwarz ?? false,
    declarerWon: result?.declarer_won ?? false,
    // Getters
    getRole,
    // Actions
    removeCardFromHand,
    optimisticallyPlayCard,
    undoOptimisticPlayCard,
    setGameInfo,
    addMessage,
    reset,
    addAgent,
    updatePlayerOnlineStatus,
    // Session state
    sessionResults,
    gamesPlayed,
    canPlayNext: canPlayNextFromState,
    setSessionResults,
    setGamesPlayed,
    // Game result (when game is complete)
    result,
  };
}

export type Game = ReturnType<typeof useGame>;
