package game

import (
	"testing"
)

// Test basic bidding scenarios
func TestBiddingPhases(t *testing.T) {
	tests := []struct {
		name           string
		bids           []bool // true = accept/hold, false = pass
		expectedWinner GamePosition
		expectedBid    int
	}{
		{
			name:           "Speaker passes, Dealer bids, Listener holds, Dealer raises, Listener passes",
			bids:           []bool{false, true, true, true, false}, // Speaker pass, Dealer bid 18, Listener hold, Dealer bid 20, Listener pass
			expectedWinner: Dealer,
			expectedBid:    20,
		},
		{
			name:           "Speaker passes, Dealer bids, Listener passes",
			bids:           []bool{false, true, false}, // Speaker pass, Dealer bid 18, Listener pass
			expectedWinner: Dealer,
			expectedBid:    18,
		},
		{
			name:           "Speaker passes, Dealer passes, Listener plays",
			bids:           []bool{false, false, true}, // Speaker pass, Dealer pass, Listener accepts -> Listener wins at 0
			expectedWinner: Listener,
			expectedBid:    0,
		},
		{
			name:           "Speaker bids, Listener passes, Dealer passes",
			bids:           []bool{true, false, false}, // Speaker bid 18, Listener pass, Dealer pass -> Speaker wins at 18
			expectedWinner: Speaker,
			expectedBid:    18,
		},
		{
			name:           "Speaker bids, Listener holds, Speaker passes, Dealer passes",
			bids:           []bool{true, true, false, false}, // Listener held 18 and is the last player left
			expectedWinner: Listener,
			expectedBid:    18,
		},
		{
			name:           "Speaker bids, Listener passes, Dealer bids, Speaker passes",
			bids:           []bool{true, false, true, false}, // Speaker bid 18, Listener pass, Dealer bid 20, Speaker pass
			expectedWinner: Dealer,
			expectedBid:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new game
			gs := NewGame()

			// Add three players
			gs.AddPlayer(&PlayerState{ID: "p1", Name: "Player1"})
			gs.AddPlayer(&PlayerState{ID: "p2", Name: "Player2"})
			gs.AddPlayer(&PlayerState{ID: "p3", Name: "Player3"})

			// Deal cards
			gs.Deal()

			// Process bids
			for i, bid := range tt.bids {
				if gs.Phase != PhaseBidding {
					break // Bidding ended
				}
				action := "pass"
				if bid {
					action = "accept/bid"
				}
				t.Logf("Bid %d: Player %d %s (current bid: %d)", i, gs.CurrentPlayer, action, gs.BidValue)
				_, err := gs.Bid(bid)
				if err != nil {
					t.Fatalf("Bid %d failed: %v", i, err)
				}
				t.Logf("  After: Player %d's turn, bid value: %d, phase: %s", gs.CurrentPlayer, gs.BidValue, gs.Phase)
			}

			// Check results
			if gs.Phase != PhaseSkatExchange {
				t.Errorf("Expected phase %s, got %s", PhaseSkatExchange, gs.Phase)
			}
			if gs.Declarer == nil {
				t.Errorf("Expected winner %d, got nil", tt.expectedWinner)
			} else if *gs.Declarer != tt.expectedWinner {
				t.Errorf("Expected winner %d, got %d", tt.expectedWinner, *gs.Declarer)
			}
			if tt.expectedBid > 0 && gs.BidValue != tt.expectedBid {
				t.Errorf("Expected bid value %d, got %d", tt.expectedBid, gs.BidValue)
			}
		})
	}
}

func newBiddingGameForTest(policy PassPolicy) *GameState {
	gs := NewGame()
	gs.PassPolicy = policy
	gs.AddPlayer(&PlayerState{ID: "dealer", Name: "Dealer"})
	gs.AddPlayer(&PlayerState{ID: "listener", Name: "Listener"})
	gs.AddPlayer(&PlayerState{ID: "speaker", Name: "Speaker"})
	gs.Deal()
	return gs
}

func TestListenerCanChooseAfterSpeakerAndDealerPassAtZero(t *testing.T) {
	gs := newBiddingGameForTest(PassPolicyForceListener)

	if _, err := gs.Bid(false); err != nil {
		t.Fatalf("speaker pass failed: %v", err)
	}
	if gs.CurrentPlayer != Dealer {
		t.Fatalf("expected dealer to act after speaker pass, got %d", gs.CurrentPlayer)
	}

	if _, err := gs.Bid(false); err != nil {
		t.Fatalf("dealer pass failed: %v", err)
	}
	if gs.Phase != PhaseBidding {
		t.Fatalf("expected bidding to continue for listener decision, got %s", gs.Phase)
	}
	if gs.CurrentPlayer != Listener {
		t.Fatalf("expected listener to get all-pass decision, got %d", gs.CurrentPlayer)
	}
	if gs.Declarer != nil {
		t.Fatalf("expected no declarer before listener decision, got %d", *gs.Declarer)
	}
}

func TestListenerCanVoluntarilyPlayAtZeroAfterOthersPass(t *testing.T) {
	gs := newBiddingGameForTest(PassPolicyReshuffle)

	gs.Bid(false) // Speaker passes
	gs.Bid(false) // Dealer passes
	if _, err := gs.Bid(true); err != nil {
		t.Fatalf("listener accept failed: %v", err)
	}

	if gs.Phase != PhaseSkatExchange {
		t.Fatalf("expected skat exchange, got %s", gs.Phase)
	}
	if gs.Declarer == nil || *gs.Declarer != Listener {
		t.Fatalf("expected listener declarer, got %v", gs.Declarer)
	}
	if gs.BidValue != 0 {
		t.Fatalf("expected zero bid, got %d", gs.BidValue)
	}
}

func TestAllPassPolicyForceListenerAfterExplicitListenerPass(t *testing.T) {
	gs := newBiddingGameForTest(PassPolicyForceListener)

	gs.Bid(false) // Speaker passes
	gs.Bid(false) // Dealer passes
	if _, err := gs.Bid(false); err != nil {
		t.Fatalf("listener pass failed: %v", err)
	}

	if gs.Phase != PhaseSkatExchange {
		t.Fatalf("expected forced listener skat exchange, got %s", gs.Phase)
	}
	if gs.Declarer == nil || *gs.Declarer != Listener {
		t.Fatalf("expected listener declarer, got %v", gs.Declarer)
	}
	if !gs.AllPlayersPassed() {
		t.Fatalf("expected all players passed")
	}
}

func TestAllPassPolicyReshuffleAfterExplicitListenerPass(t *testing.T) {
	gs := newBiddingGameForTest(PassPolicyReshuffle)

	gs.Bid(false) // Speaker passes
	gs.Bid(false) // Dealer passes
	if _, err := gs.Bid(false); err != nil {
		t.Fatalf("listener pass failed: %v", err)
	}

	if gs.Phase != PhaseDealing {
		t.Fatalf("expected redeal phase, got %s", gs.Phase)
	}
	if gs.Declarer != nil {
		t.Fatalf("expected no declarer after reshuffle, got %d", *gs.Declarer)
	}
	if gs.SpeakerPassed || gs.DealerPassed || gs.ListenerPassed {
		t.Fatalf("expected pass flags reset after reshuffle")
	}
}

func TestAllPassPolicyRamschAfterExplicitListenerPass(t *testing.T) {
	gs := newBiddingGameForTest(PassPolicyRamsch)

	gs.Bid(false) // Speaker passes
	gs.Bid(false) // Dealer passes
	if _, err := gs.Bid(false); err != nil {
		t.Fatalf("listener pass failed: %v", err)
	}

	if gs.Phase != PhasePlaying {
		t.Fatalf("expected Ramsch to start playing, got %s", gs.Phase)
	}
	if gs.Mode != ModeRamsch {
		t.Fatalf("expected Ramsch mode, got %s", gs.Mode)
	}
	if gs.Declarer != nil {
		t.Fatalf("expected no declarer in Ramsch, got %d", *gs.Declarer)
	}
	if gs.CurrentPlayer != Listener {
		t.Fatalf("expected forehand/listener to lead Ramsch, got %d", gs.CurrentPlayer)
	}
}

// Test that bidding starts with Speaker
func TestBiddingStartsWithSpeaker(t *testing.T) {
	gs := NewGame()
	gs.AddPlayer(&PlayerState{ID: "p1", Name: "Player1"})
	gs.AddPlayer(&PlayerState{ID: "p2", Name: "Player2"})
	gs.AddPlayer(&PlayerState{ID: "p3", Name: "Player3"})

	gs.Deal()

	if gs.CurrentPlayer != Speaker {
		t.Errorf("Bidding should start with Speaker (2), got %d", gs.CurrentPlayer)
	}
}

// Test Speaker bids, Listener holds, Speaker raises
func TestSpeakerListenerBidding(t *testing.T) {
	gs := NewGame()
	gs.AddPlayer(&PlayerState{ID: "p1", Name: "Player1"})
	gs.AddPlayer(&PlayerState{ID: "p2", Name: "Player2"})
	gs.AddPlayer(&PlayerState{ID: "p3", Name: "Player3"})

	gs.Deal()

	// Speaker bids 18
	gs.Bid(true)
	if gs.BidValue != 18 {
		t.Errorf("Expected bid 18, got %d", gs.BidValue)
	}
	if gs.CurrentPlayer != Listener {
		t.Errorf("Expected Listener's turn, got %d", gs.CurrentPlayer)
	}

	// Listener holds (accepts the bid of 18)
	gs.Bid(true)
	if gs.BidValue != 18 {
		t.Errorf("Expected bid to stay at 18 when Listener holds, got %d", gs.BidValue)
	}
	if gs.CurrentPlayer != Speaker {
		t.Errorf("Expected Speaker's turn, got %d", gs.CurrentPlayer)
	}

	// Speaker must now raise to 20
	gs.Bid(true)
	if gs.BidValue != 20 {
		t.Errorf("Expected bid raised to 20, got %d", gs.BidValue)
	}
	if gs.CurrentPlayer != Listener {
		t.Errorf("Expected Listener's turn, got %d", gs.CurrentPlayer)
	}
}
