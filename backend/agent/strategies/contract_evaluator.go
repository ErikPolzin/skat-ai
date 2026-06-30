package strategies

import (
	"fmt"
	"math"
	"skat/game"
	"sort"
)

// ContractCandidate is one playable contract the agent could choose.
type ContractCandidate struct {
	Mode      game.GameMode
	TrumpSuit game.Suit

	GameValue      int
	WinProbability float64
	ExpectedValue  float64
	LegalForBid    bool

	Reason string
}

// GameChoice is an atomic post-skat decision: the contract and the two cards
// discarded for that exact contract.
type GameChoice struct {
	Mode      game.GameMode
	TrumpSuit game.Suit
	Discard   [2]game.Card
}

// ContractWinProbabilityEstimator supplies the probability model used by
// ContractEvaluator. This lets the same candidate-ranking logic use heuristic,
// neural, or test probability sources.
type ContractWinProbabilityEstimator interface {
	EstimateWinProbability(hand game.Cards, mode game.GameMode, suit game.Suit) float64
}

// EstimateRamschWinProbabilities estimates each player's chance of finishing
// with the lowest card-point score. Unlike contract strength, Ramsch strength
// is relative to all three hands: point cards and cards likely to take tricks
// are liabilities. The result is a normalized three-player distribution.
func EstimateRamschWinProbabilities(hands [3][]game.Card) [3]float64 {
	var liability [3]float64
	for player, hand := range hands {
		for _, card := range hand {
			liability[player] += float64(card.Value())
			switch card.Rank {
			case game.Jack:
				// Jacks are Ramsch trumps; stronger jacks are harder to avoid winning with.
				liability[player] += float64(8 - int(card.Suit))
			case game.Ace:
				liability[player] += 4
			case game.Ten:
				liability[player] += 3
			case game.King:
				liability[player]++
			}
		}
	}

	minLiability := math.Min(liability[0], math.Min(liability[1], liability[2]))
	var weights [3]float64
	total := 0.0
	for player := range weights {
		weights[player] = math.Exp(-(liability[player] - minLiability) / 12.0)
		total += weights[player]
	}
	var probabilities [3]float64
	for player := range probabilities {
		probabilities[player] = weights[player] / total
	}
	return probabilities
}

// ContractEvaluatorConfig controls the risk preferences used by the shared
// bidding/game-choice evaluator.
type ContractEvaluatorConfig struct {
	MinWinProbability float64
	MinExpectedValue  float64
	LossMultiplier    float64
}

// DefaultContractEvaluatorConfig is tuned to behave like the stronger
// threshold-swept heuristic while still ranking contracts by risk-adjusted EV.
func DefaultContractEvaluatorConfig() ContractEvaluatorConfig {
	return ContractEvaluatorConfig{
		MinWinProbability: 0.55,
		MinExpectedValue:  0.0,
		LossMultiplier:    1.2,
	}
}

// ContractEvaluator scores every candidate contract from the same hand model.
// Bidding and game choice both use this evaluator so they do not drift apart.
type ContractEvaluator struct {
	config    ContractEvaluatorConfig
	estimator ContractWinProbabilityEstimator
}

func NewContractEvaluator() *ContractEvaluator {
	return NewContractEvaluatorWithConfig(DefaultContractEvaluatorConfig())
}

func NewContractEvaluatorWithConfig(config ContractEvaluatorConfig) *ContractEvaluator {
	return NewContractEvaluatorWithEstimator(config, NewHeuristicContractWinProbabilityEstimator())
}

func NewContractEvaluatorWithEstimator(config ContractEvaluatorConfig, estimator ContractWinProbabilityEstimator) *ContractEvaluator {
	if config.MinWinProbability == 0 {
		config.MinWinProbability = DefaultContractEvaluatorConfig().MinWinProbability
	}
	if config.LossMultiplier == 0 {
		config.LossMultiplier = DefaultContractEvaluatorConfig().LossMultiplier
	}
	if estimator == nil {
		estimator = NewHeuristicContractWinProbabilityEstimator()
	}
	return &ContractEvaluator{
		config:    config,
		estimator: estimator,
	}
}

func (e *ContractEvaluator) Evaluate(hand []game.Card, bidValue int) []ContractCandidate {
	return e.evaluate(hand, bidValue, nil)
}

// evaluate scores contracts using the original hand for game value and an
// optional contract-specific hand for win probability. Game choice uses this
// to account for the two cards it will discard; bidding has no skat yet and
// therefore uses the original hand for both.
func (e *ContractEvaluator) evaluate(
	hand []game.Card,
	bidValue int,
	probabilityHand func(game.GameMode, game.Suit) game.Cards,
) []ContractCandidate {
	cards := game.Cards(hand)
	candidates := make([]ContractCandidate, 0, 6)

	addCandidate := func(mode game.GameMode, suit game.Suit) {
		evaluationCards := cards
		if probabilityHand != nil {
			evaluationCards = probabilityHand(mode, suit)
		}
		candidates = append(candidates, e.candidate(cards, evaluationCards, mode, suit, bidValue))
	}

	addCandidate(game.ModeGrand, game.NoSuit)
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		addCandidate(game.ModeSuit, suit)
	}
	addCandidate(game.ModeNull, game.NoSuit)

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].LegalForBid != candidates[j].LegalForBid {
			return candidates[i].LegalForBid
		}
		if candidates[i].ExpectedValue != candidates[j].ExpectedValue {
			return candidates[i].ExpectedValue > candidates[j].ExpectedValue
		}
		if candidates[i].WinProbability != candidates[j].WinProbability {
			return candidates[i].WinProbability > candidates[j].WinProbability
		}
		return candidates[i].GameValue > candidates[j].GameValue
	})

	return candidates
}

func (e *ContractEvaluator) Best(hand []game.Card, bidValue int) (ContractCandidate, bool) {
	return e.best(e.Evaluate(hand, bidValue))
}

func (e *ContractEvaluator) best(candidates []ContractCandidate) (ContractCandidate, bool) {
	for _, candidate := range candidates {
		if e.IsAcceptable(candidate) {
			return candidate, true
		}
	}
	if len(candidates) == 0 {
		return ContractCandidate{}, false
	}
	return candidates[0], false
}

func (e *ContractEvaluator) IsAcceptable(candidate ContractCandidate) bool {
	return candidate.LegalForBid &&
		candidate.WinProbability >= e.config.MinWinProbability &&
		candidate.ExpectedValue >= e.config.MinExpectedValue
}

func (e *ContractEvaluator) candidate(cards, evaluationCards game.Cards, mode game.GameMode, suit game.Suit, bidValue int) ContractCandidate {
	gameValue := cards.GameValue(mode, suit)
	winProbability := e.winProbability(evaluationCards, mode, suit)
	expectedValue := expectedContractValue(float64(gameValue), winProbability, e.config.LossMultiplier)

	return ContractCandidate{
		Mode:           mode,
		TrumpSuit:      suit,
		GameValue:      gameValue,
		WinProbability: winProbability,
		ExpectedValue:  expectedValue,
		LegalForBid:    gameValue >= bidValue,
		Reason:         fmt.Sprintf("p=%.2f value=%d ev=%.1f", winProbability, gameValue, expectedValue),
	}
}

func (e *ContractEvaluator) winProbability(cards game.Cards, mode game.GameMode, suit game.Suit) float64 {
	return e.estimator.EstimateWinProbability(cards, mode, suit)
}

// EstimateContractWinProbability estimates the declarer's chance of winning
// from the final ten-card hand and the contract that will actually be played.
// It is strategy-independent so evaluation can normalize every agent against
// the same hand-strength model.
func EstimateContractWinProbability(hand []game.Card, mode game.GameMode, suit game.Suit) float64 {
	return NewHeuristicContractWinProbabilityEstimator().EstimateWinProbability(game.Cards(hand), mode, suit)
}

func expectedContractValue(gameValue float64, winProbability float64, lossMultiplier float64) float64 {
	return winProbability*gameValue - (1-winProbability)*gameValue*lossMultiplier
}
