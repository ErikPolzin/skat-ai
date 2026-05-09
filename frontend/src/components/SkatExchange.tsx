import React from "react";
import { CircularProgress } from "@mui/material";
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

  // Check if everyone passed (minimum bid of 18 was assigned)
  const everyonePassed = game.bidValue === 0 && game.isDeclarer;
  const isDisabled = !game.controls.isConnected || game.controls.isLoading;

  if (!game.hasPickedUpSkat) {
    return (
      <div className="skat-exchange">
        <div className="skat-preview">
          <h3>Skat Decision</h3>
          {everyonePassed && (
            <p className="everyone-passed-notice">
              All players passed. As dealer, you must declare with minimum bid
              of 18.
            </p>
          )}
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
              disabled={isDisabled}
              style={{
                cursor: isDisabled ? "not-allowed" : "pointer",
              }}
            >
              {game.controls.isLoading ? (
                <CircularProgress size={20} />
              ) : (
                "Pick Up Skat"
              )}
            </button>
            <button
              className="skat-btn play-hand"
              onClick={game.controls.playHand}
              disabled={isDisabled}
              style={{
                cursor: isDisabled ? "not-allowed" : "pointer",
              }}
            >
              {game.controls.isLoading ? (
                <CircularProgress size={20} />
              ) : (
                "Play Hand"
              )}
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
          disabled={selectedCards.length !== 2 || isDisabled}
          style={{
            opacity: isDisabled ? 0.5 : 1,
            cursor: isDisabled ? "not-allowed" : "pointer",
          }}
        >
          {game.controls.isLoading ? (
            <CircularProgress size={20} />
          ) : (
            "Discard Selected"
          )}
        </button>
      </div>
    </div>
  );
}
