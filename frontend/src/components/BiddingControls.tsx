import { useMemo } from "react";
import { Box, Button, Typography } from "@mui/material";
import { useGameContext } from "../context/GameContext";

// Legal named bid values in Skat. The backend uses 0 as the "no bid yet"
// sentinel, but players never accept a bid value of 0.
const VALID_BID_VALUES = [
  18, 20, 22, 23, 24, 27, 30, 33, 35, 36, 40, 44, 45, 46, 48, 50, 54, 55, 59,
  60, 63, 66, 70, 72, 77, 80, 81, 84, 88, 90, 96, 99, 100, 108, 110, 117, 120,
  121, 126, 130, 132, 135, 140, 143, 144, 150, 153, 154, 156, 160, 162, 165,
  168, 170, 176, 180, 187, 192, 198, 204, 216, 240, 264,
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
  const currentBidText = game.bidValue > 0 ? game.bidValue : "No bid";

  // Calculate next bid value
  const nextBidValue = useMemo(
    () => getNextBidValue(game.bidValue),
    [game.bidValue],
  );

  // Match backend bidding semantics:
  // - Speaker names the next bid while bidding against Listener.
  // - Dealer names the next bid in phase 2.
  // - Listener and phase-2 Speaker hold the current named bid.
  const namesNextBid =
    game.currentPlayer === 2 && !game.listenerPassed
      ? true
      : game.currentPlayer === 0;

  // Determine button text based on role
  const positiveBidButtonText = namesNextBid
    ? `Raise (${nextBidValue})`
    : game.bidValue > 0
      ? `Accept (${game.bidValue})`
      : "Play";
  const isPositiveBidDisabled =
    isDisabled || (namesNextBid && nextBidValue === 0);

  if (!game.isMyTurn) {
    return (
      <div className="bidding-controls waiting">
        <span>Waiting for bid...</span>
      </div>
    );
  }

  return (
    <Box
      sx={{
        position: "absolute",
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
        textAlign: "center",
        zIndex: 50,
      }}
    >
      <Typography variant="h6" sx={{ mb: 2 }}>
        Current Bid: {currentBidText}
      </Typography>

      <Box
        sx={{
          display: "flex",
          gap: "12px",
          justifyContent: "center",
        }}
      >
        <Button
          variant="contained"
          color="success"
          onClick={() => game.controls.bid(true)}
          disabled={isPositiveBidDisabled}
          loading={game.controls.isLoading}
        >
          {positiveBidButtonText}
        </Button>
        <Button
          variant="outlined"
          color="warning"
          onClick={() => game.controls.bid(false)}
          disabled={isDisabled}
          loading={game.controls.isLoading}
        >
          Pass
        </Button>
      </Box>
    </Box>
  );
}
