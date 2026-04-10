package agent

import (
	"math"
	"math/rand"
	"skat/game"
)

// BiddingAgent learns to bid based on hand strength
type BiddingAgent struct {
	// Q-table: map[handFeatures][bidLevel] -> expectedValue
	// Simplified: use hand strength score as index
	qTable map[int]map[int]float64

	// Learning parameters
	alpha      float64 // Learning rate
	gamma      float64 // Discount factor
	Epsilon    float64 // Exploration rate (exported)

	// Track current episode (exported for trainer access)
	CurrentHandScore int
	CurrentBid       int
}

func NewBiddingAgent() *BiddingAgent {
	return &BiddingAgent{
		qTable:  make(map[int]map[int]float64),
		alpha:   0.1,
		gamma:   0.9,
		Epsilon: 0.15,
	}
}

// Bid decides whether to bid and at what value
func (ba *BiddingAgent) Bid(state *game.GameState, currentBid int) int {
	handScore := ba.evaluateHand(state.Players[state.CurrentPlayer].Hand)
	ba.CurrentHandScore = handScore

	// Get Q-values for this hand
	if ba.qTable[handScore] == nil {
		ba.qTable[handScore] = make(map[int]float64)
	}

	// Possible bid levels in Skat: 18, 20, 22, 23, 24, 27, 30, 33, 35, 36, etc.
	// Simplified: bid in increments, or pass (0)
	possibleBids := []int{0} // 0 means pass
	for bid := currentBid + 1; bid <= 60; bid++ {
		possibleBids = append(possibleBids, bid)
	}

	// Epsilon-greedy selection
	var selectedBid int
	if rand.Float64() < ba.Epsilon {
		// Explore: random bid
		selectedBid = possibleBids[rand.Intn(len(possibleBids))]
	} else {
		// Exploit: choose best Q-value
		bestBid := 0
		bestQ := ba.getQ(handScore, 0)

		for _, bid := range possibleBids {
			q := ba.getQ(handScore, bid)
			if q > bestQ {
				bestQ = q
				bestBid = bid
			}
		}
		selectedBid = bestBid
	}

	ba.CurrentBid = selectedBid
	return selectedBid
}

// OnGameEnd updates Q-values based on game outcome
func (ba *BiddingAgent) OnGameEnd(wonBid bool, becameDeclarer bool, wonGame bool, gameValue int, pointsScored int) {
	// Calculate reward with better credit assignment
	reward := 0.0

	if !becameDeclarer {
		// Didn't become declarer - check for regret
		if wonBid {
			// We bid but someone outbid us - small penalty
			reward = -0.05
		} else {
			// We passed - but was it a mistake?
			// REGRET PENALTY: If declarer got crushed, we should have bid!
			if !wonGame && pointsScored < 40 {
				// Declarer got destroyed - we missed an opportunity
				// Scale penalty by hand strength
				regretPenalty := -0.3 * (float64(ba.CurrentHandScore) / 100.0)
				reward = regretPenalty
			} else if !wonGame && pointsScored < 50 {
				// Declarer barely lost - maybe we could have won
				regretPenalty := -0.15 * (float64(ba.CurrentHandScore) / 100.0)
				reward = regretPenalty
			} else {
				// Declarer won or played well - passing was safe
				reward = 0.05
			}
		}
	} else {
		// We became declarer - stronger signal
		if wonGame {
			// Won the game - BIG reward
			// Base reward + bonus for overbidding safety margin
			safetyMargin := float64(pointsScored-61) / 60.0 // 0 to 1
			reward = 1.0 + safetyMargin*0.5

			// OPPORTUNITY BONUS: Extra reward for bidding on strong hands
			if ba.CurrentHandScore >= 70 {
				reward += 0.2 // Strong hand AND won - excellent!
			}
		} else {
			// Lost the game - but consider how close we were
			if pointsScored >= 55 {
				// Very close to winning - small penalty (maybe unlucky)
				reward = -0.2
				// OPPORTUNITY BONUS: Reduce penalty if hand was strong (worth the risk)
				if ba.CurrentHandScore >= 70 {
					reward += 0.1 // Made a reasonable attempt
				}
			} else if pointsScored >= 45 {
				// Reasonably close - medium penalty
				reward = -0.5
			} else {
				// Got crushed - big penalty, we overbid badly
				reward = -1.0
			}

			// Extra penalty for high bids that failed badly
			if ba.CurrentBid > 30 && pointsScored < 40 {
				reward -= 0.5
			}
		}
	}

	// Q-learning update
	oldQ := ba.getQ(ba.CurrentHandScore, ba.CurrentBid)
	newQ := oldQ + ba.alpha*(reward-oldQ)
	ba.setQ(ba.CurrentHandScore, ba.CurrentBid, newQ)
}

// evaluateHand returns a score representing hand strength
func (ba *BiddingAgent) evaluateHand(hand []game.Card) int {
	score := 0

	// Count jacks (most important in Grand)
	jacks := 0
	jackSuits := make(map[game.Suit]bool)

	// Count aces and tens (high value cards)
	aces := 0
	tens := 0

	// Count suits
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		if card.Rank == game.Ace {
			aces++
		}
		if card.Rank == game.Ten {
			tens++
		}
		suitCounts[card.Suit]++

		// Add card value
		score += card.Value()
	}

	// Bonus for jacks (they're trump in most games)
	score += jacks * 15

	// Bonus for having top jacks in sequence (matadors)
	if jackSuits[game.Clubs] {
		score += 10
		if jackSuits[game.Spades] {
			score += 8
			if jackSuits[game.Hearts] {
				score += 6
			}
		}
	}

	// Bonus for long suits (good for suit games)
	for _, count := range suitCounts {
		if count >= 5 {
			score += (count - 4) * 3
		}
	}

	// Normalize to 0-100 range
	// Max possible: ~120 points in cards + 40 for jacks + 24 matadors + suits
	if score > 100 {
		score = 100
	}

	return score
}

func (ba *BiddingAgent) getQ(handScore, bid int) float64 {
	if ba.qTable[handScore] == nil {
		return 0.0
	}
	return ba.qTable[handScore][bid]
}

func (ba *BiddingAgent) setQ(handScore, bid int, value float64) {
	if ba.qTable[handScore] == nil {
		ba.qTable[handScore] = make(map[int]float64)
	}
	ba.qTable[handScore][bid] = value
}

// SetQ is exported for testing/pre-training
func (ba *BiddingAgent) SetQ(handScore, bid int, value float64) {
	ba.setQ(handScore, bid, value)
}

// DecayEpsilon reduces exploration over time
func (ba *BiddingAgent) DecayEpsilon(minEpsilon float64) {
	ba.Epsilon = math.Max(minEpsilon, ba.Epsilon*0.995)
}

// GetQTableSize returns the number of states learned
func (ba *BiddingAgent) GetQTableSize() int {
	total := 0
	for _, bids := range ba.qTable {
		total += len(bids)
	}
	return total
}

// ChooseGameMode selects the best game mode based on hand composition
func (ba *BiddingAgent) ChooseGameMode(hand []game.Card) (game.GameMode, game.Suit) {
	// Count jacks and suits
	jacks := 0
	jackSuits := make(map[game.Suit]bool)
	suitCounts := make(map[game.Suit]int)
	suitPoints := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Rank == game.Jack {
			jacks++
			jackSuits[card.Suit] = true
		}
		suitCounts[card.Suit]++
		suitPoints[card.Suit] += card.Value()
	}

	// Strategy 1: Grand if we have 3+ jacks
	if jacks >= 3 {
		return game.ModeGrand, game.Clubs // Suit doesn't matter for Grand
	}

	// Strategy 2: Grand if we have top jacks (matadors)
	if jackSuits[game.Clubs] && jackSuits[game.Spades] {
		return game.ModeGrand, game.Clubs
	}

	// Strategy 3: Suit game - pick suit with most cards + highest points
	bestSuit := game.Clubs
	bestScore := 0

	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		// Score = card count * 10 + points
		score := suitCounts[suit]*10 + suitPoints[suit]
		if score > bestScore {
			bestScore = score
			bestSuit = suit
		}
	}

	// If best suit has 5+ cards, play it
	if suitCounts[bestSuit] >= 5 {
		return game.ModeSuit, bestSuit
	}

	// Default: Grand with whatever jacks we have
	return game.ModeGrand, game.Clubs
}

// GetBestAction returns the best action and its Q-value for a given hand score
func (ba *BiddingAgent) GetBestAction(handScore, currentBid int) (int, float64) {
	if ba.qTable[handScore] == nil {
		return 0, 0.0 // No learned strategy, default to pass
	}

	bestBid := 0
	bestQ := ba.getQ(handScore, 0) // Start with passing

	// Check all possible bids
	for bid := currentBid + 1; bid <= 60; bid++ {
		q := ba.getQ(handScore, bid)
		if q > bestQ {
			bestQ = q
			bestBid = bid
		}
	}

	return bestBid, bestQ
}
