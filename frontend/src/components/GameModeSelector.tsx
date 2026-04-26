import React, { useState, useMemo } from "react";
import { CircularProgress } from "@mui/material";
import { useGameContext } from "../context/GameContext";
import { Card } from "../types";
import "./GameModeSelector.css";

// Calculate matadors from hand (including skat cards)
function countMatadors(hand: Card[], skatCards: Card[]): number {
  const jackOrder = ["♣", "♠", "♥", "♦"];

  // Combine hand and skat cards for matador calculation
  // In Skat, matadors are based on all 12 cards the declarer had access to
  const allCards = [...hand, ...skatCards];

  // Check if player has Club Jack
  const hasClubJack = allCards.some((c) => c.rank === "J" && c.suit === "♣");

  let matadors = 0;
  if (hasClubJack) {
    // "With" matadors - count consecutive jacks from top
    for (const suit of jackOrder) {
      if (allCards.some((c) => c.rank === "J" && c.suit === suit)) {
        matadors++;
      } else {
        break;
      }
    }
  } else {
    // "Without" matadors - count consecutive jacks from top that are missing
    for (const suit of jackOrder) {
      if (!allCards.some((c) => c.rank === "J" && c.suit === suit)) {
        matadors++;
      } else {
        break;
      }
    }
  }

  return matadors;
}

// Calculate potential game value
function calculateGameValue(
  mode: string,
  trumpSuit: string,
  hand: Card[],
  skatCards: Card[],
): number {
  let baseValue = 0;

  switch (mode) {
    case "grand":
      baseValue = 24;
      break;
    case "suit":
      const suitMap: Record<string, number> = {
        "♦": 9,
        "♥": 10,
        "♠": 11,
        "♣": 12,
      };
      baseValue = suitMap[trumpSuit] || 9;
      break;
    case "null":
      return 23;
  }

  const matadorCount = countMatadors(hand, skatCards);
  const multiplier = 1 + matadorCount; // 1 for "game" + matadors

  return baseValue * multiplier;
}

export function GameModeSelector() {
  const game = useGameContext();
  const [selectedMode, setSelectedMode] = useState<string>("suit");
  const [selectedTrump, setSelectedTrump] = useState<string>("♣");
  const [announceSchneider, setAnnounceSchneider] = useState<boolean>(false);
  const [announceSchwarz, setAnnounceSchwarz] = useState<boolean>(false);

  // Check if everyone passed (minimum bid of 18 was assigned)
  const everyonePassed = game.bidValue === 0;

  // Check if player is playing hand (didn't pick up skat)
  // Only true if skat cards are available but player hasn't picked them up
  const isPlayingHand = game.playedHand;

  // Calculate game value for current selection
  const gameValue = useMemo(() => {
    return calculateGameValue(
      selectedMode,
      selectedTrump,
      game.hand,
      game.skatCards,
    );
  }, [selectedMode, selectedTrump, game.hand, game.skatCards]);

  const isDisabled = !game.controls.isConnected || game.controls.isLoading;

  const handleDeclare = () => {
    if (!isDisabled) {
      game.controls.declareGame(
        selectedMode,
        selectedMode === "suit" ? selectedTrump : "",
        announceSchneider,
        announceSchwarz,
      );
    }
  };

  return (
    <div className="game-mode-selector">
      {everyonePassed && (
        <div className="everyone-passed-notice">
          All players passed. As dealer, you must declare with minimum bid of
          18.
        </div>
      )}

      <div className="game-value-info">
        <span>Game Value: {gameValue}</span>
        {gameValue < game.bidValue && (
          <span className="invalid">✗ Below bid ({game.bidValue})</span>
        )}
      </div>

      <div
        className={`trump-selection ${selectedMode !== "suit" ? "disabled" : ""}`}
      >
        <h4>Select Trump:</h4>
        <div className="trump-options">
          {["♣", "♠", "♥", "♦"].map((suit) => (
            <button
              key={suit}
              className={`trump-option ${suit === "♥" || suit === "♦" ? "red" : "black"} ${
                selectedTrump === suit ? "selected" : ""
              }`}
              onClick={() =>
                selectedMode === "suit" ? setSelectedTrump(suit) : undefined
              }
              disabled={selectedMode !== "suit"}
            >
              {suit}
            </button>
          ))}
        </div>
      </div>

      <div className="mode-options">
        <button
          className={`mode-option ${selectedMode === "grand" ? "selected" : ""}`}
          onClick={() => setSelectedMode("grand")}
        >
          <span className="mode-name">Grand</span>
          <span className="mode-desc">Jacks only</span>
        </button>

        <button
          className={`mode-option ${selectedMode === "suit" ? "selected" : ""}`}
          onClick={() => setSelectedMode("suit")}
        >
          <span className="mode-name">Suit</span>
          <span className="mode-desc">Choose trump</span>
        </button>

        <button
          className={`mode-option ${selectedMode === "null" ? "selected" : ""}`}
          onClick={() => setSelectedMode("null")}
        >
          <span className="mode-name">Null</span>
          <span className="mode-desc">No tricks</span>
        </button>
      </div>

      {isPlayingHand && selectedMode !== "null" && (
        <div className="announcements">
          <label className="announcement-option">
            <input
              type="checkbox"
              checked={announceSchneider}
              onChange={(e) => {
                setAnnounceSchneider(e.target.checked);
                if (!e.target.checked) {
                  setAnnounceSchwarz(false); // Can't announce schwarz without schneider
                }
              }}
            />
            <span>Announce Schneider (+1 multiplier)</span>
          </label>
          <label
            className={`announcement-option ${!announceSchneider ? "disabled" : ""}`}
          >
            <input
              type="checkbox"
              checked={announceSchwarz}
              onChange={(e) => setAnnounceSchwarz(e.target.checked)}
              disabled={!announceSchneider}
            />
            <span>Announce Schwarz (+1 multiplier)</span>
          </label>
        </div>
      )}

      <button
        className="declare-button"
        onClick={handleDeclare}
        disabled={gameValue < game.bidValue || isDisabled}
        style={{
          opacity: isDisabled ? 0.5 : 1,
          cursor: isDisabled ? "not-allowed" : "pointer",
        }}
      >
        {game.controls.isLoading ? (
          <CircularProgress size={20} />
        ) : (
          <>
            Declare{" "}
            {selectedMode === "grand"
              ? "Grand"
              : selectedMode === "null"
                ? "Null"
                : `${selectedTrump} Suit`}
            {announceSchwarz
              ? " (Schwarz)"
              : announceSchneider
                ? " (Schneider)"
                : ""}
          </>
        )}
      </button>
    </div>
  );
}
