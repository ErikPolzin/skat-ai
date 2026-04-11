import React, { useState, useEffect, useMemo, useCallback } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Card as CardType } from "../api/games";
import "./MotionCardTable.css";
import { useGameContext } from "../context/GameContext";
import Card from "./Card";
import { GameModeSelector } from "./GameModeSelector";
import { GameLobbyWaiting } from "./GameLobbyWaiting";
import { BiddingControls } from "./BiddingControls";
import { SkatExchange } from "./SkatExchange";

export function MotionCardTable() {
  const game = useGameContext();

  const [selectedCards, setSelectedCards] = useState<CardType[]>([]);

  // Track window size for responsive positioning
  const [windowSize, setWindowSize] = useState({
    width: window.innerWidth,
    height: window.innerHeight,
  });

  const showDeck = game.phase === "dealing";
  const showDealButton = game.phase === "dealing" && game.isDealer;

  // Update window size on resize
  useEffect(() => {
    const handleResize = () => {
      setWindowSize({
        width: window.innerWidth,
        height: window.innerHeight,
      });
    };

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // Helper to check if a card is selected
  const isCardSelected = (card: CardType) => {
    return selectedCards.some(
      (c) => c.rank === card.rank && c.suit === card.suit,
    );
  };

  const handlePlayCard = (card: CardType) => {
    if (game.isSkatExchange && game.isDeclarer && game.hasPickedUpSkat) {
      // In skat exchange phase, clicking cards selects them for discard
      const isSelected = selectedCards.some(
        (c) => c.rank === card.rank && c.suit === card.suit,
      );

      if (isSelected) {
        // Deselect card
        setSelectedCards(
          selectedCards.filter(
            (c) => !(c.rank === card.rank && c.suit === card.suit),
          ),
        );
      } else if (selectedCards.length < 2) {
        // Select card (max 2)
        setSelectedCards([...selectedCards, card]);
      }
    } else {
      game.controls.playCard(card);
    }
  };

  const handleDiscardCards = () => {
    if (selectedCards.length === 2) {
      game.controls.discardCards(selectedCards);
      setSelectedCards([]);
    }
  };

  // Calculate card positions for player hand
  const getPlayerCardPosition = (index: number, total: number) => {
    const spacing = 40;
    const totalWidth = total * spacing;
    const startX = -totalWidth / 2;

    // Shift cards further down when declaring game to make room for selector
    const yPosition = game.isDeclarerChoice ? 220 : 180;

    return {
      x: startX + index * spacing + 20,
      y: yPosition, // Position cards at reasonable distance from center
      rotate: 0,
      scale: 1,
    };
  };

  // Calculate positions for opponent cards
  const getOpponentCardPosition = (
    opponent: "top" | "left",
    index: number,
    total: number,
  ) => {
    const spacing = 40; // More spacing for better visibility
    const totalWidth = total * spacing;
    const startOffset = -totalWidth / 2;

    // Shift cards outward when declaring game
    const topY = game.isDeclarerChoice ? -250 : -220;
    const leftX = game.isDeclarerChoice ? -380 : -350;

    if (opponent === "top") {
      return {
        x: startOffset + index * spacing + 20,
        y: topY, // Symmetrical distance from center
        rotate: 0,
        scale: 1, // Same size as player cards
      };
    } else {
      // For left opponent, calculate vertical centering
      const totalHeight = total * spacing;
      const startY = -totalHeight / 2;
      return {
        x: leftX, // Symmetrical distance from center
        y: startY + index * spacing + 20,
        rotate: 90,
        scale: 1, // Same size as player cards
      };
    }
  };

  // Get deck position (center)
  const getDeckPosition = () => ({
    x: 0,
    y: 0,
    rotate: 0,
    scale: 1,
  });

  // Get game.trick position for a card
  const getTrickPosition = (index: number, ntricks: number) => ({
    x: index * 80 - ntricks * 80 * 0.5 + 40,
    y: 0,
    rotate: 0,
    scale: 1,
  });

  // Calculate pile positions based on table size
  // Using percentages of viewport size for responsive positioning
  const tableWidth = Math.min(1000, windowSize.width - 24); // Match table max-width
  const tableHeight = windowSize.height - 16; // Match table height calculation

  // Position piles at corners with some padding
  const pileOffsetX = tableWidth * 0.5 - 35 - 35;
  const pileOffsetY = tableHeight * 0.5 - 30 - 50;

  // Determine who is partnered with whom
  const playerIsDeclarer = game.isDeclarer;
  const leftOpponentIsDeclarer = game.declarer === game.leftPlayer;

  // Get position for player's score pile based on partnerships
  const getPlayerPilePosition = () => {
    if (playerIsDeclarer) {
      // Player is declarer - pile at bottom right
      return { x: pileOffsetX, y: pileOffsetY, rotate: 0, scale: 1 };
    } else if (leftOpponentIsDeclarer) {
      // Playing with top opponent against left - pile at bottom left
      return { x: -pileOffsetX, y: pileOffsetY, rotate: 0, scale: 1 };
    } else {
      // Playing with left opponent against top - pile at middle right
      return { x: pileOffsetX, y: 0, rotate: 0, scale: 1 };
    }
  };

  // Get position for opponent's score pile based on partnerships
  const getOpponentPilePosition = () => {
    if (playerIsDeclarer) {
      // Player is declarer - opponents' pile at top left
      return { x: -pileOffsetX, y: -(pileOffsetY - 30), rotate: 0, scale: 1 };
    } else if (leftOpponentIsDeclarer) {
      // Left opponent is declarer - their pile at top right
      return { x: pileOffsetX, y: -(pileOffsetY - 30), rotate: 0, scale: 1 };
    } else {
      // Top opponent is declarer - their pile at top left
      return { x: -pileOffsetX, y: -(pileOffsetY - 30), rotate: 0, scale: 1 };
    }
  };

  const topOpponentCardsMap = <T,>(fn: (index: number, key: string) => T) =>
    game.topPlayer &&
    Array.from({ length: game.topPlayer?.card_count ?? 0 }).map((_, index) =>
      fn(index, `card-${game.topPlayer?.position}-${index}`),
    );

  const leftOpponentCardsMap = <T,>(fn: (index: number, key: string) => T) =>
    game.leftPlayer &&
    Array.from({ length: game.leftPlayer.card_count ?? 0 }).map((_, index) =>
      fn(index, `card-${game.leftPlayer?.position}-${index}`),
    );

  // Sort player's hand with trumps on the right
  const sortPlayerHand = useCallback(
    (hand: CardType[]) => {
      const rankOrder = ["7", "8", "9", "Q", "K", "10", "A", "J"];
      const suitOrder = ["♦", "♥", "♠", "♣"];

      return [...hand].sort((a, b) => {
        // In skat, jacks are always trumps
        const aIsJack = a.rank === "J";
        const bIsJack = b.rank === "J";

        // Check if cards are trumps (considering game mode)
        const aIsTrump =
          aIsJack || (game.trumpSuit && a.suit === game.trumpSuit);
        const bIsTrump =
          bIsJack || (game.trumpSuit && b.suit === game.trumpSuit);

        // Trumps go to the right (higher index)
        if (aIsTrump && !bIsTrump) return 1;
        if (!aIsTrump && bIsTrump) return -1;

        // Both trumps or both not trumps
        if (aIsTrump && bIsTrump) {
          // Jacks are higher than suit trumps
          if (aIsJack && !bIsJack) return 1;
          if (!aIsJack && bIsJack) return -1;

          // Both jacks - sort by suit
          if (aIsJack && bIsJack) {
            return suitOrder.indexOf(a.suit) - suitOrder.indexOf(b.suit);
          }

          // Both suit trumps - sort by rank
          return rankOrder.indexOf(a.rank) - rankOrder.indexOf(b.rank);
        }

        // Both non-trumps - sort by suit then rank
        if (a.suit !== b.suit) {
          return suitOrder.indexOf(a.suit) - suitOrder.indexOf(b.suit);
        }
        return rankOrder.indexOf(a.rank) - rankOrder.indexOf(b.rank);
      });
    },
    [game.trumpSuit],
  );

  const sortedPlayerHand = useMemo(
    () => sortPlayerHand(game.playerHand),
    [game.playerHand, sortPlayerHand],
  );

  const trickKeys = useMemo(
    () =>
      game.trick.map((card, index) => {
        const playedByIndex = (game.trickStarter + index) % 3;
        const cardIndex =
          playedByIndex === game.playerPosition
            ? index
            : playedByIndex === game.leftPlayer?.position
              ? (game.leftPlayer.card_count ?? 0)
              : (game.topPlayer?.card_count ?? 0);
        return playedByIndex === game.playerPosition
          ? `player-card-${card.rank}-${card.suit}`
          : `card-${playedByIndex}-${cardIndex}`;
      }),
    [
      game.playerPosition,
      game.trick,
      game.trickStarter,
      game.leftPlayer?.card_count,
      game.leftPlayer?.position,
      game.topPlayer?.card_count,
    ],
  );

  const playerKeys = useMemo(
    () =>
      sortedPlayerHand.map((card) => `player-card-${card.rank}-${card.suit}`),
    [sortedPlayerHand],
  );

  return (
    <div className="motion-card-table">
      <div className="table-surface">
        {/* Center UI: Lobby, Bidding, Skat Exchange, Game Mode Selection, or Game Mode Display */}
        {game.isInLobby ? (
          <GameLobbyWaiting />
        ) : game.isBiddingPhase ? (
          <BiddingControls />
        ) : game.isSkatExchange && game.isDeclarer ? (
          <SkatExchange
            selectedCards={selectedCards}
            onDiscardCards={handleDiscardCards}
          />
        ) : game.isDeclarerChoice && game.isDeclarer ? (
          <GameModeSelector />
        ) : game.isDeclarerChoice && !game.isDeclarer ? (
          <div className="waiting-for-declarer">
            <span>Waiting for declarer to choose game mode...</span>
          </div>
        ) : game.gameMode ? (
          <div className="game-mode-display">
            <span className="mode-value">{game.trumpSuit}</span>
          </div>
        ) : null}

        {/* Top Opponent Avatar */}
        {game.topPlayer && (
          <div
            className={`opponent-avatar-container top ${game.topPlayer.position === game.currentPlayer ? "current-turn" : ""}`}
          >
            <div className="avatar-circle">
              <span>{game.topPlayer.name.charAt(0).toUpperCase()}</span>
            </div>
            <div className="opponent-name">{game.topPlayer.name}</div>
            {game.getRole(game.topPlayer.position) && (
              <div className="player-role">
                {game.getRole(game.topPlayer.position)}
              </div>
            )}
            {/* Speech bubble for player messages */}
            {game.messages
              .filter((msg) => msg.playerPosition === game.topPlayer?.position)
              .slice(-1) // Only show most recent message
              .map((msg) => (
                <div key={msg.id} className="speech-bubble top-bubble">
                  {msg.text}
                </div>
              ))}
          </div>
        )}

        {/* Left Opponent Avatar */}
        {game.leftPlayer && (
          <div
            className={`opponent-avatar-container left ${game.leftPlayer.position === game.currentPlayer ? "current-turn" : ""}`}
          >
            <div className="avatar-circle">
              <span>{game.leftPlayer.name.charAt(0).toUpperCase()}</span>
            </div>
            <div className="opponent-name">{game.leftPlayer.name}</div>
            {game.getRole(game.leftPlayer.position) && (
              <div className="player-role">
                {game.getRole(game.leftPlayer.position)}
              </div>
            )}
            {/* Speech bubble for player messages */}
            {game.messages
              .filter((msg) => msg.playerPosition === game.leftPlayer?.position)
              .slice(-1) // Only show most recent message
              .map((msg) => (
                <div key={msg.id} className="speech-bubble left-bubble">
                  {msg.text}
                </div>
              ))}
          </div>
        )}

        {/* Player Avatar */}
        <div
          className={`player-avatar-container ${game.isMyTurn ? "current-turn" : ""}`}
        >
          <div className="avatar-circle">
            <span>{game.playerName.charAt(0).toUpperCase()}</span>
          </div>
          <div className="player-name">{game.playerName}</div>
          {game.getRole(game.playerPosition) && (
            <div className="player-role">
              {game.getRole(game.playerPosition)}
            </div>
          )}
          {/* Speech bubble for player messages */}
          {game.messages
            .filter((msg) => msg.playerPosition === game.playerPosition)
            .slice(-1) // Only show most recent message
            .map((msg) => (
              <div key={msg.id} className="speech-bubble player-bubble">
                {msg.text}
              </div>
            ))}
        </div>

        <AnimatePresence>
          {/* Deck (shown before and during dealing) */}
          {showDeck && (
            <motion.div
              key="deck"
              className="deck"
              style={{
                position: "absolute",
              }}
              exit={{ opacity: 0 }}
            >
              <img src="/res/back.svg" alt="deck" className="card-back" />
            </motion.div>
          )}

          {/* Deal Button */}
          {showDealButton && (
            <motion.button
              key="deal-button"
              className="deal-button"
              onClick={game.controls.deal}
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              whileTap={{ scale: 0.95 }}
            >
              Deal Cards
            </motion.button>
          )}

          {/* Player Hand - only show after deal started */}
          {sortedPlayerHand.map((card, index) => {
            const selected = isCardSelected(card);
            const basePosition = getPlayerCardPosition(
              index,
              sortedPlayerHand.length,
            );
            const canClickCard =
              (game.phase === "playing" && game.isMyTurn) ||
              (game.isSkatExchange && game.hasPickedUpSkat);

            return (
              <Card
                index={index}
                rank={card.rank}
                suit={card.suit}
                key={playerKeys[index]}
                className={`motion-card ${selected ? "selected" : ""}`}
                animate={
                  selected
                    ? { ...basePosition, y: basePosition.y - 20 }
                    : { ...basePosition }
                }
                initial={{ ...getDeckPosition() }}
                whileHover={
                  canClickCard ? { y: basePosition.y - 20 } : undefined
                }
                onClick={() => {
                  canClickCard && handlePlayCard(card);
                }}
                style={{
                  cursor:
                    game.isBiddingPhase ||
                    (game.isSkatExchange && !game.hasPickedUpSkat)
                      ? "not-allowed"
                      : "pointer",
                  zIndex: 100 + index,
                }}
              />
            );
          })}

          {/* Opponent Cards - Top */}
          {topOpponentCardsMap((index, key) => (
            <Card
              index={index}
              key={key}
              className="motion-card opponent-card"
              animate={{
                ...getOpponentCardPosition(
                  "top",
                  index,
                  game.topPlayer?.card_count ?? 0,
                ),
              }}
              initial={{ ...getDeckPosition() }}
              style={{
                zIndex: 50 + index,
              }}
            />
          ))}

          {/* Opponent Cards - Left */}
          {leftOpponentCardsMap((index, key) => (
            <Card
              index={index}
              key={key}
              className="motion-card opponent-card"
              animate={{
                ...getOpponentCardPosition(
                  "left",
                  index,
                  game.leftPlayer?.card_count ?? 0,
                ),
              }}
              initial={{ ...getDeckPosition() }}
              style={{
                zIndex: 50 + index,
              }}
            />
          ))}

          {/* Trick Cards */}
          {game.trick.map((card, index) => (
            <Card
              key={trickKeys[index]}
              index={index}
              rank={card.rank}
              suit={card.suit}
              className="motion-card"
              animate={{
                ...getTrickPosition(index, game.trick.length),
              }}
              exit={(() => {
                // When player is declarer:
                // - Player pile (bottom right) = declarer pile
                // - Opponent pile (top left) = defenders pile

                // When player is defender:
                // - Player pile (varies) = defenders pile
                // - Opponent pile (varies) = declarer pile

                const trickWinner = game.trickWinner;
                const declarer = game.declarer;

                if (playerIsDeclarer) {
                  // Player is declarer
                  if (trickWinner === declarer) {
                    // Declarer won - go to declarer pile (player's pile at bottom right)
                    return { ...getPlayerPilePosition() };
                  } else {
                    // Defenders won - go to defenders pile (opponent pile at top left)
                    return { ...getOpponentPilePosition() };
                  }
                } else {
                  // Player is defender
                  if (trickWinner === declarer) {
                    // Declarer won - go to declarer pile (opponent pile)
                    return { ...getOpponentPilePosition() };
                  } else {
                    // Defenders won - go to defenders pile (player's pile)
                    return { ...getPlayerPilePosition() };
                  }
                }
              })()}
            />
          ))}
        </AnimatePresence>

        {/* Score Pile Labels - only show during playing phase when declarer is set and there are cards */}
        {game.phase === "playing" && (
          <>
            {/* Player's team Score Label */}
            <div
              className={`score-pile-label`}
              style={{
                ...(() => {
                  const pos = getPlayerPilePosition();
                  return {
                    left: pos.x < 0 ? "25px" : "unset",
                    right: pos.x > 0 ? "25px" : "unset",
                    bottom: pos.y > 0 ? "25px" : "unset",
                    top: pos.y <= 0 ? (pos.y === 0 ? "50%" : "25px") : "unset",
                    transform: pos.y === 0 ? "translateY(-50%)" : "none",
                  };
                })(),
              }}
            >
              <span className="pile-label">
                {playerIsDeclarer ? "Declarer" : "Defenders"}
              </span>
              <span className="pile-score">
                {playerIsDeclarer ? game.declarerScore : game.opponentScore}
              </span>
            </div>

            {/* Opponent's team Score Label */}
            <div
              className={`score-pile-label`}
              style={{
                ...(() => {
                  const pos = getOpponentPilePosition();
                  return {
                    left: pos.x < 0 ? "25px" : "unset",
                    right: pos.x > 0 ? "25px" : "unset",
                    bottom: pos.y > 0 ? "25px" : "unset",
                    top: pos.y <= 0 ? "25px" : "unset",
                  };
                })(),
              }}
            >
              <span className="pile-label">
                {playerIsDeclarer ? "Defenders" : "Declarer"}
              </span>
              <span className="pile-score">
                {playerIsDeclarer ? game.opponentScore : game.declarerScore}
              </span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
