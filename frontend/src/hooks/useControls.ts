import { useState, useCallback } from "react";
import type { Card as CardType } from "../types";
import { type Game } from "./useGame";
import { type SkatWebSocket } from "./useWebSocket";
import { useProfileStore } from "../stores/profileStore";
import { useSnackbarStore } from "../stores/snackbarStore";
import * as api from "../api/games";

// Convert card to string format: "rank.suit"
const cardToString = (card: CardType): string => {
  return `${card.rank}.${card.suit}`;
};

export function useControls(game: Game, websocket: SkatWebSocket) {
  const [isLoading, setIsLoading] = useState(false);
  const playerId = useProfileStore((state) => state.playerId);
  const showSnackbar = useSnackbarStore((state) => state.showSnackbar);

  const playCard = useCallback(
    async (card: CardType) => {
      if (game.isMyTurn && !game.isBiddingPhase && !isLoading && playerId) {
        // Optimistically update the UI before server responds
        game.optimisticallyPlayCard(card);

        setIsLoading(true);
        try {
          await api.playCard(game.gameId, playerId, cardToString(card));
        } catch (error) {
          console.error("Play card action failed:", error);
          // Undo the optimistic update
          game.undoOptimisticPlayCard(card);
          showSnackbar("Failed to play card", "error");
        } finally {
          setIsLoading(false);
        }
      }
    },
    [game, isLoading, playerId, showSnackbar],
  );

  const pickUpSkat = useCallback(async () => {
    if (
      game.isSkatExchange &&
      game.isDeclarer &&
      !game.hasPickedUpSkat &&
      !isLoading &&
      playerId
    ) {
      setIsLoading(true);
      try {
        await api.skatDecision(game.gameId, playerId, true);
      } catch (error) {
        console.error("Pick up skat action failed:", error);
      } finally {
        setIsLoading(false);
      }
    }
  }, [
    game.isSkatExchange,
    game.isDeclarer,
    game.hasPickedUpSkat,
    game.gameId,
    isLoading,
    playerId,
  ]);

  const playHand = useCallback(async () => {
    if (
      game.isSkatExchange &&
      game.isDeclarer &&
      !game.hasPickedUpSkat &&
      !isLoading &&
      playerId
    ) {
      setIsLoading(true);
      try {
        await api.skatDecision(game.gameId, playerId, false);
      } catch (error) {
        console.error("Play hand action failed:", error);
      } finally {
        setIsLoading(false);
      }
    }
  }, [
    game.isSkatExchange,
    game.isDeclarer,
    game.hasPickedUpSkat,
    game.gameId,
    isLoading,
    playerId,
  ]);

  const discardCards = useCallback(
    async (cards: CardType[]) => {
      if (!isLoading && playerId) {
        setIsLoading(true);
        const cardsStr = cards.map(cardToString).join("-");
        try {
          await api.discardCards(game.gameId, playerId, cardsStr);
        } catch (error) {
          console.error("Discard cards action failed:", error);
        } finally {
          setIsLoading(false);
        }
      }
    },
    [game.gameId, isLoading, playerId],
  );

  const bid = useCallback(
    async (accept: boolean) => {
      if (game.isMyTurn && !isLoading && playerId) {
        setIsLoading(true);
        try {
          await api.bid(game.gameId, playerId, accept);
        } catch (error) {
          console.error("Bid action failed:", error);
        } finally {
          setIsLoading(false);
        }
      } else if (!game.isMyTurn) {
        console.error("Cannot bid, it is not your turn");
      }
    },
    [game.isMyTurn, game.gameId, isLoading, playerId],
  );

  const deal = useCallback(async () => {
    if (game.isDealer && !isLoading && playerId) {
      setIsLoading(true);
      try {
        await api.dealCards(game.gameId, playerId);
      } catch (error) {
        console.error("Deal action failed:", error);
      } finally {
        setIsLoading(false);
      }
    } else if (!game.isDealer) {
      console.error("Cannot deal, you are not the dealer");
    }
  }, [game.isDealer, game.gameId, isLoading, playerId]);

  const declareGame = useCallback(
    async (
      mode: string,
      trump: string,
      announceSchneider: boolean = false,
      announceSchwarz: boolean = false,
    ) => {
      if (game.isDeclarer && game.isDeclarerChoice && !isLoading && playerId) {
        setIsLoading(true);
        try {
          await api.chooseGame(
            game.gameId,
            playerId,
            mode,
            trump,
            announceSchneider,
            announceSchwarz,
          );
        } catch (error) {
          console.error("Declare game action failed:", error);
        } finally {
          setIsLoading(false);
        }
      } else if (!game.isDeclarer || !game.isDeclarerChoice) {
        console.error("Cannot declare the game, you are not the declarer");
      }
    },
    [game.isDeclarer, game.isDeclarerChoice, game.gameId, isLoading, playerId],
  );

  const playNextGame = useCallback(async () => {
    if (!isLoading && playerId) {
      setIsLoading(true);
      try {
        await api.readyForNextGame(game.gameId, playerId);
      } catch (error) {
        console.error("Ready for next game action failed:", error);
        showSnackbar("Failed to mark ready for next game", "error");
      } finally {
        setIsLoading(false);
      }
    }
  }, [game.gameId, isLoading, playerId, showSnackbar]);

  return {
    playCard,
    pickUpSkat,
    playHand,
    discardCards,
    bid,
    deal,
    declareGame,
    playNextGame,
    isLoading,
    isConnected: websocket.isConnected,
    reconnectCountdown: websocket.reconnectCountdown,
  };
}

export type GameControls = ReturnType<typeof useControls>;
