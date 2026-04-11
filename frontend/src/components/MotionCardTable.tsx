import React, { useState, useEffect, useMemo, useCallback } from "react";
import { AnimatePresence, motion } from "motion/react";
import { useMediaQuery, useTheme } from "@mui/material";
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
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const isTablet = useMediaQuery(theme.breakpoints.down("md"));

  const [selectedCards, setSelectedCards] = useState<CardType[]>([]);

  // Track window size for responsive positioning
  const [windowSize, setWindowSize] = useState({
    width: window.innerWidth,
    height: window.innerHeight,
  });

  // Calculate table size based on window size
  // This matches the CSS table-surface dimensions
  const tableSize = {
    width: isMobile ? windowSize.width : Math.min(1000, windowSize.width - 24),
    height: isMobile ? windowSize.height : windowSize.height - 16,
  };

  const showDeck = game.phase === "dealing";
  const showDealButton = game.phase === "dealing" && game.isDealer;

  // Responsive card dimensions and spacing
  const getCardSpacing = () => {
    if (isMobile) return 25; // Increased overlap on mobile
    if (isTablet) return 35;
    return 40;
  };

  // Unified pile positioning function - returns center position for both CSS and animations
  const getPileAbsolutePosition = (isPlayer: boolean) => {
    // Distance from edges
    const edgeOffset = isMobile ? 120 : isTablet ? 100 : 80;
    const rightOffset = isMobile ? 10 : 25;

    // Pile dimensions
    const pileWidth = 90;
    const pileHeight = 140;

    // Calculate CENTER position of pile relative to table center
    // X: distance from right edge to pile center
    const x = (tableSize.width / 2) - rightOffset - (pileWidth / 2);
    // Y: distance from top/bottom edge to pile center
    const y = (tableSize.height / 2) - edgeOffset - (pileHeight / 2);

    return {
      x: x,
      y: isPlayer ? y : -y,  // Positive for bottom (player), negative for top (opponent)
    };
  };

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
    const spacing = getCardSpacing();
    const totalWidth = total * spacing;
    const startX = -totalWidth / 2;

    // Base y position
    let yPosition = 200;

    // Adjust y position for mobile - bring cards closer to center
    if (isMobile) {
      yPosition = 200;
    } else if (isTablet) {
      yPosition = 200;
    }

    return {
      x: startX + index * spacing + spacing / 2,
      y: yPosition,
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
    const spacing = getCardSpacing();
    const totalWidth = total * spacing;
    const startOffset = -totalWidth / 2;

    // Base positions
    let topY = -200;
    let leftX = -350;

    // Adjust positions for mobile - bring cards closer to center
    if (isMobile) {
      topY = -200;
      leftX = -110;
    } else if (isTablet) {
      topY = -200;
      leftX = -270;
    }

    if (opponent === "top") {
      return {
        x: startOffset + index * spacing + spacing / 2,
        y: topY,
        rotate: 0,
        scale: 1,
      };
    } else {
      // For left opponent, calculate vertical centering
      const totalHeight = total * spacing;
      const startY = -totalHeight / 2;
      return {
        x: leftX,
        y: startY + index * spacing + spacing / 2,
        rotate: 90,
        scale: 1,
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
  const getTrickPosition = (index: number, ntricks: number) => {
    const spacing = isMobile ? 70 : isTablet ? 75 : 80;
    return {
      x: index * spacing - ntricks * spacing * 0.5 + spacing / 2,
      y: 0,
      rotate: 0,
      scale: 1,
    };
  };


  // Determine who is partnered with whom
  const playerIsDeclarer = game.isDeclarer;

  // Get position for player's score pile - always bottom right
  const getPlayerPilePosition = () => {
    const pos = getPileAbsolutePosition(true);
    return { x: pos.x, y: pos.y, rotate: 0, scale: 1 };
  };

  // Get position for opponent's score pile - always top right
  const getOpponentPilePosition = () => {
    const pos = getPileAbsolutePosition(false);
    return { x: pos.x, y: pos.y, rotate: 0, scale: 1 };
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
            className={`opponent-avatar-container top ${game.topPlayer.position === game.currentPlayer ? "current-turn" : ""} ${isMobile ? "mobile" : ""}`}
          >
            <div className="avatar-circle">
              <span>{game.topPlayer.name.charAt(0).toUpperCase()}</span>
            </div>
            <div className="opponent-name">
              {game.topPlayer.name}
              {isMobile && game.declarer === game.topPlayer && " (D)"}
            </div>
            {!isMobile && game.getRole(game.topPlayer.position) && (
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
            className={`opponent-avatar-container left ${game.leftPlayer.position === game.currentPlayer ? "current-turn" : ""} ${isMobile ? "mobile" : ""}`}
          >
            <div className="avatar-circle">
              <span>{game.leftPlayer.name.charAt(0).toUpperCase()}</span>
            </div>
            <div className="opponent-name">
              {game.leftPlayer.name}
              {isMobile && game.declarer === game.leftPlayer && " (D)"}
            </div>
            {!isMobile && game.getRole(game.leftPlayer.position) && (
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
          className={`player-avatar-container ${game.isMyTurn ? "current-turn" : ""} ${isMobile ? "mobile" : ""}`}
        >
          <div className="avatar-circle">
            <span>{game.playerName.charAt(0).toUpperCase()}</span>
          </div>
          <div className="player-name">
            {game.playerName}
            {isMobile && game.isDeclarer && " (D)"}
          </div>
          {!isMobile && game.getRole(game.playerPosition) && (
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

            // Apply declarer choice offset
            const declarerOffset = game.isDeclarerChoice ? 40 : 0;
            const animatePosition = selected
              ? { ...basePosition, y: basePosition.y + declarerOffset - 20 }
              : { ...basePosition, y: basePosition.y + declarerOffset };

            return (
              <Card
                index={index}
                rank={card.rank}
                suit={card.suit}
                key={playerKeys[index]}
                className={`motion-card ${selected ? "selected" : ""}`}
                animate={animatePosition}
                initial={{ ...getDeckPosition() }}
                whileHover={
                  canClickCard
                    ? { y: basePosition.y + declarerOffset - 20 }
                    : undefined
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
          {topOpponentCardsMap((index, key) => {
            const basePosition = getOpponentCardPosition(
              "top",
              index,
              game.topPlayer?.card_count ?? 0,
            );
            // Apply declarer choice offset - move cards up when choosing
            const declarerOffset = game.isDeclarerChoice ? -40 : 0;

            return (
              <Card
                index={index}
                key={key}
                className="motion-card opponent-card"
                animate={{
                  ...basePosition,
                  y: basePosition.y + declarerOffset,
                }}
                initial={{ ...getDeckPosition() }}
                style={{
                  zIndex: 50 + index,
                }}
              />
            );
          })}

          {/* Opponent Cards - Left */}
          {leftOpponentCardsMap((index, key) => {
            const basePosition = getOpponentCardPosition(
              "left",
              index,
              game.leftPlayer?.card_count ?? 0,
            );
            // Apply declarer choice offset - move cards left when choosing
            const declarerOffset = game.isDeclarerChoice ? -30 : 0;

            return (
              <Card
                index={index}
                key={key}
                className="motion-card opponent-card"
                animate={{
                  ...basePosition,
                  x: basePosition.x + declarerOffset,
                }}
                initial={{ ...getDeckPosition() }}
                style={{
                  zIndex: 50 + index,
                }}
              />
            );
          })}

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
                // Determine which pile based on who won and who is declarer
                const trickWinner = game.trickWinner;
                const declarerPosition = game.declarer?.position;

                // Check if the trick winner is the declarer
                const declarerWonTrick = trickWinner === declarerPosition;

                if (declarerWonTrick) {
                  // Declarer won - cards go to declarer's pile
                  if (playerIsDeclarer) {
                    // Player is declarer - go to player pile (bottom)
                    return { ...getPlayerPilePosition() };
                  } else {
                    // Opponent is declarer - go to opponent pile (top)
                    return { ...getOpponentPilePosition() };
                  }
                } else {
                  // Defenders won - cards go to defenders' pile
                  if (playerIsDeclarer) {
                    // Player is declarer - defenders' pile is opponent pile (top)
                    return { ...getOpponentPilePosition() };
                  } else {
                    // Player is defender - defenders' pile is player pile (bottom)
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
            {/* Player's team Score Label - always bottom right */}
            <div
              className={`score-pile-label player-pile`}
              style={{
                position: 'absolute',
                left: '50%',
                top: '50%',
                transform: `translate(calc(-50% + ${getPileAbsolutePosition(true).x}px), calc(-50% + ${getPileAbsolutePosition(true).y}px))`,
              }}
            >
              <span className="pile-label">Player</span>
              <span className="pile-subtitle">
                {playerIsDeclarer ? "DECLARER" : "DEFENDER"}
              </span>
              <span className="pile-score">
                {playerIsDeclarer ? game.declarerScore : game.opponentScore}
              </span>
            </div>

            {/* Opponent's team Score Label - always top right */}
            <div
              className={`score-pile-label opponent-pile`}
              style={{
                position: 'absolute',
                left: '50%',
                top: '50%',
                transform: `translate(calc(-50% + ${getPileAbsolutePosition(false).x}px), calc(-50% + ${getPileAbsolutePosition(false).y}px))`,
              }}
            >
              <span className="pile-label">
                {playerIsDeclarer ? "Opponents" : "Opponent"}
              </span>
              <span className="pile-subtitle">
                {playerIsDeclarer ? "DEFENDERS" : "DECLARER"}
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
