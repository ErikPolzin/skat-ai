import React, {
  useState,
  useEffect,
  useMemo,
  useCallback,
  useRef,
} from "react";
import { AnimatePresence, motion } from "motion/react";
import { Button, useMediaQuery, useTheme } from "@mui/material";
import SignalWifiOffIcon from "@mui/icons-material/SignalWifiOff";
import { Card as CardType } from "../api/games";
import "./MotionCardTable.css";
import { useGameContext } from "../context/GameContext";
import Card from "./Card";
import { GameModeSelector } from "./GameModeSelector";
import { GameLobbyWaiting } from "./GameLobbyWaiting";
import { BiddingControls } from "./BiddingControls";
import { SkatExchange } from "./SkatExchange";
import { GameOverScreen } from "./GameOverScreen";
import { canPlayCard } from "../utils/skatRules";

// Helper function to convert suit emoji to word
function getSuitName(suitEmoji?: string): string {
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

// Helper function to format game mode display
function getGameModeDisplay(gameMode: string, trumpSuit?: string): string {
  switch (gameMode.toLowerCase()) {
    case "null":
      return "Null";
    case "grand":
      return "Grand";
    case "suit":
      if (trumpSuit) {
        // Convert emoji to suit name
        const suitName = getSuitName(trumpSuit);
        return suitName;
      }
      return "Suit";
    default:
      return gameMode;
  }
}

export function MotionCardTable() {
  const game = useGameContext();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const isTablet = useMediaQuery(theme.breakpoints.down("md"));

  const [selectedCards, setSelectedCards] = useState<CardType[]>([]);

  // Track whether cards should animate from deck (true) or spread from left (false)
  const [shouldAnimateFromDeck, setShouldAnimateFromDeck] = useState(false);
  const hasInitializedRef = useRef(false);

  // Track window size for responsive positioning
  const [windowSize, setWindowSize] = useState({
    width: window.innerWidth,
    height: window.innerHeight,
  });

  // Calculate table size based on window size
  // This matches the CSS table-surface dimensions
  const showSessionResults = isMobile && game.playerCount === 3;
  const tableSize = {
    width: isMobile ? windowSize.width : Math.min(1000, windowSize.width - 24),
    height: isMobile ? windowSize.height - 30 : windowSize.height - 16,
  };

  const showDeck = game.phase === "dealing";
  const showDealButton = game.phase === "dealing" && game.isDealer;

  // Card dimensions - single source of truth
  const CARD_HEIGHT = isMobile ? 120 : isTablet ? 100 : 110;
  const CARD_WIDTH = CARD_HEIGHT * (5 / 7);

  // Responsive card dimensions and spacing
  const getCardSpacing = () => {
    if (isMobile) return 25; // Increased overlap on mobile
    if (isTablet) return 35;
    return 40;
  };

  const getCardPadding = () => {
    if (isMobile) return 50; // Decreased padding on mobile
    if (isTablet) return 75;
    return 100;
  };

  // Unified pile positioning function - returns center position for both CSS and animations
  const getPileAbsolutePosition = (isPlayer: boolean) => {
    // Distance from edges
    const rightOffset = isMobile ? 10 : 100;

    // Calculate CENTER position of pile relative to table center
    // X: distance from right edge to pile center
    const x = tableSize.width / 2 - rightOffset - CARD_WIDTH / 2;
    // Y: distance from top/bottom edge to pile center
    const y = tableSize.height / 2 - getCardPadding() - CARD_HEIGHT / 2;

    return {
      x: x,
      y: isPlayer ? y : -y, // Positive for bottom (player), negative for top (opponent)
    };
  };

  // Determine animation behavior: animate from deck when dealing, spread from left on reload
  useEffect(() => {
    if (!hasInitializedRef.current) {
      // On first render: animate from deck only if we're actively dealing or have no cards
      const animateFromDeck =
        game.phase === "dealing" || game.playerHand.length === 0;
      setShouldAnimateFromDeck(animateFromDeck);
      hasInitializedRef.current = true;
    } else if (game.phase === "dealing") {
      // When entering dealing phase, enable deck animation
      setShouldAnimateFromDeck(true);
    }
  }, [game.phase, game.playerHand.length]);

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
    // Don't allow actions when disconnected or loading
    if (!game.controls.isConnected || game.controls.isLoading) {
      return;
    }

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
      // Play card
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
    const yPosition = tableSize.height / 2 - CARD_HEIGHT / 2;
    // Shift cards up slightly on mobile
    const yOffset = isMobile ? -40 : 0;

    return {
      x: startX + index * spacing + spacing / 2,
      y: yPosition - getCardPadding() + yOffset,
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
    const totalHeight = total * spacing;

    if (opponent === "top") {
      const startX = -totalWidth / 2;
      const yPosition = -tableSize.height / 2 + CARD_HEIGHT / 2;
      return {
        x: startX + index * spacing + spacing / 2,
        y: yPosition + getCardPadding(),
        rotate: 0,
        scale: 1,
      };
    } else {
      // For left opponent, calculate vertical centering
      const startY = -totalHeight / 2;
      const xPosition = -tableSize.width / 2 + CARD_HEIGHT / 2;
      return {
        x: Math.min(xPosition + getCardPadding(), -190),
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

  // Get initial position for player cards (either from deck or spreading from left)
  const getPlayerCardInitialPosition = (
    index: number,
    basePosition: { x: number; y: number; rotate: number; scale: number },
    declarerOffset: number,
  ) => {
    if (shouldAnimateFromDeck) {
      return getDeckPosition();
    }
    // Spread from left: cards start stacked on the left
    return {
      ...basePosition,
      x: basePosition.x - index * 20,
      y: basePosition.y + declarerOffset,
      rotate: 0,
      scale: 1,
    };
  };

  // Get initial position for opponent cards
  const getOpponentCardInitialPosition = (
    index: number,
    basePosition: { x: number; y: number; rotate: number; scale: number },
    declarerOffset: number,
    orientation: "horizontal" | "vertical",
  ) => {
    if (shouldAnimateFromDeck) {
      return getDeckPosition();
    }
    // Spread from left for horizontal, from bottom for vertical
    if (orientation === "horizontal") {
      return {
        ...basePosition,
        x: basePosition.x - index * 20,
        y: basePosition.y + declarerOffset,
        rotate: 0,
        scale: 1,
      };
    } else {
      return {
        ...basePosition,
        x: basePosition.x + declarerOffset,
        y: basePosition.y + index * 20,
        rotate: 90,
        scale: 1,
      };
    }
  };

  // Get game.trick position for a card
  const getTrickPosition = (index: number, ntricks: number) => {
    const spacing = CARD_WIDTH + 10;
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

  // CSS variable overrides - use the card dimensions as source of truth
  const cardTableStyle = {
    "--card-width": `${CARD_WIDTH}px`,
    "--card-height": `${CARD_HEIGHT}px`,
  } as React.CSSProperties;

  return (
    <div className="motion-card-table" style={cardTableStyle}>
      <div
        className={`table-surface ${showSessionResults ? "with-session-bar" : ""}`}
      >
        {/* Center UI: Disconnected indicator takes priority over everything */}
        {!game.controls.isConnected ? (
          <div
            style={{
              position: "absolute",
              top: "50%",
              left: "50%",
              transform: "translate(-50%, -50%)",
              zIndex: 2000,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: "16px",
            }}
          >
            <SignalWifiOffIcon
              sx={{
                fontSize: 60,
                color: "warning.main",
                filter: "drop-shadow(0 2px 4px rgba(0, 0, 0, 0.2))",
              }}
            />
            <span
              style={{
                fontSize: "20px",
                fontWeight: "bold",
                color: "#ed6c02",
                textShadow: "0 1px 2px rgba(0, 0, 0, 0.1)",
              }}
            >
              Disconnected from server
            </span>
            <span
              style={{
                fontSize: "14px",
                color: "#d4d4d4",
              }}
            >
              {game.controls.reconnectCountdown !== null
                ? `Reconnecting in ${Math.ceil(game.controls.reconnectCountdown)}s...`
                : "Attempting to reconnect..."}
            </span>
          </div>
        ) : game.isInLobby ? (
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
        ) : game.gameOver ? (
          <GameOverScreen />
        ) : game.gameMode ? (
          <div className="game-mode-display">
            <span className="mode-value">{game.trumpSuit}</span>
            <span className="mode-title">
              {getGameModeDisplay(game.gameMode, game.trumpSuit)}
            </span>
          </div>
        ) : null}

        {/* Top Opponent Avatar */}
        {game.topPlayer && (
          <div
            className={`opponent-avatar-container top ${game.topPlayer.position === game.currentPlayer ? "current-turn" : ""} ${isMobile ? "mobile" : ""}`}
          >
            <div
              className={`avatar-circle ${!game.controls.isConnected || !game.topPlayer.is_online ? "offline" : ""}`}
            >
              {game.topPlayer.profile_icon ? (
                <img
                  src={game.topPlayer.profile_icon}
                  alt={game.topPlayer.name}
                />
              ) : (
                <span>{game.topPlayer.name.charAt(0).toUpperCase()}</span>
              )}
            </div>
            <div className="avatar-info">
              <div
                className={`opponent-name ${!game.controls.isConnected || !game.topPlayer.is_online ? "offline" : ""}`}
              >
                {game.topPlayer.name}
                {game.declarer === game.topPlayer && " (D)"}
              </div>
              {game.getRole(game.topPlayer.position) && (
                <div className="player-role">
                  {game.getRole(game.topPlayer.position)}
                </div>
              )}
            </div>
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
            <div
              className={`avatar-circle ${!game.controls.isConnected || !game.leftPlayer.is_online ? "offline" : ""}`}
            >
              {game.leftPlayer.profile_icon ? (
                <img
                  src={game.leftPlayer.profile_icon}
                  alt={game.leftPlayer.name}
                />
              ) : (
                <span>{game.leftPlayer.name.charAt(0).toUpperCase()}</span>
              )}
            </div>
            <div className="avatar-info">
              <div
                className={`opponent-name ${!game.controls.isConnected || !game.leftPlayer.is_online ? "offline" : ""}`}
              >
                {game.leftPlayer.name}
                {game.declarer === game.leftPlayer && " (D)"}
              </div>
              {game.getRole(game.leftPlayer.position) && (
                <div className="player-role">
                  {game.getRole(game.leftPlayer.position)}
                </div>
              )}
            </div>
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
          className={`player-avatar-container ${game.isMyTurn ? "current-turn" : ""} ${game.controls.isLoading ? "loading" : ""} ${isMobile ? "mobile" : ""}`}
        >
          <div className="avatar-circle">
            {game.playerProfileIcon ? (
              <img
                src={game.playerProfileIcon}
                alt={game.playerName}
              />
            ) : (
              <span>{game.playerName.charAt(0).toUpperCase()}</span>
            )}
          </div>
          <div className="avatar-info">
            <div className="player-name">
              {game.playerName}
              {game.isDeclarer && " (D)"}
            </div>
            {game.getRole(game.playerPosition) && (
              <div className="player-role">
                {game.getRole(game.playerPosition)}
              </div>
            )}
          </div>
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
              <img src="/res/cards/back.svg" alt="deck" className="card-back" />
            </motion.div>
          )}

          {/* Deal Button */}
          {showDealButton && (
            <Button
              variant="contained"
              key="deal-button"
              className="deal-button"
              onClick={game.controls.deal}
              disabled={!game.controls.isConnected || game.controls.isLoading}
              style={{
                opacity:
                  !game.controls.isConnected || game.controls.isLoading
                    ? 0.5
                    : 1,
                cursor:
                  !game.controls.isConnected || game.controls.isLoading
                    ? "not-allowed"
                    : "pointer",
              }}
            >
              {game.controls.isLoading ? "Dealing..." : "Deal Cards"}
            </Button>
          )}

          {/* Player Hand - only show after deal started */}
          {sortedPlayerHand.map((card, index) => {
            const selected = isCardSelected(card);
            const basePosition = getPlayerCardPosition(
              index,
              sortedPlayerHand.length,
            );

            // Check if card can be played according to Skat rules
            const isValidMove =
              game.phase === "playing"
                ? canPlayCard(
                    card,
                    sortedPlayerHand,
                    game.trick,
                    game.gameMode,
                    game.trumpSuit,
                  )
                : true; // During skat exchange, all cards are valid for selection

            const canClickCard =
              game.controls.isConnected &&
              !game.controls.isLoading &&
              ((game.phase === "playing" && game.isMyTurn && isValidMove) ||
                (game.isSkatExchange && game.hasPickedUpSkat));

            const declarerOffset = game.isDeclarerChoice ? 40 : 0;
            // Raise card if selected
            const animatePosition = selected
              ? { ...basePosition, y: basePosition.y + declarerOffset - 20 }
              : { ...basePosition, y: basePosition.y + declarerOffset };
            const initialPosition = getPlayerCardInitialPosition(
              index,
              basePosition,
              declarerOffset,
            );

            return (
              <Card
                index={index}
                rank={card.rank}
                suit={card.suit}
                key={playerKeys[index]}
                selected={selected}
                disabled={game.phase === "playing" && game.isMyTurn && !isValidMove}
                animate={animatePosition}
                initial={initialPosition}
                skipInitialAnimation={!shouldAnimateFromDeck}
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
                    !game.controls.isConnected || game.controls.isLoading
                      ? "not-allowed"
                      : game.isBiddingPhase ||
                          (game.isSkatExchange && !game.hasPickedUpSkat) ||
                          (game.phase === "playing" && !isValidMove)
                        ? "not-allowed"
                        : "pointer",
                  opacity: !game.controls.isConnected ? 0.6 : 1,
                  zIndex: 300 + index,
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
            const declarerOffset = game.isDeclarerChoice ? -40 : 0;
            const animatePosition = {
              ...basePosition,
              y: basePosition.y + declarerOffset,
            };
            const initialPosition = getOpponentCardInitialPosition(
              index,
              basePosition,
              declarerOffset,
              "horizontal",
            );

            return (
              <Card
                index={index}
                key={key}
                className="motion-card opponent-card"
                animate={animatePosition}
                initial={initialPosition}
                skipInitialAnimation={!shouldAnimateFromDeck}
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
            const declarerOffset = game.isDeclarerChoice ? -30 : 0;
            const animatePosition = {
              ...basePosition,
              x: basePosition.x + declarerOffset,
            };
            const initialPosition = getOpponentCardInitialPosition(
              index,
              basePosition,
              declarerOffset,
              "vertical",
            );

            return (
              <Card
                index={index}
                key={key}
                className="motion-card opponent-card"
                animate={animatePosition}
                initial={initialPosition}
                skipInitialAnimation={!shouldAnimateFromDeck}
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
              skipInitialAnimation={true}
              animate={{
                ...getTrickPosition(index, game.trick.length),
              }}
              // Pass custom exit prop that will be used when card is removed
              custom={{
                trickWinner: game.trickWinner,
                declarerPosition: game.declarerPosition,
                playerIsDeclarer,
              }}
              exit={(() => {
                // Use the trick winner stored when the WebSocket message arrived
                // This is the most reliable way to get the correct winner for exit animations
                const trickWinner = game.trickWinnerRef.current.winner;
                const declarerPosition = game.trickWinnerRef.current.declarer;

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
        {/* Player's team Score Label - always bottom right */}
        <div
          className={`score-pile-label player-pile`}
          style={{
            position: "absolute",
            left: "50%",
            top: "50%",
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
            position: "absolute",
            left: "50%",
            top: "50%",
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
      </div>
    </div>
  );
}
