import React, { useState } from "react";
import { useGameContext } from "../context/GameContext";
import "./GameModeSelector.css";

export function GameModeSelector() {
  const game = useGameContext();
  const [selectedMode, setSelectedMode] = useState<string>("suit");
  const [selectedTrump, setSelectedTrump] = useState<string>("♣");

  const handleDeclare = () => {
    game.controls.declareGame(
      selectedMode,
      selectedMode === "suit" ? selectedTrump : "",
    );
  };

  return (
    <div className="game-mode-selector">
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
              onClick={() => selectedMode === "suit" ? setSelectedTrump(suit) : undefined}
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

      <button className="declare-button" onClick={handleDeclare}>
        Declare{" "}
        {selectedMode === "grand"
          ? "Grand"
          : selectedMode === "null"
            ? "Null"
            : `${selectedTrump} Suit`}
      </button>
    </div>
  );
}
