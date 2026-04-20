import React from "react";
import "./BiddingPanel.css";
import { useGameContext } from "../context/GameContext";

export function BiddingPanel() {
  const game = useGameContext();

  // Get the next valid bid value based on current bid
  const getNextBidValue = () => {
    const validBids = [
      18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50, 54, 55,
      59, 60, 63, 66, 70, 72, 77, 80, 81, 84, 88, 90, 96, 99, 100, 108, 110,
      117, 120, 121, 126, 130, 132, 135, 140, 143, 144, 150, 153, 154, 156, 160,
      162, 165, 168, 170, 176, 180, 187, 192, 198, 204, 216, 240, 264,
    ];

    // Find the next bid value
    for (const bid of validBids) {
      if (bid > game.bidValue) {
        return bid;
      }
    }
    return game.bidValue + 1; // Fallback
  };

  // Determine if current player is in the "announcing" role
  const isSpeaker = game.playerPosition === 2;
  const isListener = game.playerPosition === 1;
  const isDealer = game.playerPosition === 0;

  const isAnnouncing =
    (isSpeaker && !game.speakerPassed) ||
    (isListener && game.speakerPassed && !game.listenerPassed) ||
    (isDealer &&
      game.speakerPassed &&
      game.listenerPassed &&
      !game.dealerPassed);

  const canBid = game.bidValue > 0;

  return (
    <div className="bidding-panel">
      <h3>Bidding Phase</h3>
      <div className="current-bid-display">
        Current Bid: <strong>{game.bidValue || "No bid yet"}</strong>
      </div>

      {game.isMyTurn ? (
        <div className="bid-controls">
          <div className="bid-prompt">Your turn to bid!</div>
          {isAnnouncing ? (
            // Announcing player - can raise or pass
            <>
              <button
                className="btn btn-bid btn-raise"
                onClick={() => game.controls.bid(String(getNextBidValue()))}
              >
                Bid {getNextBidValue()}
              </button>
              <button
                className="btn btn-bid btn-pass"
                onClick={() => game.controls.bid("pass")}
              >
                Pass
              </button>
            </>
          ) : (
            // Responding player - can hold or pass
            <>
              {canBid ? (
                <button
                  className="btn btn-bid btn-hold"
                  onClick={() => game.controls.bid("hold")}
                >
                  Yes ({game.bidValue})
                </button>
              ) : (
                <button
                  className="btn btn-bid btn-raise"
                  onClick={() => game.controls.bid(String(getNextBidValue()))}
                >
                  Bid {getNextBidValue()}
                </button>
              )}
              <button
                className="btn btn-bid btn-pass"
                onClick={() => game.controls.bid("pass")}
              >
                Pass
              </button>
            </>
          )}
        </div>
      ) : (
        <div className="waiting-message">
          Waiting for {game.currentPlayer} to bid...
        </div>
      )}
    </div>
  );
}
