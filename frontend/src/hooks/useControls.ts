import type { Card as CardType } from "../types";
import { Game } from "./useGame";
import { SkatWebSocket } from "./useWebSocket";

export function useControls(game: Game, websocket: SkatWebSocket) {
  const playCard = (card: CardType) => {
    if (game.isMyTurn && !game.isBiddingPhase) {
      // Normal card play
      websocket.sendMessage("play_card", { card, game_id: game.gameId });
    }
  };

  const pickUpSkat = () => {
    if (game.isSkatExchange && game.isDeclarer && !game.hasPickedUpSkat) {
      websocket.sendMessage("skat_decision", {
        pickup: true,
        game_id: game.gameId,
      });
      // The backend will add the skat cards to our hand
      // and send has_picked_up_skat: true in the state update
    }
  };

  const playHand = () => {
    if (game.isSkatExchange && game.isDeclarer && !game.hasPickedUpSkat) {
      websocket.sendMessage("skat_decision", {
        pickup: false,
        game_id: game.gameId,
      });
    }
  };

  const discardCards = (cards: CardType[]) => {
    websocket.sendMessage("discard_cards", { cards, game_id: game.gameId });
    // The backend will move to DeclarerChoice phase and reset has_picked_up_skat
  };

  const bid = (bid: string) => {
    if (game.isMyTurn) {
      websocket.sendMessage("bid", { bid, game_id: game.gameId });
    } else {
      console.error("Cannot bid, it is not your turn");
    }
  };

  const deal = () => {
    if (game.isDealer) {
      websocket.sendMessage("deal", { game_id: game.gameId });
    } else {
      console.error("Cannot deal, you are not the dealer");
    }
  };

  const declareGame = (mode: string, trump: string) => {
    if (game.isDeclarer && game.isDeclarerChoice) {
      websocket.sendMessage("choose_game", {
        mode: mode,
        trump: trump,
        game_id: game.gameId,
      });
    } else {
      console.error("Cannot declare the game, you are not the declarer");
    }
  };

  return {
    playCard,
    pickUpSkat,
    playHand,
    discardCards,
    bid,
    deal,
    declareGame,
  };
}

export type GameControls = ReturnType<typeof useControls>;
