import React, { useState } from "react";
import "./GameDeclarationPanel.css";
import { useGameContext } from "../context/GameContext";

export function GameDeclarationPanel() {
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
    <div className="declaration-overlay">
      <div className="declaration-panel">
        <h3>Choose Your Game</h3>
        {game.isDeclarer ? (
          <div className="game-selection">
            <div className="game-modes">
              <label className="game-mode-option">
                <input
                  type="radio"
                  name="gameMode"
                  value="grand"
                  checked={selectedMode === "grand"}
                  onChange={(e) => setSelectedMode(e.target.value)}
                />
                <span>Grand (Jacks only)</span>
              </label>
              <label className="game-mode-option">
                <input
                  type="radio"
                  name="gameMode"
                  value="suit"
                  checked={selectedMode === "suit"}
                  onChange={(e) => setSelectedMode(e.target.value)}
                />
                <span>Suit Game</span>
              </label>
              <label className="game-mode-option">
                <input
                  type="radio"
                  name="gameMode"
                  value="null"
                  checked={selectedMode === "null"}
                  onChange={(e) => setSelectedMode(e.target.value)}
                />
                <span>Null (Take no tricks)</span>
              </label>
            </div>

            {selectedMode === "suit" && (
              <div className="trump-selection">
                <h4>Select Trump Suit:</h4>
                <div className="trump-suits">
                  {["♣", "♠", "♥", "♦"].map((suit) => (
                    <button
                      key={suit}
                      className={`trump-suit-btn ${selectedTrump === suit ? "selected" : ""}`}
                      onClick={() => setSelectedTrump(suit)}
                    >
                      {suit}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <button
              className="btn btn-primary declare-btn"
              onClick={handleDeclare}
            >
              Declare{" "}
              {selectedMode === "grand"
                ? "Grand"
                : selectedMode === "null"
                  ? "Null"
                  : `${selectedTrump} Suit`}
            </button>
          </div>
        ) : (
          <div className="waiting-message">
            Waiting for declarer to choose game mode...
          </div>
        )}
      </div>
    </div>
  );
}
