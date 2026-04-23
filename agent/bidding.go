package agent

import (
	"math"
	"math/rand"
	"skat/game"
)

// Bid uses Q-learning to make bidding decisions (accept=true, pass=false)
func (sa *SkatAgent) Bid(state *game.GameState) bool {
	handScore := sa.evaluateHand(state.Players[state.CurrentPlayer].Hand)
	sa.CurrentHandScore = handScore

	if sa.qTable[handScore] == nil {
		sa.qTable[handScore] = make(map[int]float64)
	}

	// Two actions: pass (0) or accept (which will raise bid to nextBid)
	// We need to evaluate accepting at the NEXT bid value, since accepting raises the bid
	nextBid := state.GetNextBidValue()

	// If no higher bid is available, must pass
	if nextBid == 0 {
		sa.CurrentBid = 0
		return false
	}

	qPass := sa.getQ(handScore, 0)
	qAccept := sa.getQ(handScore, nextBid)

	var accept bool
	if rand.Float64() < sa.Epsilon {
		// Explore: random choice
		accept = rand.Float64() < 0.5
	} else {
		// Exploit: choose best action
		// If qAccept is untrained (zero), use heuristic instead of random
		if qAccept == 0.0 && qPass == 0.0 {
			accept = sa.heuristicBid(state.Players[state.CurrentPlayer].Hand, nextBid)
		} else if qAccept > qPass {
			accept = true
		} else if qPass > qAccept {
			accept = false
		} else {
			// Tied (both non-zero) - randomize to avoid systematic bias
			accept = rand.Float64() < 0.5
		}
	}

	if accept {
		sa.CurrentBid = nextBid // Store the bid we're committing to (next value)
	} else {
		sa.CurrentBid = 0
	}

	return accept
}

// OnGameEnd updates Q-values based on game outcome
func (sa *SkatAgent) OnGameEnd(becameDeclarer bool, wonGame bool, gameValue int, pointsScored int) {
	reward := 0.0

	if becameDeclarer {
		// Agent became declarer - reward based on game outcome
		if wonGame {
			// Base reward + bonus for safety margin
			safetyMargin := float64(pointsScored-61) / 60.0
			reward = 1.0 + safetyMargin*0.5
		} else {
			// Lost as declarer - penalize based on how badly
			if pointsScored >= 55 {
				reward = -0.3 // Close loss
			} else if pointsScored >= 45 {
				reward = -0.6 // Moderate loss
			} else {
				reward = -1.0 // Bad loss
			}

			// Extra penalty for overbidding badly
			if sa.CurrentBid > 30 && pointsScored < 40 {
				reward -= 0.5
			}
		}
	} else {
		// Agent did not become declarer
		if sa.CurrentBid == 0 {
			// Agent passed
			if wonGame {
				// Declarer won - slight penalty for missing opportunity
				reward = -0.1
			} else {
				// Declarer lost - small reward for correctly passing
				reward = 0.1
			}
		} else {
			// Agent bid but didn't become declarer - slight penalty for losing bidding war
			reward = -0.05
		}
	}

	oldQ := sa.getQ(sa.CurrentHandScore, sa.CurrentBid)
	newQ := oldQ + sa.alpha*(reward-oldQ)
	sa.setQ(sa.CurrentHandScore, sa.CurrentBid, newQ)
}

// DecayEpsilon reduces exploration over time
func (sa *SkatAgent) DecayEpsilon(minEpsilon float64) {
	sa.Epsilon = math.Max(minEpsilon, sa.Epsilon*0.995)
}

// GetQTableSize returns the number of states learned
func (sa *SkatAgent) GetQTableSize() int {
	total := 0
	for _, bids := range sa.qTable {
		total += len(bids)
	}
	return total
}

// GetBestAction returns the best action and its Q-value for a given hand score
func (sa *SkatAgent) GetBestAction(handScore, currentBid int) (int, float64) {
	if sa.qTable[handScore] == nil {
		return 0, 0.0
	}

	bestBid := 0
	bestQ := sa.getQ(handScore, 0)

	for bid := currentBid + 1; bid <= 60; bid++ {
		q := sa.getQ(handScore, bid)
		if q > bestQ {
			bestQ = q
			bestBid = bid
		}
	}

	return bestBid, bestQ
}

func (sa *SkatAgent) SetQ(handScore, bid int, value float64) {
	sa.setQ(handScore, bid, value)
}

// HandState represents the state for Q-learning based on high-value card counts
type HandState struct {
	Aces          int
	Tens          int
	Jacks         int
	Trumps        int // Largest suit count (for suit games)
	GamesPlayable int // Number of games that can be played at current bid
}

// encodeHandState converts (aces, tens, jacks, trumps, gamesPlayable) into a single integer key
// State space: 5×5×5×11×7 = 9,625 possible states
// - aces, tens, jacks: 0-4 each
// - trumps (longest suit): 0-10
// - gamesPlayable: 0-6 (Grand + 4 suits + Null = 6 max)
func encodeHandState(aces, tens, jacks, trumps, gamesPlayable int) int {
	return aces*1925 + tens*385 + jacks*77 + trumps*7 + gamesPlayable
}

// evaluateHand returns a state encoding based on high-value card counts
func (sa *SkatAgent) evaluateHand(hand []game.Card) int {
	aces := 0
	tens := 0
	jacks := 0
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
		}
		if card.Rank == game.Ace {
			aces++
		}
		if card.Rank == game.Ten {
			tens++
		}
		suitCounts[card.Suit]++
	}

	// Find longest suit (best potential trump suit)
	maxSuitCount := 0
	for _, count := range suitCounts {
		if count > maxSuitCount {
			maxSuitCount = count
		}
	}

	// Calculate how many games can be played at current bid level
	cards := game.Cards(hand)
	gamesPlayable := cards.CountGamesPlayable(sa.CurrentBid)

	return encodeHandState(aces, tens, jacks, maxSuitCount, gamesPlayable)
}

func (sa *SkatAgent) getQ(handScore, bid int) float64 {
	if sa.qTable[handScore] == nil {
		return 0.0
	}
	return sa.qTable[handScore][bid]
}

func (sa *SkatAgent) setQ(handScore, bid int, value float64) {
	if sa.qTable[handScore] == nil {
		sa.qTable[handScore] = make(map[int]float64)
	}
	sa.qTable[handScore][bid] = value
}

// heuristicBid decides whether to accept a bid based on hand strength
func (sa *SkatAgent) heuristicBid(hand []game.Card, bidValue int) bool {
	cards := game.Cards(hand)

	// Count how many games can be played at this bid value
	gamesPlayable := cards.CountGamesPlayable(bidValue)

	// If no games can be played at this bid, definitely pass
	if gamesPlayable == 0 {
		return false
	}

	// Find the best possible game value for this hand
	bestGameValue := 0

	// Check Grand
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	if grandValue > bestGameValue {
		bestGameValue = grandValue
	}

	// Check each suit
	for _, suit := range []game.Suit{game.Diamonds, game.Hearts, game.Spades, game.Clubs} {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		if suitValue > bestGameValue {
			bestGameValue = suitValue
		}
	}

	// Accept if we have a comfortable margin (at least 20% above bid)
	if bestGameValue >= bidValue*12/10 {
		return true
	}

	// Accept if we can play multiple games and bid is reasonable
	if gamesPlayable >= 3 && bestGameValue >= bidValue {
		return true
	}

	// Pass if it's too close
	return false
}
