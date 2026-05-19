import type { Card } from "../types";

export const SUITS = ["♣", "♠", "♥", "♦"] as const;
export const SUIT_GAME_BASE_VALUES: Record<string, number> = {
  "♦": 9,
  "♥": 10,
  "♠": 11,
  "♣": 12,
};

const jackOrder = ["♣", "♠", "♥", "♦"];
const suitMatadorRanks = ["A", "10", "K", "Q", "9", "8", "7"];
const suitSortOrder = ["♦", "♥", "♠", "♣"];
const rankOrder = ["7", "8", "9", "Q", "K", "10", "A", "J"];
const rankOrderNull = ["7", "8", "9", "10", "J", "Q", "K", "A"];

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

export function getSuitName(suitEmoji?: string): string {
  if (!suitEmoji) return "";

  switch (suitEmoji) {
    case "♠":
      return "Spades";
    case "♥":
      return "Hearts";
    case "♦":
      return "Diamonds";
    case "♣":
      return "Clubs";
    default:
      return suitEmoji;
  }
}

export function getGameModeDisplay(gameMode: string, trumpSuit?: string): string {
  switch (gameMode.toLowerCase()) {
    case "null":
      return "Null";
    case "grand":
      return "Grand";
    case "suit":
      return trumpSuit ? getSuitName(trumpSuit) : "Suit";
    default:
      return gameMode;
  }
}

export function getGameModeSVG(gameMode: string, trumpSuit?: string): string {
  return `/res/${getGameModeDisplay(gameMode, trumpSuit)}.svg`;
}

export function isSameCard(a: Card, b: Card): boolean {
  return a.rank === b.rank && a.suit === b.suit;
}

export function compareCardsForHand(
  a: Card,
  b: Card,
  gameMode: string,
  trumpSuit?: string,
): number {
  if (gameMode === "null") {
    if (a.suit !== b.suit) {
      return suitSortOrder.indexOf(a.suit) - suitSortOrder.indexOf(b.suit);
    }
    return rankOrderNull.indexOf(a.rank) - rankOrderNull.indexOf(b.rank);
  }

  const aIsJack = a.rank === "J";
  const bIsJack = b.rank === "J";
  const aIsTrump = aIsJack || (trumpSuit ? a.suit === trumpSuit : false);
  const bIsTrump = bIsJack || (trumpSuit ? b.suit === trumpSuit : false);

  if (aIsTrump && !bIsTrump) return 1;
  if (!aIsTrump && bIsTrump) return -1;

  if (aIsTrump && bIsTrump) {
    if (aIsJack && !bIsJack) return 1;
    if (!aIsJack && bIsJack) return -1;
    if (aIsJack && bIsJack) {
      return suitSortOrder.indexOf(a.suit) - suitSortOrder.indexOf(b.suit);
    }
    return rankOrder.indexOf(a.rank) - rankOrder.indexOf(b.rank);
  }

  if (a.suit !== b.suit) {
    return suitSortOrder.indexOf(a.suit) - suitSortOrder.indexOf(b.suit);
  }
  return rankOrder.indexOf(a.rank) - rankOrder.indexOf(b.rank);
}

function getMatadorOrder(mode: string, trumpSuit: string): Card[] {
  const topTrumps = jackOrder.map((suit) => ({ suit, rank: "J" }));
  if (mode !== "suit") {
    return topTrumps;
  }
  return [
    ...topTrumps,
    ...suitMatadorRanks.map((rank) => ({ suit: trumpSuit, rank })),
  ];
}

export function countMatadorsWithSign(
  cards: Card[],
  mode: string,
  trumpSuit: string,
): number {
  const matadorOrder = getMatadorOrder(mode, trumpSuit);
  const hasClubJack = cards.some((card) => isSameCard(card, matadorOrder[0]));

  let matadors = 0;
  if (hasClubJack) {
    for (const matador of matadorOrder) {
      if (cards.some((card) => isSameCard(card, matador))) {
        matadors++;
      } else {
        break;
      }
    }
    return matadors;
  }

  for (const matador of matadorOrder) {
    if (!cards.some((card) => isSameCard(card, matador))) {
      matadors++;
    } else {
      break;
    }
  }
  return -matadors;
}

export function canAnnounceSchneider(mode: string, playedHand: boolean): boolean {
  return playedHand && mode !== "null";
}

export function canAnnounceSchwarz(
  mode: string,
  playedHand: boolean,
  announceSchneider: boolean,
): boolean {
  return canAnnounceSchneider(mode, playedHand) && announceSchneider;
}

export function getNullGameValue(playedHand: boolean): number {
  return playedHand ? 35 : 23;
}

export function calculatePotentialGameValue({
  mode,
  trumpSuit,
  hand,
  skatCards,
  playedHand,
  announcedSchneider,
  announcedSchwarz,
}: {
  mode: string;
  trumpSuit: string;
  hand: Card[];
  skatCards: Card[];
  playedHand: boolean;
  announcedSchneider: boolean;
  announcedSchwarz: boolean;
}): number {
  if (mode === "null") {
    return getNullGameValue(playedHand);
  }

  const baseValue = mode === "grand" ? 24 : SUIT_GAME_BASE_VALUES[trumpSuit];
  if (!baseValue) {
    return 0;
  }

  const matadors = Math.abs(
    countMatadorsWithSign([...hand, ...skatCards], mode, trumpSuit),
  );
  let multiplier = 1 + matadors;

  if (playedHand) multiplier += 1;
  if (canAnnounceSchneider(mode, playedHand) && announcedSchneider) {
    multiplier += 1;
  }
  if (canAnnounceSchwarz(mode, playedHand, announcedSchneider) && announcedSchwarz) {
    multiplier += 1;
  }

  return baseValue * multiplier;
}
