import { useState, useCallback, useRef, useEffect, useMemo } from "react";
import type { Card, Player } from "../types";
import {
  addAIAgent,
  fetchGameState,
  type GameState as APIGameState,
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
  const [state, setState] = useState<APIGameState>({
    id: "",
    code: "",
    players: [],
    current_player: -1,
    phase: "lobby",
    trick: [],
    declarer_score: 0,
    opponent_score: 0,
    declarer: -1,
    game_over: false,
    game_mode: undefined,
    trump_suit: undefined,
    valid_bids: [],
    hand: [],
    declarer_pile_count: 0,
    opponent_pile_count: 0,
    trick_starter: 0,
    trick_winner: -1,
    declarer_tricks: 0,
    skat_cards: undefined,
    has_picked_up_skat: false,
    bid_value: undefined,
  });

  // Derive player info from players array
  const myPlayer = useMemo(
    () => state.players.find((p) => p?.player_id === playerId),
    [state.players, playerId],
  );

  const playerPosition = myPlayer?.position ?? null;
  const isBiddingPhase = state.phase === "bidding";
  const isSkatExchange = state.phase === "skat_exchange";
  const isDeclarerChoice = state.phase === "declarer_choice";
  const isInLobby = state.phase === "lobby";
  const isDeclarer = playerPosition === state.declarer;
  const isDealer = playerPosition === 0;
  const isNull = state.game_mode === "Null";

  // Check if it's my turn by comparing position
  const isMyTurn =
    state.current_player !== undefined &&
    state.current_player !== null &&
    state.current_player >= 0 &&
    state.current_player === playerPosition;

  // UI-only state
  const [messages, setMessages] = useState<GameMessage[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const messageIdCounter = useRef(0);

  // Track if we've already fetched for this gameId to prevent duplicates
  const lastFetchedGameId = useRef<string | null>(null);

  const opponents = useMemo(
    () =>
      state.players.filter(
        (p) => p && p.position !== state.declarer,
      ) as Player[],
    [state.declarer, state.players],
  );

  const declarer = useMemo(
    () => state.players.find((p) => p && p.position === state.declarer),
    [state.declarer, state.players],
  );

  const leftPlayer = useMemo(
    () =>
      playerPosition != null
        ? state.players[(playerPosition + 1) % state.players.length]
        : undefined,
    [playerPosition, state.players],
  );

  const topPlayer = useMemo(
    () =>
      playerPosition != null
        ? state.players[(playerPosition + 2) % state.players.length]
        : undefined,
    [playerPosition, state.players],
  );

  const playerCount = useMemo(
    () => state.players.filter((p) => !!p).length,
    [state.players],
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
      setIsLoading(false);
      setError("No game ID provided");
      return;
    }

    // Skip if we've already fetched this game
    if (lastFetchedGameId.current === gameId) return;

    const loadGameState = async () => {
      setIsLoading(true);
      setError(null);
      try {
        const data = await fetchGameState(gameId, playerId);
        setState(data);
        lastFetchedGameId.current = gameId;
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
    setState((prev) => ({
      ...prev,
      hand: (prev.hand || []).filter(
        (c) => !(c.rank === card.rank && c.suit === card.suit),
      ),
    }));
  }, []);

  const setGameState = useCallback(
    (updates: any) => {
      const changes: Partial<APIGameState> = {};
      Object.entries(updates).forEach(([k, v]) => {
        if (Object.hasOwn(updates, k)) {
          changes[k as keyof APIGameState] = v as any;
        }
      });
      setState((prev) => ({
        ...prev,
        ...changes,
      }));
      // Handle special case: removing card from hand when played
      if (updates.last_card_played && !updates.hand) {
        // Only remove from hand if we didn't get a new hand update
        removeCardFromHand(updates.last_card_played);
      }
    },
    [removeCardFromHand],
  );

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
    setState({
      id: "",
      code: "",
      players: [],
      current_player: -1,
      phase: "lobby",
      trick: [],
      declarer_score: 0,
      opponent_score: 0,
      declarer: -1,
      game_over: false,
      game_mode: undefined,
      trump_suit: undefined,
      valid_bids: [],
      hand: [],
      declarer_pile_count: 0,
      opponent_pile_count: 0,
      trick_starter: 0,
      trick_winner: -1,
      declarer_tricks: 0,
      skat_cards: undefined,
      has_picked_up_skat: false,
    });
    setMessages([]);
  }, []);

  const addAgent = async (agentType: string) => {
    if (!state.id) return;
    try {
      await addAIAgent(state.id, agentType);
    } catch (error) {
      console.error("Failed to add AI agent:", error);
    }
  };

  return {
    // Loading and error states
    isLoading,
    error,
    // State - derived from server state
    gameId: state.id,
    gameCode: state.code,
    playerId,
    playerName: myPlayer?.name || "",
    playerPosition,
    playerHand: state.hand ?? [],
    leftPlayer,
    topPlayer,
    players: state.players,
    playerCount,
    opponents,
    declarer,
    hand: state.hand || [],
    trick: state.trick,
    trickStarter: state.trick_starter || 0,
    trickWinner: state.trick_winner,
    currentPlayer: state.current_player,
    declarerScore: state.declarer_score,
    opponentScore: state.opponent_score,
    declarerPosition: state.declarer,
    bidValue: state.bid_value,
    phase: state.phase,
    gameOver: state.game_over,
    gameMode: state.game_mode,
    trumpSuit: state.trump_suit,
    messages,
    validBids: state.valid_bids || [],
    declarerPileCount: state.declarer_pile_count || 0,
    opponentPileCount: state.opponent_pile_count || 0,
    declarerTricks: state.declarer_tricks || 0,
    skatCards: state.skat_cards ?? [],
    hasPickedUpSkat: state.has_picked_up_skat || false,
    isNull,
    isBiddingPhase,
    isSkatExchange,
    isDealer,
    isDeclarer,
    isDeclarerChoice,
    isMyTurn,
    isInLobby,
    playerWon:
      state.game_over && playerPosition === state.declarer
        ? isNull
          ? state.declarer_tricks === 0
          : state.declarer_score >= 61
        : isNull
          ? (state.declarer_tricks ?? 0) > 0
          : state.declarer_score < 61,
    isSchneider:
      state.game_over &&
      !isNull &&
      ((state.declarer_score >= 61 && state.opponent_score < 30) ||
        (state.declarer_score < 61 && state.declarer_score < 30)),
    isSchwarz:
      state.game_over &&
      isNull &&
      (state.declarer_score === 120 ||
        state.opponent_score === 120 ||
        state.declarer_score === 0 ||
        state.opponent_score === 0),
    // Getters
    getRole,
    // Actions
    removeCardFromHand,
    setGameState,
    addMessage,
    reset,
    addAgent,
  };
}

export type Game = ReturnType<typeof useGame>;
