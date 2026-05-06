package strategies

import (
	"math"
	"skat/game"
)

// WeightedHeuristicBiddingStrategy uses learned weights to evaluate bidding strength
type WeightedHeuristicBiddingStrategy struct {
	weights          BidWeights
	biddingThreshold float64 // Win probability threshold for bidding (default: 0.6)
}

// BidWeights contains learned weights for hand evaluation
type BidWeights struct {
	// Grand game weights
	GrandJacks        float64 // Weight per jack in hand
	GrandAces         float64 // Weight per ace
	GrandTens         float64 // Weight per ten
	GrandAceTenPairs  float64 // Weight for ace-ten combinations
	GrandTotalWinners float64 // Weight for (jacks + aces)

	// Suit game weights
	SuitTrumpLength   float64 // Weight per trump card
	SuitTrumpLengthSq float64 // Weight for trump_length^2 (captures trump dominance)
	SuitTopTrumps     float64 // Weight for having club/spade jack or trump ace
	SuitSideAces      float64 // Weight per ace in side suits
	SuitVoidSuits     float64 // Weight per void suit (good for ruffing)
	SuitShortSuits    float64 // Weight per singleton/doubleton
	SuitAceTenPairs   float64 // Weight for ace-ten pairs in side suits

	// Shared weights
	Matadors    float64 // Weight per matador (with or against)
	TotalPoints float64 // Weight for total card points in hand

	// Bias terms
	GrandBias float64 // Constant offset for grand evaluation
	SuitBias  float64 // Constant offset for suit evaluation
}

// DefaultBidWeights returns reasonable default weights (based on heuristic knowledge)
// These can be replaced with learned weights from training data
func DefaultBidWeights() BidWeights {
	return BidWeights{
		// Grand weights (trained from 50k episodes)
		GrandJacks:        12.002,
		GrandAces:         17.977,
		GrandTens:         4.962,
		GrandAceTenPairs:  19.975,
		GrandTotalWinners: 14.979,

		// Suit weights (balanced for selective but not overly conservative bidding)
		SuitTrumpLength:   5.0,
		SuitTrumpLengthSq: 1.5, // Moderate squared bonus
		SuitTopTrumps:     15.0,
		SuitSideAces:      10.0,
		SuitVoidSuits:     15.0,
		SuitShortSuits:    6.0,
		SuitAceTenPairs:   15.0,

		// Shared
		Matadors:    9.892,
		TotalPoints: -0.627,

		// Bias
		GrandBias: -20.0, // Modest penalty - Grand is viable with good jacks
		SuitBias:  0.0,
	}
}

// NewWeightedHeuristicBiddingStrategy creates a new weighted bidding strategy
func NewWeightedHeuristicBiddingStrategy() *WeightedHeuristicBiddingStrategy {
	return &WeightedHeuristicBiddingStrategy{
		weights:          DefaultBidWeights(),
		biddingThreshold: 0.70, // Higher threshold for more conservative bidding
	}
}

// NewWeightedHeuristicBiddingStrategyWithWeights creates strategy with custom weights
func NewWeightedHeuristicBiddingStrategyWithWeights(weights BidWeights, threshold float64) *WeightedHeuristicBiddingStrategy {
	return &WeightedHeuristicBiddingStrategy{
		weights:          weights,
		biddingThreshold: threshold,
	}
}

func (w *WeightedHeuristicBiddingStrategy) GetName() string {
	return "WeightedHeuristicBidding"
}

func (w *WeightedHeuristicBiddingStrategy) ShouldBid(gs *game.GameState, hand []game.Card, currentBid int) bool {
	cards := game.Cards(hand)

	// Get the next bid value
	nextBid := gs.GetNextBidValue()
	if nextBid == 0 {
		return false
	}

	// Evaluate all possible games and find the best one
	bestScore := -math.MaxFloat64
	bestGameValue := 0

	// Evaluate Grand
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)
	grandScore := w.evaluateGrand(cards)
	if grandScore > bestScore {
		bestScore = grandScore
		bestGameValue = grandValue
	}

	// Evaluate all suit games
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitValue := cards.GameValue(game.ModeSuit, suit)
		suitScore := w.evaluateSuit(cards, suit)
		if suitScore > bestScore {
			bestScore = suitScore
			bestGameValue = suitValue
		}
	}

	// Normalize score to approximate win probability (sigmoid-like)
	// Scores typically range from -100 to +200
	// Map to [0, 1] using a sigmoid: 1 / (1 + exp(-score/50))
	winProbability := 1.0 / (1.0 + math.Exp(-bestScore/50.0))

	// Apply safety margin to required game value
	// Game value must significantly exceed the bid to account for uncertainty
	safetyMargin := 1.2 // Need 50% more than bid value
	requiredValue := int(float64(nextBid) * safetyMargin)

	// Bid if:
	// 1. Win probability exceeds threshold, AND
	// 2. Game value meets the safety-adjusted requirement, AND
	// 3. Best score is positive and exceeds minimum threshold
	meetsThreshold := winProbability >= w.biddingThreshold
	meetsValue := bestGameValue >= requiredValue
	meetsStrength := bestScore > 110.0 // Balanced threshold for realistic bidding

	return meetsThreshold && meetsValue && meetsStrength
}

// evaluateGrand scores a hand for playing Grand
func (w *WeightedHeuristicBiddingStrategy) evaluateGrand(cards game.Cards) float64 {
	score := w.weights.GrandBias

	// Extract features
	jackCount := 0
	aceCount := 0
	tenCount := 0
	totalPoints := 0
	aceTenPairs := 0

	// Count jacks
	for _, card := range cards {
		totalPoints += card.Value()
		if card.Rank == game.Jack {
			jackCount++
		}
		if card.Rank == game.Ace {
			aceCount++
		}
		if card.Rank == game.Ten {
			tenCount++
		}
	}

	// Count ace-ten pairs
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		hasAce, hasTen := false, false
		for _, card := range cards {
			if card.Suit == suit {
				if card.Rank == game.Ace {
					hasAce = true
				}
				if card.Rank == game.Ten {
					hasTen = true
				}
			}
		}
		if hasAce && hasTen {
			aceTenPairs++
		}
	}

	// Grand requires good trump control
	if jackCount < 2 {
		return -100.0 // Nearly impossible without 3+ jacks
	}
	// 3 jacks is viable with good side suits - don't add extra penalty

	// Calculate matadors
	matadors := w.countMatadors(cards, game.ModeGrand, game.NoSuit)

	// Apply weights
	score += w.weights.GrandJacks * float64(jackCount)
	score += w.weights.GrandAces * float64(aceCount)
	score += w.weights.GrandTens * float64(tenCount)
	score += w.weights.GrandAceTenPairs * float64(aceTenPairs)
	score += w.weights.GrandTotalWinners * float64(jackCount+aceCount)
	score += w.weights.Matadors * float64(matadors)
	score += w.weights.TotalPoints * float64(totalPoints)

	return score
}

// evaluateSuit scores a hand for playing a specific suit game
func (w *WeightedHeuristicBiddingStrategy) evaluateSuit(cards game.Cards, trumpSuit game.Suit) float64 {
	score := w.weights.SuitBias

	// Extract features
	trumpCount := 0
	hasTopTrumps := false
	sideAces := 0
	voidSuits := 0
	shortSuits := 0
	aceTenPairs := 0
	totalPoints := 0

	// Count trumps and check for top trumps
	for _, card := range cards {
		totalPoints += card.Value()
		isTrump := card.Rank == game.Jack || card.Suit == trumpSuit
		if isTrump {
			trumpCount++
			// Check for top trumps (club jack, spade jack, trump ace)
			if card.Rank == game.Jack && (card.Suit == game.Clubs || card.Suit == game.Spades) {
				hasTopTrumps = true
			}
			if card.Suit == trumpSuit && card.Rank == game.Ace {
				hasTopTrumps = true
			}
		}
	}

	// Analyze side suits
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if suit == trumpSuit {
			continue
		}

		suitCards := []game.Card{}
		hasAce, hasTen := false, false

		for _, card := range cards {
			if card.Suit == suit && card.Rank != game.Jack {
				suitCards = append(suitCards, card)
				if card.Rank == game.Ace {
					hasAce = true
					sideAces++
				}
				if card.Rank == game.Ten {
					hasTen = true
				}
			}
		}

		// Count ace-ten pairs
		if hasAce && hasTen {
			aceTenPairs++
		}

		// Count void/short suits
		if len(suitCards) == 0 {
			voidSuits++
		} else if len(suitCards) <= 2 {
			shortSuits++
		}
	}

	// Calculate matadors
	matadors := w.countMatadors(cards, game.ModeSuit, trumpSuit)

	// Apply weights
	score += w.weights.SuitTrumpLength * float64(trumpCount)
	score += w.weights.SuitTrumpLengthSq * float64(trumpCount*trumpCount)
	if hasTopTrumps {
		score += w.weights.SuitTopTrumps
	}
	score += w.weights.SuitSideAces * float64(sideAces)
	score += w.weights.SuitVoidSuits * float64(voidSuits)
	score += w.weights.SuitShortSuits * float64(shortSuits)
	score += w.weights.SuitAceTenPairs * float64(aceTenPairs)
	score += w.weights.Matadors * float64(matadors)
	score += w.weights.TotalPoints * float64(totalPoints)

	return score
}

// countMatadors returns the absolute matador count
func (w *WeightedHeuristicBiddingStrategy) countMatadors(cards game.Cards, mode game.GameMode, trumpSuit game.Suit) int {
	jackSuits := make(map[game.Suit]bool)
	for _, card := range cards {
		if card.Rank == game.Jack {
			jackSuits[card.Suit] = true
		}
	}

	matadors := 0
	withJacks := jackSuits[game.Clubs]

	if withJacks {
		// "With" - count consecutive jacks from clubs
		if jackSuits[game.Clubs] {
			matadors++
			if jackSuits[game.Spades] {
				matadors++
				if jackSuits[game.Hearts] {
					matadors++
					if jackSuits[game.Diamonds] {
						matadors++
					}
				}
			}
		}
	} else {
		// "Without" - count how many top jacks are missing
		if !jackSuits[game.Clubs] {
			matadors++
			if !jackSuits[game.Spades] {
				matadors++
				if !jackSuits[game.Hearts] {
					matadors++
					if !jackSuits[game.Diamonds] {
						matadors++
					}
				}
			}
		}
	}

	return matadors
}

// SetWeights updates the weights (for training/tuning)
func (w *WeightedHeuristicBiddingStrategy) SetWeights(weights BidWeights) {
	w.weights = weights
}

// SetBiddingThreshold updates the bidding threshold
func (w *WeightedHeuristicBiddingStrategy) SetBiddingThreshold(threshold float64) {
	w.biddingThreshold = threshold
}

// GetWeights returns the current weights (for inspection/serialization)
func (w *WeightedHeuristicBiddingStrategy) GetWeights() BidWeights {
	return w.weights
}

// GetBiddingThreshold returns the current threshold
func (w *WeightedHeuristicBiddingStrategy) GetBiddingThreshold() float64 {
	return w.biddingThreshold
}

// WeightedHeuristicGameChoiceStrategy uses the same learned weights for game choice
type WeightedHeuristicGameChoiceStrategy struct {
	weights BidWeights
}

// NewWeightedHeuristicGameChoiceStrategy creates a new weighted game choice strategy
func NewWeightedHeuristicGameChoiceStrategy() *WeightedHeuristicGameChoiceStrategy {
	return &WeightedHeuristicGameChoiceStrategy{
		weights: DefaultBidWeights(),
	}
}

// NewWeightedHeuristicGameChoiceStrategyWithWeights creates strategy with custom weights
func NewWeightedHeuristicGameChoiceStrategyWithWeights(weights BidWeights) *WeightedHeuristicGameChoiceStrategy {
	return &WeightedHeuristicGameChoiceStrategy{
		weights: weights,
	}
}

func (w *WeightedHeuristicGameChoiceStrategy) GetName() string {
	return "WeightedHeuristicGameChoice"
}

// ChooseGame selects the best game mode and trump suit based on weighted evaluation
func (w *WeightedHeuristicGameChoiceStrategy) ChooseGame(hand []game.Card, bidValue int) (game.GameMode, game.Suit) {
	cards := game.Cards(hand)

	// Evaluate all possible games and find the best one
	bestScore := -math.MaxFloat64
	bestMode := game.ModeGrand
	bestSuit := game.NoSuit

	// Create evaluator for hand scoring
	evaluator := &WeightedHeuristicBiddingStrategy{weights: w.weights}

	// Evaluate Grand
	grandScore := evaluator.evaluateGrand(cards)
	grandValue := cards.GameValue(game.ModeGrand, game.NoSuit)

	// Only consider if it meets the bid value
	if grandValue >= bidValue && grandScore > bestScore {
		bestScore = grandScore
		bestMode = game.ModeGrand
		bestSuit = game.NoSuit
	}

	// Evaluate all suit games
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		suitScore := evaluator.evaluateSuit(cards, suit)
		suitValue := cards.GameValue(game.ModeSuit, suit)

		// Only consider if it meets the bid value
		if suitValue >= bidValue && suitScore > bestScore {
			bestScore = suitScore
			bestMode = game.ModeSuit
			bestSuit = suit
		}
	}

	return bestMode, bestSuit
}

// SetWeights updates the weights (for training/tuning)
func (w *WeightedHeuristicGameChoiceStrategy) SetWeights(weights BidWeights) {
	w.weights = weights
}

// GetWeights returns the current weights
func (w *WeightedHeuristicGameChoiceStrategy) GetWeights() BidWeights {
	return w.weights
}

// ChooseSkatDiscard uses the same heuristic as the standard strategy
// (skat discard is less amenable to simple weight-based optimization)
func (w *WeightedHeuristicGameChoiceStrategy) ChooseSkatDiscard(hand []game.Card, mode game.GameMode, trumpSuit game.Suit) (game.Card, game.Card) {
	// Delegate to heuristic strategy for skat discard
	h := &HeuristicGameChoiceStrategy{}
	return h.ChooseSkatDiscard(hand, mode, trumpSuit)
}
