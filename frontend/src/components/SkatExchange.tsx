import React from "react";
import { useGameContext } from "../context/GameContext";
import { Card as CardType } from "../api/games";
import Card from "./Card";
import "./SkatExchange.css";

interface SkatExchangeProps {
  selectedCards: CardType[];
  onDiscardCards: () => void;
}

export function SkatExchange({
  selectedCards,
  onDiscardCards,
}: SkatExchangeProps) {
  const game = useGameContext();

  if (!game.hasPickedUpSkat) {
    return (
      <div className="skat-exchange">
        <div className="skat-preview">
          <h3>Skat Decision</h3>
          <div className="skat-cards">
            {Array.from({ length: 2 }).map((card, index) => (
              <Card
                key={index}
                index={index}
                animate={{ x: index * 80 - 75, y: 0 }}
                exit={{ opacity: 0 }}
                style={{ position: "absolute" }}
              />
            ))}
          </div>
          <div className="skat-actions">
            <button
              className="skat-btn pickup"
              onClick={game.controls.pickUpSkat}
            >
              Pick Up Skat
            </button>
            <button
              className="skat-btn play-hand"
              onClick={game.controls.playHand}
            >
              Play Hand
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="skat-exchange">
      <div className="discard-info">
        <h3>Select 2 cards to discard</h3>
        <p>{selectedCards.length} / 2 selected</p>
        <button
          className="skat-btn discard"
          onClick={onDiscardCards}
          disabled={selectedCards.length !== 2}
        >
          Discard Selected
        </button>
      </div>
    </div>
  );
}
