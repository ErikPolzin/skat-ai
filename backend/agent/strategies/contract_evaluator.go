package strategies

import (
	"fmt"
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
	heuristic *HeuristicGameChoiceStrategy
}

func NewContractEvaluator() *ContractEvaluator {
	return NewContractEvaluatorWithConfig(DefaultContractEvaluatorConfig())
}

func NewContractEvaluatorWithConfig(config ContractEvaluatorConfig) *ContractEvaluator {
	if config.MinWinProbability == 0 {
		config.MinWinProbability = DefaultContractEvaluatorConfig().MinWinProbability
	}
	if config.LossMultiplier == 0 {
		config.LossMultiplier = DefaultContractEvaluatorConfig().LossMultiplier
	}
	return &ContractEvaluator{
		config:    config,
		heuristic: &HeuristicGameChoiceStrategy{},
	}
}

func (e *ContractEvaluator) Evaluate(hand []game.Card, bidValue int) []ContractCandidate {
	cards := game.Cards(hand)
	candidates := make([]ContractCandidate, 0, 6)

	candidates = append(candidates, e.candidate(cards, game.ModeGrand, game.NoSuit, bidValue))
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		candidates = append(candidates, e.candidate(cards, game.ModeSuit, suit, bidValue))
	}
	candidates = append(candidates, e.candidate(cards, game.ModeNull, game.NoSuit, bidValue))

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
	candidates := e.Evaluate(hand, bidValue)
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

func (e *ContractEvaluator) candidate(cards game.Cards, mode game.GameMode, suit game.Suit, bidValue int) ContractCandidate {
	gameValue := cards.GameValue(mode, suit)
	winProbability := e.winProbability(cards, mode, suit)
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
	switch mode {
	case game.ModeGrand:
		return e.heuristic.evaluateGrandStrength(cards)
	case game.ModeSuit:
		return e.heuristic.evaluateSuitStrength(cards, suit)
	case game.ModeNull:
		return e.heuristic.evaluateNullStrength(cards)
	default:
		return 0
	}
}

func expectedContractValue(gameValue float64, winProbability float64, lossMultiplier float64) float64 {
	return winProbability*gameValue - (1-winProbability)*gameValue*lossMultiplier
}
