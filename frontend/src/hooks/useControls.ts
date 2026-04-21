import { useState, useCallback, useRef } from "react";
import type { Card as CardType } from "../types";
import { Game } from "./useGame";
import { SkatWebSocket } from "./useWebSocket";

// Convert card to string format: "rank.suit"
const cardToString = (card: CardType): string => {
  return `${card.rank}.${card.suit}`;
};

export function useControls(game: Game, websocket: SkatWebSocket) {
  const [isLoading, setIsLoading] = useState(false);
  const pendingActionRef = useRef<string | null>(null);

  const playCard = useCallback((card: CardType) => {
    if (game.isMyTurn && !game.isBiddingPhase && !isLoading) {
      setIsLoading(true);
      const actionId = websocket.sendMessage(
        "play_card",
        { card: cardToString(card), game_id: game.gameId },
        () => {
          setIsLoading(false);
          pendingActionRef.current = null;
        },
        () => {
          setIsLoading(false);
          pendingActionRef.current = null;
          console.error("Play card action timed out");
        }
      );
      pendingActionRef.current = actionId;
    }
  }, [game.isMyTurn, game.isBiddingPhase, game.gameId, isLoading, websocket]);

  const pickUpSkat = useCallback(() => {
    if (game.isSkatExchange && game.isDeclarer && !game.hasPickedUpSkat && !isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "skat_decision",
        { pickup: true, game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Pick up skat action timed out");
        }
      );
    }
  }, [game.isSkatExchange, game.isDeclarer, game.hasPickedUpSkat, game.gameId, isLoading, websocket]);

  const playHand = useCallback(() => {
    if (game.isSkatExchange && game.isDeclarer && !game.hasPickedUpSkat && !isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "skat_decision",
        { pickup: false, game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Play hand action timed out");
        }
      );
    }
  }, [game.isSkatExchange, game.isDeclarer, game.hasPickedUpSkat, game.gameId, isLoading, websocket]);

  const discardCards = useCallback((cards: CardType[]) => {
    if (!isLoading) {
      setIsLoading(true);
      const cardsStr = cards.map(cardToString).join("-");
      websocket.sendMessage(
        "discard_cards",
        { cards: cardsStr, game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Discard cards action timed out");
        }
      );
    }
  }, [game.gameId, isLoading, websocket]);

  const bid = useCallback((accept: boolean) => {
    if (game.isMyTurn && !isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "bid",
        { accept, game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Bid action timed out");
        }
      );
    } else if (!game.isMyTurn) {
      console.error("Cannot bid, it is not your turn");
    }
  }, [game.isMyTurn, game.gameId, isLoading, websocket]);

  const deal = useCallback(() => {
    if (game.isDealer && !isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "deal",
        { game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Deal action timed out");
        }
      );
    } else if (!game.isDealer) {
      console.error("Cannot deal, you are not the dealer");
    }
  }, [game.isDealer, game.gameId, isLoading, websocket]);

  const declareGame = useCallback((mode: string, trump: string) => {
    if (game.isDeclarer && game.isDeclarerChoice && !isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "choose_game",
        { mode: mode, trump: trump, game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Declare game action timed out");
        }
      );
    } else if (!game.isDeclarer || !game.isDeclarerChoice) {
      console.error("Cannot declare the game, you are not the declarer");
    }
  }, [game.isDeclarer, game.isDeclarerChoice, game.gameId, isLoading, websocket]);

  const playNextGame = useCallback(async () => {
    if (!isLoading) {
      setIsLoading(true);
      websocket.sendMessage(
        "start_next_game",
        { game_id: game.gameId },
        () => setIsLoading(false),
        () => {
          setIsLoading(false);
          console.error("Play next game action timed out");
        }
      );
    }
  }, [game.gameId, isLoading, websocket]);

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
  };
}

export type GameControls = ReturnType<typeof useControls>;
