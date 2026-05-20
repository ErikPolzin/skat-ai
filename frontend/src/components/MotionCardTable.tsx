import React, {
  useState,
  useEffect,
  useMemo,
  useRef,
  useCallback,
} from "react";
import { AnimatePresence, motion } from "motion/react";
import {
  Box,
  Button,
  Chip,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import CheckIcon from "@mui/icons-material/Check";
import SignalWifiOffIcon from "@mui/icons-material/SignalWifiOff";
import WarningIcon from "@mui/icons-material/Warning";
import { type Card as CardType, reportTimeout } from "../api/games";
import "./MotionCardTable.css";
import { useGameContext } from "../context/GameContext";
import Card from "./Card";
import { GameModeSelector } from "./GameModeSelector";
import { GameLobbyWaiting } from "./GameLobbyWaiting";
import { BiddingControls } from "./BiddingControls";
import { SkatExchange } from "./SkatExchange";
import { GameOverScreen } from "./GameOverScreen";
import {
  canPlayCard,
  compareCardsForHand,
  getGameModeDisplay,
  getGameModeSVG,
  isSameCard,
} from "../utils/skatRules";
import { CircularTimer } from "./CircularTimer";
import { useDeadlineTimer } from "../hooks/useDeadlineTimer";
import ThemedLoader from "./ThemedLoader";
import { useNavigate } from "react-router-dom";

export function MotionCardTable() {
  const game = useGameContext();
  const theme = useTheme();
  const navigate = useNavigate();
  const isMobile = useMediaQuery(theme.breakpoints.down("sm"));
  const isTablet = useMediaQuery(theme.breakpoints.down("md"));

  const [selectedCards, setSelectedCards] = useState<CardType[]>([]);

  // Track whether cards should animate from deck (true) or spread from left (false)
  const [shouldAnimateFromDeck, setShouldAnimateFromDeck] = useState(false);
  const hasInitializedRef = useRef(false);
  const [selectedPlayedCard, setSelectedPlayedCard] = useState<CardType | null>(
    null,
  );
  const reportedTimeoutDeadlineRef = useRef<string | null>(null);

  // Track window size for responsive positioning
  const [windowSize, setWindowSize] = useState({
    width: window.innerWidth,
    height: window.innerHeight,
  });

  // Track deadline timer for countdown display
  const { secondsRemaining, formattedTime, isExpired } = useDeadlineTimer(
    game.currentPlayerDeadline || "",
  );
  const showDeadlineCountdown =
    game.timerEnabled &&
    !!game.currentPlayerDeadline &&
    secondsRemaining !== null &&
    secondsRemaining > 0 &&
    secondsRemaining <= 30;

  // Handle timeout when deadline expires
  useEffect(() => {
    if (
      !game.timerEnabled ||
      !isExpired ||
      !game.currentPlayerDeadline ||
      !game.player?.id ||
      game.phase === "complete" ||
      reportedTimeoutDeadlineRef.current === game.currentPlayerDeadline
    ) {
      return;
    }

    reportedTimeoutDeadlineRef.current = game.currentPlayerDeadline;
    reportTimeout(game.gameId, game.player.id).catch((err) => {
      reportedTimeoutDeadlineRef.current = null;
      console.error("Failed to report timeout:", err);
    });
  }, [
    game.timerEnabled,
    isExpired,
    game.currentPlayerDeadline,
    game.gameId,
    game.player?.id,
    game.phase,
  ]);

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
  const avatarTimerSize = isMobile ? 66 : isTablet ? 70 : 65;

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
      // eslint-disable-next-line react-hooks/set-state-in-effect
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

  const canKeepCardSelection =
    (game.phase === "playing" && game.isMyTurn) ||
    (game.isSkatExchange && game.isDeclarer && game.hasPickedUpSkat);

  const activeSelectedCards = useMemo(() => {
    if (!canKeepCardSelection) {
      return [];
    }

    return selectedCards.filter((selectedCard) =>
      game.playerHand.some((handCard) => isSameCard(handCard, selectedCard)),
    );
  }, [canKeepCardSelection, game.playerHand, selectedCards]);

  // Helper to check if a card is selected
  const isCardSelected = (card: CardType) => {
    return activeSelectedCards.some((c) => isSameCard(c, card));
  };

  const handlePlayCard = (card: CardType) => {
    // Don't allow actions when disconnected or loading
    if (!game.controls.isConnected || game.controls.isLoading) {
      return;
    }

    if (game.isSkatExchange && game.isDeclarer && game.hasPickedUpSkat) {
      // In skat exchange phase, clicking cards selects them for discard
      const isSelected = activeSelectedCards.some((c) => isSameCard(c, card));

      if (isSelected) {
        // Deselect card
        setSelectedCards(
          activeSelectedCards.filter((c) => !isSameCard(c, card)),
        );
      } else if (activeSelectedCards.length < 2) {
        // Select card (max 2)
        setSelectedCards([...activeSelectedCards, card]);
      }
    } else {
      if (!activeSelectedCards.some((c) => isSameCard(c, card))) {
        setSelectedCards([card]);
        return;
      }

      setSelectedPlayedCard(card);
      setSelectedCards([]);
      game.controls.playCard(card);
    }
  };

  const handleDiscardCards = useCallback(() => {
    if (activeSelectedCards.length === 2) {
      game.controls.discardCards(activeSelectedCards);
      setSelectedCards([]);
    }
  }, [activeSelectedCards, game.controls]);

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
	const isRamsch = game.gameMode === "ramsch";
	const playerPileScore =
		isRamsch && game.playerPosition !== null
			? game.playerScores[game.playerPosition]
			: playerIsDeclarer
				? game.declarerScore
				: game.opponentScore;
	const opponentPileScore =
		isRamsch && game.playerPosition !== null
			? game.playerScores.reduce(
					(sum, score, index) =>
						index === game.playerPosition ? sum : sum + score,
					0,
				)
			: playerIsDeclarer
				? game.opponentScore
				: game.declarerScore;
  const totalCardPoints = 120;
  const clampedPlayerScore = Math.max(
    0,
    Math.min(totalCardPoints, playerPileScore),
  );
  const clampedOpponentScore = Math.max(
    0,
    Math.min(totalCardPoints - clampedPlayerScore, opponentPileScore),
  );
  const playerScorePercent = (clampedPlayerScore / totalCardPoints) * 100;
  const opponentScorePercent = (clampedOpponentScore / totalCardPoints) * 100;
  const unclaimedScorePercent = Math.max(
    0,
    100 - playerScorePercent - opponentScorePercent,
  );

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

  const sortedPlayerHand = useMemo(
    () =>
      game.playerHand.toSorted((a, b) =>
        compareCardsForHand(a, b, game.gameMode, game.trumpSuit),
      ),
    [game.gameMode, game.playerHand, game.trumpSuit],
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
      game.leftPlayer,
      game.playerPosition,
      game.topPlayer?.card_count,
      game.trick,
      game.trickStarter,
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

  const makeExtraCenterSpace = useMemo(() => {
    return game.isDeclarer && (game.isDeclarerChoice || game.isSkatExchange);
  }, [game.isDeclarer, game.isDeclarerChoice, game.isSkatExchange]);

  const getTrickInitialPosition = (card: CardType, index: number) => {
    const playedByIndex = (game.trickStarter + index) % 3;
    const playerOffset = makeExtraCenterSpace ? 50 : 0;
    const opponentOffset = makeExtraCenterSpace ? -50 : 0;

    if (playedByIndex === game.playerPosition) {
      const handWithCard = sortedPlayerHand.some(
        (candidate) =>
          candidate.rank === card.rank && candidate.suit === card.suit,
      )
        ? sortedPlayerHand
        : [...sortedPlayerHand, card].toSorted((a, b) =>
            compareCardsForHand(a, b, game.gameMode, game.trumpSuit),
          );
      const cardIndex = handWithCard.findIndex(
        (candidate) =>
          candidate.rank === card.rank && candidate.suit === card.suit,
      );
      const position = getPlayerCardPosition(
        Math.max(cardIndex, 0),
        handWithCard.length,
      );
      const selectedLift = selectedPlayedCard
        ? isSameCard(selectedPlayedCard, card)
          ? -20
          : 0
        : 0;
      return {
        ...position,
        y: position.y + playerOffset + selectedLift,
        rotateY: 180,
      };
    }

    if (playedByIndex === game.leftPlayer?.position) {
      const cardIndex = game.leftPlayer.card_count ?? 0;
      const position = getOpponentCardPosition(
        "left",
        cardIndex,
        cardIndex + 1,
      );
      return { ...position, x: position.x + opponentOffset };
    }

    const cardIndex = game.topPlayer?.card_count ?? 0;
    const position = getOpponentCardPosition("top", cardIndex, cardIndex + 1);
    return { ...position, y: position.y + opponentOffset };
  };

  const centerOverrideUI = useMemo(() => {
    return !game.controls.isConnected ? (
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
    ) : game.isLoading ? (
      <Box sx={{ textAlign: "center" }}>
        <ThemedLoader size={60} />
        <Typography variant="h6" sx={{ mt: 2 }}>
          Loading game...
        </Typography>
      </Box>
    ) : game.error ? (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "100vh",
        }}
      >
        <WarningIcon color="warning" sx={{ fontSize: 60, mb: 2 }} />
        <Typography variant="h5" gutterBottom>
          Unable to Load Game
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 3 }}>
          {game.error}
        </Typography>
        <Box sx={{ display: "flex", gap: 2, justifyContent: "center" }}>
          <Button variant="contained" onClick={() => game.refetch()}>
            Try Again
          </Button>
          <Button variant="outlined" onClick={() => navigate("/")}>
            Back to Lobby
          </Button>
        </Box>
      </Box>
    ) : game.isInLobby ? (
      <GameLobbyWaiting />
    ) : game.isBiddingPhase ? (
      <BiddingControls />
    ) : game.isSkatExchange && game.isDeclarer ? (
      <SkatExchange
        selectedCards={activeSelectedCards}
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
    ) : null;
  }, [activeSelectedCards, game, handleDiscardCards, navigate]);

  return (
    <div className="motion-card-table" style={cardTableStyle}>
      <div
        className={`table-surface ${showSessionResults ? "with-session-bar" : ""}`}
      >
        {/* Top Opponent Avatar */}
        {game.topPlayer && (
          <div
            className={`opponent-avatar-container top ${game.topPlayer.position === game.currentPlayer ? "current-turn" : ""} ${isMobile ? "mobile" : ""}`}
          >
            <div
              className={`avatar-circle ${!game.controls.isConnected || !game.topPlayer.is_online ? "offline" : ""}`}
              style={{ position: "relative", overflow: "visible" }}
            >
              <CircularTimer
                deadline={game.currentPlayerDeadline || ""}
                isCurrentPlayer={game.topPlayer.position === game.currentPlayer}
                isAI={game.topPlayer.is_agent}
                size={avatarTimerSize}
              />
              <div
                className="avatar-content"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: "100%",
                  borderRadius: "50%",
                  overflow: "hidden",
                  zIndex: 1,
                }}
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
              {showDeadlineCountdown &&
                game.topPlayer.position === game.currentPlayer && (
                  <div
                    className={`avatar-deadline-countdown ${secondsRemaining <= 10 ? "urgent" : ""}`}
                  >
                    {formattedTime}
                  </div>
                )}
              {(game.topPlayer.ready_for_next ||
                (game.gameOver && game.topPlayer.is_agent)) && (
                <div className="avatar-ready-check" aria-label="Ready">
                  <CheckIcon fontSize="inherit" />
                </div>
              )}
            </div>
            <div className="avatar-info">
              <Chip
                label={`${game.topPlayer.name} ${game.declarer === game.topPlayer ? "(D)" : ""}`}
                sx={{
                  bgcolor: "background.paper",
                }}
              ></Chip>
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
              style={{ position: "relative", overflow: "visible" }}
            >
              <CircularTimer
                deadline={game.currentPlayerDeadline || ""}
                isCurrentPlayer={
                  game.leftPlayer.position === game.currentPlayer
                }
                isAI={game.leftPlayer.is_agent}
                size={avatarTimerSize}
              />
              <div
                className="avatar-content"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: "100%",
                  borderRadius: "50%",
                  overflow: "hidden",
                  zIndex: 1,
                }}
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
              {showDeadlineCountdown &&
                game.leftPlayer.position === game.currentPlayer && (
                  <div
                    className={`avatar-deadline-countdown ${secondsRemaining <= 10 ? "urgent" : ""}`}
                  >
                    {formattedTime}
                  </div>
                )}
              {(game.leftPlayer.ready_for_next ||
                (game.gameOver && game.leftPlayer.is_agent)) && (
                <div className="avatar-ready-check" aria-label="Ready">
                  <CheckIcon fontSize="inherit" />
                </div>
              )}
            </div>
            <div className="avatar-info">
              <Chip
                label={`${game.leftPlayer.name} ${game.declarer === game.leftPlayer ? "(D)" : ""}`}
                sx={{
                  bgcolor: "background.paper",
                }}
              ></Chip>
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
        {game.player && (
          <div
            className={`player-avatar-container ${game.isMyTurn ? "current-turn" : ""} ${game.controls.isLoading ? "loading" : ""} ${isMobile ? "mobile" : ""}`}
          >
            <div
              className="avatar-circle"
              style={{ position: "relative", overflow: "visible" }}
            >
              <CircularTimer
                deadline={game.currentPlayerDeadline || ""}
                isCurrentPlayer={game.isMyTurn}
                isAI={false}
                size={avatarTimerSize}
              />
              <div
                className="avatar-content"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: "100%",
                  borderRadius: "50%",
                  overflow: "hidden",
                  zIndex: 1,
                }}
              >
                {game.player?.profile_icon ? (
                  <img
                    src={game.player?.profile_icon}
                    alt={game.player?.name}
                  />
                ) : (
                  <span>{game.player?.name.charAt(0).toUpperCase()}</span>
                )}
              </div>
              {showDeadlineCountdown && game.isMyTurn && (
                <div
                  className={`avatar-deadline-countdown ${secondsRemaining <= 10 ? "urgent" : ""}`}
                >
                  {formattedTime}
                </div>
              )}
              {game.player.ready_for_next && (
                <div className="avatar-ready-check" aria-label="Ready">
                  <CheckIcon fontSize="inherit" />
                </div>
              )}
            </div>
            <div className="avatar-info">
              <Chip
                label={`${game.player?.name} ${game.isDeclarer ? "(D)" : ""}`}
                sx={{
                  bgcolor: "background.paper",
                }}
              ></Chip>
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
        )}

        <AnimatePresence>
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

            const declarerOffset = makeExtraCenterSpace ? 50 : 0;
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
                disabled={
                  game.phase === "playing" && game.isMyTurn && !isValidMove
                }
                animate={animatePosition}
                initial={initialPosition}
                skipInitialAnimation={!shouldAnimateFromDeck}
                whileHover={
                  canClickCard && !isMobile
                    ? { y: basePosition.y + declarerOffset - 20 }
                    : undefined
                }
                onClick={() => canClickCard && handlePlayCard(card)}
                style={{
                  cursor:
                    !game.controls.isConnected || game.controls.isLoading
                      ? "not-allowed"
                      : game.isBiddingPhase ||
                          (game.isSkatExchange && !game.hasPickedUpSkat) ||
                          (game.phase === "playing" && !isValidMove)
                        ? "not-allowed"
                        : "pointer",
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
            const declarerOffset = makeExtraCenterSpace ? -50 : 0;
            const animatePosition = {
              ...basePosition,
              y: basePosition.y + declarerOffset,
              // opacity: isMobile ? 0 : 1,
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
            const declarerOffset = makeExtraCenterSpace ? -50 : 0;
            const animatePosition = {
              ...basePosition,
              x: basePosition.x + declarerOffset,
              opacity: isMobile ? 0 : 1,
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

          {/*Center UI */}
          {centerOverrideUI ? (
            centerOverrideUI
          ) : (
            <>
              {game.phase == "playing" && (
                <div className="game-mode-display">
                  <img
                    src={getGameModeSVG(game.gameMode, game.trumpSuit)}
                    width="200"
                    height="200"
                    alt={game.trumpSuit}
                  />
                  <span className="mode-title">
                    {getGameModeDisplay(game.gameMode, game.trumpSuit)}
                  </span>
                </div>
              )}
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
                  <img
                    src="/res/cards/back.svg"
                    alt="deck"
                    className="card-back"
                  />
                </motion.div>
              )}

              {/* Deal Button */}
              {showDealButton && (
                <Button
                  variant="contained"
                  key="deal-button"
                  className="deal-button"
                  onClick={game.controls.deal}
                  disabled={
                    !game.controls.isConnected || game.controls.isLoading
                  }
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

              {/* Trick Cards */}
              <AnimatePresence>
                {!game.gameOver &&
                  game.trick.map((card, index) => (
                    <Card
                      key={trickKeys[index]}
                      index={index}
                      rank={card.rank}
                      suit={card.suit}
                      className="motion-card"
                      skipInitialAnimation={true}
                      initial={getTrickInitialPosition(card, index)}
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
                        // Only completed tricks should collect into a score pile.
                        // Partial trick cards can briefly disappear during optimistic/server
                        // reconciliation and should not look like they were scored.
                        if (game.trick.length < 3) {
                          return {
                            opacity: 0,
                            transition: { duration: 0.12 },
                          };
                        }

                        // Use the trick winner stored when the WebSocket message arrived
                        // This is the most reliable way to get the correct winner for exit animations
                        const trickWinner = game.trickWinnerRef.current.winner;
                        const declarerPosition =
                          game.trickWinnerRef.current.declarer;

                        if (trickWinner == null || declarerPosition == null) {
                          return {
                            opacity: 0,
                            transition: { duration: 0.12 },
                          };
                        }

                        // Check if the trick winner is the declarer
                        const declarerWonTrick =
                          trickWinner === declarerPosition;

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
            </>
          )}
        </AnimatePresence>

        {!isRamsch && (
          <>
            {/* Score Pile Labels - only show during playing phase when declarer is set and there are cards */}
            <div
              className="pile-points-bar"
              title={`Player ${playerPileScore} - Opponent ${opponentPileScore} of ${totalCardPoints}`}
              style={{
                position: "absolute",
                left: "50%",
                top: "50%",
                height: `${Math.max(
                  80,
                  Math.abs(
                    getPileAbsolutePosition(true).y -
                      getPileAbsolutePosition(false).y,
                  ) -
                    CARD_HEIGHT -
                    72,
                )}px`,
                transform: `translate(calc(-50% + ${getPileAbsolutePosition(true).x}px), -50%)`,
              }}
            >
              <div
                className="pile-points-marker schneider"
                style={{ top: "25%" }}
              />
              <div className="pile-points-marker" style={{ top: "50%" }} />
              <div
                className="pile-points-marker schneider"
                style={{ top: "75%" }}
              />
              <div
                className="pile-points-segment opponent"
                style={{ height: `${opponentScorePercent}%` }}
              />
              <div
                className="pile-points-segment unclaimed"
                style={{ height: `${unclaimedScorePercent}%` }}
              />
              <div
                className="pile-points-segment player"
                style={{ height: `${playerScorePercent}%` }}
              />
            </div>

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
              <span className="pile-score">{playerPileScore}</span>
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
              <span className="pile-score">{opponentPileScore}</span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
