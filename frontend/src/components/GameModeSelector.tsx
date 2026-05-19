import { useState, useMemo } from "react";
import { CircularProgress } from "@mui/material";
import { useGameContext } from "../context/GameContext";
import {
  SUITS,
  calculatePotentialGameValue,
  canAnnounceSchneider,
  canAnnounceSchwarz,
} from "../utils/skatRules";
import "./GameModeSelector.css";

export function GameModeSelector() {
  const game = useGameContext();
  const [selectedMode, setSelectedMode] = useState<string>("suit");
  const [selectedTrump, setSelectedTrump] = useState<string>("♣");
  const [announceSchneider, setAnnounceSchneider] = useState<boolean>(false);
  const [announceSchwarz, setAnnounceSchwarz] = useState<boolean>(false);

  const schneiderCanBeAnnounced = canAnnounceSchneider(
    selectedMode,
    game.playedHand,
  );
  const schwarzCanBeAnnounced = canAnnounceSchwarz(
    selectedMode,
    game.playedHand,
    announceSchneider,
  );
  const effectiveAnnounceSchneider =
    schneiderCanBeAnnounced && announceSchneider;
  const effectiveAnnounceSchwarz =
    schwarzCanBeAnnounced && announceSchwarz;

  // Check if everyone passed and forehand was forced to play.
  const everyonePassed = game.bidValue === 0;

  // Calculate game value for current selection
  const gameValue = useMemo(() => {
    return calculatePotentialGameValue({
      mode: selectedMode,
      trumpSuit: selectedTrump,
      hand: game.hand,
      skatCards: game.skatCards,
      playedHand: game.playedHand,
      announcedSchneider: effectiveAnnounceSchneider,
      announcedSchwarz: effectiveAnnounceSchwarz,
    });
  }, [
    selectedMode,
    selectedTrump,
    game.hand,
    game.skatCards,
    game.playedHand,
    effectiveAnnounceSchneider,
    effectiveAnnounceSchwarz,
  ]);

  const isDisabled = !game.controls.isConnected || game.controls.isLoading;
  const isOverbidDeclaration = gameValue < game.bidValue;

  const handleDeclare = () => {
    if (!isDisabled) {
      game.controls.declareGame(
        selectedMode,
        selectedMode === "suit" ? selectedTrump : "",
        effectiveAnnounceSchneider,
        effectiveAnnounceSchwarz,
      );
    }
  };

  const handleModeSelect = (mode: string) => {
    setSelectedMode(mode);
    if (!canAnnounceSchneider(mode, game.playedHand)) {
      setAnnounceSchneider(false);
      setAnnounceSchwarz(false);
    }
  };

  return (
    <div className="game-mode-selector">
      {everyonePassed && (
        <div className="everyone-passed-notice">All players passed.</div>
      )}

      <div className="game-value-info">
        <span>Game Value: {gameValue}</span>
        {isOverbidDeclaration && (
          <span className="invalid">
            Below bid ({game.bidValue}) - overbid loss
          </span>
        )}
      </div>

      <div
        className={`trump-selection ${selectedMode !== "suit" ? "disabled" : ""}`}
      >
        <h4>Select Trump:</h4>
        <div className="trump-options">
          {SUITS.map((suit) => (
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
          onClick={() => handleModeSelect("grand")}
        >
          <span className="mode-name">Grand</span>
          <span className="mode-desc">Jacks only</span>
        </button>

        <button
          className={`mode-option ${selectedMode === "suit" ? "selected" : ""}`}
          onClick={() => handleModeSelect("suit")}
        >
          <span className="mode-name">Suit</span>
          <span className="mode-desc">Choose trump</span>
        </button>

        <button
          className={`mode-option ${selectedMode === "null" ? "selected" : ""}`}
          onClick={() => handleModeSelect("null")}
        >
          <span className="mode-name">Null</span>
          <span className="mode-desc">No tricks</span>
        </button>
      </div>

      {schneiderCanBeAnnounced && (
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
            className={`announcement-option ${!schwarzCanBeAnnounced ? "disabled" : ""}`}
          >
            <input
              type="checkbox"
              checked={announceSchwarz}
              onChange={(e) => setAnnounceSchwarz(e.target.checked)}
              disabled={!schwarzCanBeAnnounced}
            />
            <span>Announce Schwarz (+1 multiplier)</span>
          </label>
        </div>
      )}

      <button
        className="declare-button"
        onClick={handleDeclare}
        disabled={isDisabled}
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
            {effectiveAnnounceSchwarz
              ? " (Schwarz)"
              : effectiveAnnounceSchneider
                ? " (Schneider)"
                : ""}
            {isOverbidDeclaration ? " and lose" : ""}
          </>
        )}
      </button>
    </div>
  );
}
