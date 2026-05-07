package weighted

import (
	"fmt"
	"math"
	"skat/agent/strategies"
	"skat/game"
)

// BiddingExample represents a single bidding training example
type BiddingExample struct {
	Hand     []game.Card    // The player's hand
	Mode     game.GameMode  // The actual game mode played
	TrumpSuit game.Suit     // The trump suit (for suit games)
	DidWin   bool           // Whether the player won the game
	Quality  float64        // Quality score (0-1, higher is better)
}

// WeightTrainer trains weights for weighted heuristic bidding using gradient descent
type WeightTrainer struct {
	learningRate     float64
	tempLearningRate float64 // Separate learning rate for temperature (usually smaller)
	epochs           int
}

// NewWeightTrainer creates a new weight trainer
func NewWeightTrainer(learningRate float64, epochs int) *WeightTrainer {
	return &WeightTrainer{
		learningRate:     learningRate,
		tempLearningRate: learningRate * 0.1, // Temperature learns slower
		epochs:           epochs,
	}
}

// FeatureVector represents extracted features from a hand
type FeatureVector struct {
	// Grand features
	GrandJacks        float64
	GrandAces         float64
	GrandTens         float64
	GrandAceTenPairs  float64
	GrandTotalWinners float64

	// Suit features (averaged across all suits or for best suit)
	SuitTrumpLength   float64
	SuitTrumpLengthSq float64
	SuitTopTrumps     float64
	SuitSideAces      float64
	SuitVoidSuits     float64
	SuitShortSuits    float64
	SuitAceTenPairs   float64

	// Shared features
	Matadors    float64
	TotalPoints float64

	// Target
	DidWin       bool    // Whether the player won the game
	WasDeclarer  bool    // Whether the player became declarer
	ChosenGrand  bool    // Whether grand was chosen
	BestGameMode game.GameMode
	BestSuit     game.Suit
}

// TrainWeights learns optimal weights from bidding examples using gradient descent
func (t *WeightTrainer) TrainWeights(examples []BiddingExample) strategies.BidWeights {
	// Extract features from examples
	features := t.extractFeatures(examples)

	// Initialize weights to defaults
	weights := strategies.DefaultBidWeights()

	// Separate grand and suit examples
	grandFeatures := []FeatureVector{}
	suitFeatures := []FeatureVector{}

	for _, f := range features {
		if f.ChosenGrand {
			grandFeatures = append(grandFeatures, f)
		} else {
			suitFeatures = append(suitFeatures, f)
		}
	}

	fmt.Printf("Training on %d examples (%d grand, %d suit)\n", len(features), len(grandFeatures), len(suitFeatures))

	// Train grand weights
	if len(grandFeatures) > 0 {
		weights = t.trainGrandWeights(weights, grandFeatures)
	}

	// Train suit weights
	if len(suitFeatures) > 0 {
		weights = t.trainSuitWeights(weights, suitFeatures)
	}

	return weights
}

// trainGrandWeights trains weights for grand evaluation
func (t *WeightTrainer) trainGrandWeights(weights strategies.BidWeights, features []FeatureVector) strategies.BidWeights {
	fmt.Println("Training grand weights...")

	for epoch := 1; epoch <= t.epochs; epoch++ {
		totalLoss := 0.0
		correct := 0

		for _, f := range features {
			// Forward pass: compute score
			score := weights.GrandBias
			score += weights.GrandJacks * f.GrandJacks
			score += weights.GrandAces * f.GrandAces
			score += weights.GrandTens * f.GrandTens
			score += weights.GrandAceTenPairs * f.GrandAceTenPairs
			score += weights.GrandTotalWinners * f.GrandTotalWinners
			score += weights.Matadors * f.Matadors
			score += weights.TotalPoints * f.TotalPoints

			// Convert to probability using sigmoid
			temperature := weights.SigmoidTemperature
			if temperature == 0 {
				temperature = 50.0
			}
			predicted := 1.0 / (1.0 + math.Exp(-score/temperature))

			// Target: 1.0 if won, 0.0 if lost
			var target float64
			if f.DidWin {
				target = 1.0
			} else {
				target = 0.0
			}

			// Compute loss (cross-entropy)
			loss := -target*math.Log(predicted+1e-10) - (1-target)*math.Log(1-predicted+1e-10)
			totalLoss += loss

			// Track accuracy
			if (predicted >= 0.5 && f.DidWin) || (predicted < 0.5 && !f.DidWin) {
				correct++
			}

			// Compute gradient (derivative of cross-entropy w.r.t. score)
			// d(loss)/d(score) = (predicted - target) / temperature
			gradient := (predicted - target) / temperature

			// Compute gradient w.r.t. temperature
			// d(loss)/d(temperature) = (predicted - target) * score / (temperature^2)
			tempGradient := (predicted - target) * score / (temperature * temperature)

			// Update weights (gradient descent)
			weights.GrandBias -= t.learningRate * gradient
			weights.GrandJacks -= t.learningRate * gradient * f.GrandJacks
			weights.GrandAces -= t.learningRate * gradient * f.GrandAces
			weights.GrandTens -= t.learningRate * gradient * f.GrandTens
			weights.GrandAceTenPairs -= t.learningRate * gradient * f.GrandAceTenPairs
			weights.GrandTotalWinners -= t.learningRate * gradient * f.GrandTotalWinners
			weights.Matadors -= t.learningRate * gradient * f.Matadors
			weights.TotalPoints -= t.learningRate * gradient * f.TotalPoints

			// Update temperature
			weights.SigmoidTemperature -= t.tempLearningRate * tempGradient
			// Clamp temperature to reasonable range
			if weights.SigmoidTemperature < 10.0 {
				weights.SigmoidTemperature = 10.0
			} else if weights.SigmoidTemperature > 200.0 {
				weights.SigmoidTemperature = 200.0
			}
		}

		if epoch%10 == 0 {
			avgLoss := totalLoss / float64(len(features))
			accuracy := float64(correct) / float64(len(features))
			fmt.Printf("  Epoch %d: Loss=%.4f, Accuracy=%.3f\n", epoch, avgLoss, accuracy)
		}
	}

	return weights
}

// trainSuitWeights trains weights for suit game evaluation
func (t *WeightTrainer) trainSuitWeights(weights strategies.BidWeights, features []FeatureVector) strategies.BidWeights {
	fmt.Println("Training suit weights...")

	for epoch := 1; epoch <= t.epochs; epoch++ {
		totalLoss := 0.0
		correct := 0

		for _, f := range features {
			// Forward pass: compute score
			score := weights.SuitBias
			score += weights.SuitTrumpLength * f.SuitTrumpLength
			score += weights.SuitTrumpLengthSq * f.SuitTrumpLengthSq
			score += weights.SuitTopTrumps * f.SuitTopTrumps
			score += weights.SuitSideAces * f.SuitSideAces
			score += weights.SuitVoidSuits * f.SuitVoidSuits
			score += weights.SuitShortSuits * f.SuitShortSuits
			score += weights.SuitAceTenPairs * f.SuitAceTenPairs
			score += weights.Matadors * f.Matadors
			score += weights.TotalPoints * f.TotalPoints

			// Convert to probability using sigmoid
			temperature := weights.SigmoidTemperature
			if temperature == 0 {
				temperature = 50.0
			}
			predicted := 1.0 / (1.0 + math.Exp(-score/temperature))

			// Target: 1.0 if won, 0.0 if lost
			var target float64
			if f.DidWin {
				target = 1.0
			} else {
				target = 0.0
			}

			// Compute loss (cross-entropy)
			loss := -target*math.Log(predicted+1e-10) - (1-target)*math.Log(1-predicted+1e-10)
			totalLoss += loss

			// Track accuracy
			if (predicted >= 0.5 && f.DidWin) || (predicted < 0.5 && !f.DidWin) {
				correct++
			}

			// Compute gradient
			gradient := (predicted - target) / temperature

			// Compute gradient w.r.t. temperature
			tempGradient := (predicted - target) * score / (temperature * temperature)

			// Update weights
			weights.SuitBias -= t.learningRate * gradient
			weights.SuitTrumpLength -= t.learningRate * gradient * f.SuitTrumpLength
			weights.SuitTrumpLengthSq -= t.learningRate * gradient * f.SuitTrumpLengthSq
			weights.SuitTopTrumps -= t.learningRate * gradient * f.SuitTopTrumps
			weights.SuitSideAces -= t.learningRate * gradient * f.SuitSideAces
			weights.SuitVoidSuits -= t.learningRate * gradient * f.SuitVoidSuits
			weights.SuitShortSuits -= t.learningRate * gradient * f.SuitShortSuits
			weights.SuitAceTenPairs -= t.learningRate * gradient * f.SuitAceTenPairs
			weights.Matadors -= t.learningRate * gradient * f.Matadors
			weights.TotalPoints -= t.learningRate * gradient * f.TotalPoints

			// Update temperature
			weights.SigmoidTemperature -= t.tempLearningRate * tempGradient
			// Clamp temperature to reasonable range
			if weights.SigmoidTemperature < 10.0 {
				weights.SigmoidTemperature = 10.0
			} else if weights.SigmoidTemperature > 200.0 {
				weights.SigmoidTemperature = 200.0
			}
		}

		if epoch%10 == 0 {
			avgLoss := totalLoss / float64(len(features))
			accuracy := float64(correct) / float64(len(features))
			fmt.Printf("  Epoch %d: Loss=%.4f, Accuracy=%.3f\n", epoch, avgLoss, accuracy)
		}
	}

	return weights
}

// extractFeatures converts bidding examples to feature vectors
func (t *WeightTrainer) extractFeatures(examples []BiddingExample) []FeatureVector {
	features := make([]FeatureVector, 0, len(examples))

	for _, ex := range examples {
		hand := ex.Hand
		cards := game.Cards(hand)

		// Use the actual game mode that was played
		actualMode := ex.Mode
		actualSuit := ex.TrumpSuit

		// Extract features
		fv := FeatureVector{
			BestGameMode: actualMode,
			BestSuit:     actualSuit,
			ChosenGrand:  actualMode == game.ModeGrand,
			DidWin:       ex.DidWin,
		}

		// Extract grand features
		fv.GrandJacks = float64(t.countJacks(hand))
		fv.GrandAces = float64(t.countAces(hand))
		fv.GrandTens = float64(t.countTens(hand))
		fv.GrandAceTenPairs = float64(t.countAceTenPairs(hand, game.NoSuit))
		fv.GrandTotalWinners = fv.GrandJacks + fv.GrandAces

		// Extract suit features (for actual suit played)
		if actualMode == game.ModeSuit {
			fv.SuitTrumpLength = float64(t.countTrumps(hand, actualSuit))
			fv.SuitTrumpLengthSq = fv.SuitTrumpLength * fv.SuitTrumpLength
			fv.SuitTopTrumps = t.boolToFloat(t.hasTopTrumps(hand, actualSuit))
			fv.SuitSideAces = float64(t.countSideAces(hand, actualSuit))
			fv.SuitVoidSuits = float64(t.countVoidSuits(hand, actualSuit))
			fv.SuitShortSuits = float64(t.countShortSuits(hand, actualSuit))
			fv.SuitAceTenPairs = float64(t.countAceTenPairs(hand, actualSuit))
		}

		// Shared features
		fv.Matadors = float64(t.countMatadors(cards, actualMode, actualSuit))
		fv.TotalPoints = float64(t.countPoints(hand))

		features = append(features, fv)
	}

	return features
}

// Helper functions to extract features from hands

func (t *WeightTrainer) countJacks(hand []game.Card) int {
	count := 0
	for _, card := range hand {
		if card.Rank == game.Jack {
			count++
		}
	}
	return count
}

func (t *WeightTrainer) countAces(hand []game.Card) int {
	count := 0
	for _, card := range hand {
		if card.Rank == game.Ace {
			count++
		}
	}
	return count
}

func (t *WeightTrainer) countTens(hand []game.Card) int {
	count := 0
	for _, card := range hand {
		if card.Rank == game.Ten {
			count++
		}
	}
	return count
}

func (t *WeightTrainer) countAceTenPairs(hand []game.Card, trumpSuit game.Suit) int {
	pairs := 0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if suit == trumpSuit {
			continue // Skip trump suit for side suit analysis
		}

		hasAce, hasTen := false, false
		for _, card := range hand {
			if card.Suit == suit && card.Rank != game.Jack {
				if card.Rank == game.Ace {
					hasAce = true
				}
				if card.Rank == game.Ten {
					hasTen = true
				}
			}
		}
		if hasAce && hasTen {
			pairs++
		}
	}
	return pairs
}

func (t *WeightTrainer) countTrumps(hand []game.Card, trumpSuit game.Suit) int {
	count := 0
	for _, card := range hand {
		if card.Rank == game.Jack || card.Suit == trumpSuit {
			count++
		}
	}
	return count
}

func (t *WeightTrainer) hasTopTrumps(hand []game.Card, trumpSuit game.Suit) bool {
	for _, card := range hand {
		if card.Rank == game.Jack && (card.Suit == game.Clubs || card.Suit == game.Spades) {
			return true
		}
		if card.Suit == trumpSuit && card.Rank == game.Ace {
			return true
		}
	}
	return false
}

func (t *WeightTrainer) countSideAces(hand []game.Card, trumpSuit game.Suit) int {
	count := 0
	for _, card := range hand {
		if card.Rank == game.Ace && card.Suit != trumpSuit {
			count++
		}
	}
	return count
}

func (t *WeightTrainer) countVoidSuits(hand []game.Card, trumpSuit game.Suit) int {
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Suit != trumpSuit && card.Rank != game.Jack {
			suitCounts[card.Suit]++
		}
	}

	voids := 0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if suit != trumpSuit && suitCounts[suit] == 0 {
			voids++
		}
	}
	return voids
}

func (t *WeightTrainer) countShortSuits(hand []game.Card, trumpSuit game.Suit) int {
	suitCounts := make(map[game.Suit]int)

	for _, card := range hand {
		if card.Suit != trumpSuit && card.Rank != game.Jack {
			suitCounts[card.Suit]++
		}
	}

	shorts := 0
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if suit != trumpSuit && suitCounts[suit] > 0 && suitCounts[suit] <= 2 {
			shorts++
		}
	}
	return shorts
}

func (t *WeightTrainer) countMatadors(cards game.Cards, mode game.GameMode, trumpSuit game.Suit) int {
	jackSuits := make(map[game.Suit]bool)
	for _, card := range cards {
		if card.Rank == game.Jack {
			jackSuits[card.Suit] = true
		}
	}

	matadors := 0
	withJacks := jackSuits[game.Clubs]

	if withJacks {
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

func (t *WeightTrainer) countPoints(hand []game.Card) int {
	points := 0
	for _, card := range hand {
		points += card.Value()
	}
	return points
}

func (t *WeightTrainer) boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
