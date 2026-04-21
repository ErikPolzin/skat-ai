import React, { useMemo } from "react";
import { CircularProgress } from "@mui/material";
import { useGameContext } from "../context/GameContext";
import "./BiddingControls.css";

// Valid bid values in Skat (matching game/game.go)
const VALID_BID_VALUES = [
  0, 18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50,
  54, 55, 59, 60, 63, 66, 70, 72, 77, 80, 81, 84, 88, 90, 96, 99,
  100, 108, 110, 117, 120, 121, 126, 130, 132, 135, 140, 143, 144,
  150, 153, 154, 156, 160, 162, 165, 168, 170, 176, 180, 187, 192,
  198, 204, 216, 240, 264,
];

function getNextBidValue(currentBid: number): number {
  for (const bid of VALID_BID_VALUES) {
    if (bid > currentBid) {
      return bid;
    }
  }
  return 0; // No higher bid available
}

export function BiddingControls() {
  const game = useGameContext();
  const isDisabled = !game.controls.isConnected || game.controls.isLoading;

  // Calculate next bid value
  const nextBidValue = useMemo(() => getNextBidValue(game.bidValue), [game.bidValue]);

  // Determine if current player is the one who raises (not accepts)
  // In Skat bidding:
  // - Speaker (2) raises vs Listener (1)
  // - Listener (1) raises vs Dealer (0) after Speaker passes
  // - Dealer (0) raises vs Speaker (2) after Listener passes
  const isRaiser = useMemo(() => {
    const pos = game.playerPosition;
    if (pos === 2 && !game.listenerPassed) {
      // Speaker raises against Listener
      return true;
    } else if (pos === 1 && game.speakerPassed && !game.dealerPassed) {
      // Listener raises against Dealer after Speaker passed
      return true;
    } else if (pos === 0 && game.listenerPassed && !game.speakerPassed) {
      // Dealer raises against Speaker after Listener passed
      return true;
    }
    return false;
  }, [game.playerPosition, game.listenerPassed, game.speakerPassed, game.dealerPassed]);

  // Determine button text based on role
  const acceptButtonText = isRaiser
    ? `Raise (${nextBidValue})`
    : `Accept (${game.bidValue})`;

  if (!game.isMyTurn) {
    return (
      <div className="bidding-controls waiting">
        <span>Waiting for bid...</span>
      </div>
    );
  }

  return (
    <div className="bidding-controls">
      <div className="bid-info">
        <span className="current-bid">Current Bid: {game.bidValue}</span>
      </div>

      <div className="bid-buttons">
        <button
          className="bid-btn hold"
          onClick={() => game.controls.bid(true)}
          disabled={isDisabled}
          style={{ opacity: isDisabled ? 0.5 : 1, cursor: isDisabled ? "not-allowed" : "pointer" }}
        >
          {game.controls.isLoading ? <CircularProgress size={20} /> : acceptButtonText}
        </button>
        <button
          className="bid-btn pass"
          onClick={() => game.controls.bid(false)}
          disabled={isDisabled}
          style={{ opacity: isDisabled ? 0.5 : 1, cursor: isDisabled ? "not-allowed" : "pointer" }}
        >
          {game.controls.isLoading ? <CircularProgress size={20} /> : "Pass"}
        </button>
      </div>
    </div>
  );
}
