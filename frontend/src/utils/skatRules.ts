import type { Card } from "../types";

/**
 * Determines if a card can be legally played according to Skat rules
 */
export function canPlayCard(
  card: Card,
  hand: Card[],
  trick: Card[],
  gameMode: string,
  trumpSuit: string,
): boolean {
  // If no cards in trick yet, any card can be played
  if (trick.length === 0) {
    return true;
  }

  const ledCard = trick[0];
  const ledSuit = getEffectiveSuit(ledCard, gameMode, trumpSuit);

  // Check if player must follow suit
  const cardSuit = getEffectiveSuit(card, gameMode, trumpSuit);

  // If card matches led suit, it can be played
  if (cardSuit === ledSuit) {
    return true;
  }

  // If card doesn't match, check if player has any cards that do match
  const hasMatchingSuit = hand.some(
    (c) => getEffectiveSuit(c, gameMode, trumpSuit) === ledSuit,
  );

  // Can only play non-matching card if player has no matching cards
  return !hasMatchingSuit;
}

/**
 * Gets the effective suit of a card based on game mode
 * In Grand and Suit games, Jacks are always trumps
 * In Suit games, cards of trump suit are also trumps
 */
function getEffectiveSuit(
  card: Card,
  gameMode: string,
  trumpSuit: string,
): string {
  const isJack = card.rank === "J";

  if (gameMode === "null") {
    // In null games, no trumps
    return card.suit;
  }

  if (gameMode === "grand") {
    // In grand, only jacks are trumps
    return isJack ? "trump" : card.suit;
  }

  if (gameMode === "suit") {
    // In suit games, jacks and trump suit cards are trumps
    if (isJack || card.suit === trumpSuit) {
      return "trump";
    }
    return card.suit;
  }

  // Default: no trumps
  return card.suit;
}
