import { useState, useCallback, useRef, useEffect, useMemo } from "react";
import type { Card, SessionGameResult } from "../types";
import {
  addAIAgent,
  fetchGameState,
  getSessionResults,
  type GameInfo,
  type Player,
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
      current_player: -1,
      phase: "waiting_for_players",
      trick: null,
      declarer_score: 0,
      opponent_score: 0,
      declarer: -1,
      mode: "",
      trump_suit: "",
      trick_starter: 0,
      trick_winner: -1,
      game_value: 0,
      bid_value: 0,
      listener_passed: false,
      speaker_passed: false,
      dealer_passed: false,
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

  // Transform server players (with position as index) to client players (with position field)
  const players = useMemo<(Player | null)[]>(
    () =>
      state.players.map((p, index) =>
        p ? { ...p, position: index } : null
      ),
    [state.players]
  );

  // Derive player info from players array
  const myPlayer = useMemo(
    () => players.find((p) => p?.id === playerId),
    [players, playerId],
  );

  const playerPosition = myPlayer?.position ?? null;
  const isBiddingPhase = state.phase === "bidding";
  const isSkatExchange = state.phase === "skat_exchange";
  const isDeclarerChoice = state.phase === "declarer_choice";
  const isInLobby = state.phase === "waiting_for_players";
  const isDeclarer = playerPosition === state.declarer;
  const isDealer = playerPosition === 0;
  const isNull = state.mode === "Null";
  const gameOver = state.phase === "complete";

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
  const [sessionResults, setSessionResults] = useState<SessionGameResult[]>([]);
  const [gamesPlayed, setGamesPlayed] = useState(0);
  const messageIdCounter = useRef(0);

  const opponents = useMemo(
    () =>
      players.filter(
        (p) => p && p.position !== state.declarer,
      ) as Player[],
    [state.declarer, players],
  );

  const declarer = useMemo(
    () => players.find((p) => p && p.position === state.declarer),
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
        current_player: -1,
        phase: "waiting_for_players",
        trick: null,
        declarer_score: 0,
        opponent_score: 0,
        declarer: -1,
        mode: "",
        trump_suit: "",
        trick_starter: 0,
        trick_winner: -1,
        game_value: 0,
        bid_value: 0,
        listener_passed: false,
        speaker_passed: false,
        dealer_passed: false,
      },
      player_id: undefined,
      hand: [],
      skat: undefined,
      can_play_next: false,
    });
    setMessages([]);
  }, []);

  const addAgent = async () => {
    if (!state.id) return;
    try {
      await addAIAgent(state.id);
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
    sessionId: state.session_id,
    gameNumber: state.game_number,
    playerId,
    playerName: myPlayer?.name || "",
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
    phase: state.phase,
    gameOver,
    gameMode: state.mode,
    trumpSuit: state.trump_suit,
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
    playerWon:
      gameOver && playerPosition === state.declarer
        ? isNull
          ? state.declarer_score === 0
          : state.declarer_score >= 61
        : isNull
          ? state.declarer_score > 0
          : state.declarer_score < 61,
    isSchneider:
      gameOver &&
      !isNull &&
      ((state.declarer_score >= 61 && state.opponent_score < 30) ||
        (state.declarer_score < 61 && state.declarer_score < 30)),
    isSchwarz:
      gameOver &&
      !isNull &&
      (state.declarer_score === 120 ||
        state.opponent_score === 120 ||
        state.declarer_score === 0 ||
        state.opponent_score === 0),
    // Getters
    getRole,
    // Actions
    removeCardFromHand,
    setGameInfo,
    addMessage,
    reset,
    addAgent,
    // Session state
    sessionResults,
    gamesPlayed,
    canPlayNext: canPlayNextFromState,
    setSessionResults,
    setGamesPlayed,
  };
}

export type Game = ReturnType<typeof useGame>;
