package strategies

import (
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"skat/game"
	"sort"
)

const handWinPredictorVersion = 7
const handWinPredictorLevels = 4

// HandWinStats aggregates declarer outcomes for one abstract hand position.
type HandWinStats struct {
	Count uint64
	Wins  uint64
}

func (s HandWinStats) Probability() float64 {
	// A light beta prior prevents tiny pure buckets from claiming certainty.
	return float64(s.Wins+1) / float64(s.Count+2)
}

func (s HandWinStats) Error() float64 {
	p := s.Probability()
	return math.Sqrt(p * (1 - p) / float64(s.Count+2))
}

// HandWinPredictor predicts declarer win probability in small-hand states.
type HandWinPredictor struct {
	Version    int
	MaxCards   int
	MaxBuckets int
	Buckets    map[uint64]HandWinStats
}

type HandWinEstimate struct {
	WinProbability float64
	Error          float64
	Samples        uint64
	Level          int
}

func NewHandWinPredictor(maxCards int) *HandWinPredictor {
	if maxCards <= 0 {
		maxCards = 6
	}
	return &HandWinPredictor{
		Version:  handWinPredictorVersion,
		MaxCards: maxCards,
		Buckets:  make(map[uint64]HandWinStats),
	}
}

func LoadHandWinPredictor(path string) (*HandWinPredictor, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var predictor HandWinPredictor
	if err := gob.NewDecoder(file).Decode(&predictor); err != nil {
		return nil, fmt.Errorf("decode hand win predictor: %w", err)
	}
	if predictor.Version != handWinPredictorVersion {
		return nil, fmt.Errorf("unsupported hand win predictor version %d", predictor.Version)
	}
	if predictor.Buckets == nil {
		predictor.Buckets = make(map[uint64]HandWinStats)
	}
	return &predictor, nil
}

func (p *HandWinPredictor) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(file).Encode(p); err != nil {
		file.Close()
		return fmt.Errorf("encode hand win predictor: %w", err)
	}
	return file.Close()
}

func (p *HandWinPredictor) Observe(state *game.GameState, declarerWon bool) bool {
	_, ok := p.key(state)
	if !ok {
		return false
	}
	cardsPerHand := len(state.Players[0].Hand)
	for level := 0; level < handWinPredictorLevels; level++ {
		if (level == 0 && cardsPerHand > 3) || (level == 1 && cardsPerHand > 4) {
			continue
		}
		key, _ := p.keyAtLevel(state, level)
		stats, found := p.Buckets[key]
		if !found && p.MaxBuckets > 0 && len(p.Buckets) >= p.MaxBuckets {
			continue
		}
		stats.Count++
		if declarerWon {
			stats.Wins++
		}
		p.Buckets[key] = stats
	}
	return true
}

func (p *HandWinPredictor) Merge(other *HandWinPredictor) {
	for key, incoming := range other.Buckets {
		current, found := p.Buckets[key]
		if !found {
			if p.MaxBuckets > 0 && len(p.Buckets) >= p.MaxBuckets {
				continue
			}
			p.Buckets[key] = incoming
			continue
		}
		current.Count += incoming.Count
		current.Wins += incoming.Wins
		p.Buckets[key] = current
	}
}

func (p *HandWinPredictor) Lookup(state *game.GameState, minSamples uint64) (HandWinEstimate, bool) {
	for level := 0; level < handWinPredictorLevels; level++ {
		key, ok := p.keyAtLevel(state, level)
		if !ok {
			return HandWinEstimate{}, false
		}
		stats, found := p.Buckets[key]
		if !found || stats.Count < minSamples {
			continue
		}
		return HandWinEstimate{
			WinProbability: stats.Probability(),
			Error:          stats.Error(),
			Samples:        stats.Count,
			Level:          level,
		}, true
	}
	return HandWinEstimate{}, false
}

func pendingHandWinSkatPoints(state *game.GameState) int {
	if state.Phase == game.PhaseComplete || (state.Mode != game.ModeSuit && state.Mode != game.ModeGrand) {
		return 0
	}
	return state.Skat[0].Value() + state.Skat[1].Value()
}

type handWinSuitSignature struct {
	values [9]uint8
}

func (p *HandWinPredictor) key(state *game.GameState) (uint64, bool) {
	return p.keyAtLevel(state, 0)
}

func (p *HandWinPredictor) keyAtLevel(state *game.GameState, level int) (uint64, bool) {
	if state.Declarer == nil || len(state.Trick) != 0 || state.Phase != game.PhasePlaying {
		return 0, false
	}
	if state.Mode != game.ModeSuit && state.Mode != game.ModeGrand {
		return 0, false
	}
	if state.Players[0] == nil {
		return 0, false
	}
	cardsPerHand := len(state.Players[0].Hand)
	if cardsPerHand < 1 || cardsPerHand > p.MaxCards {
		return 0, false
	}
	for _, player := range state.Players[1:] {
		if player == nil || len(player.Hand) != cardsPerHand {
			return 0, false
		}
	}

	declarer := int(*state.Declarer)
	hash := uint64(1469598103934665603)
	mix := func(value uint64) {
		hash ^= value + 1
		hash *= 1099511628211
	}
	mix(uint64(level))
	mix(uint64(cardsPerHand))
	if state.Mode == game.ModeGrand {
		mix(1)
	} else {
		mix(2)
	}
	mix(uint64((int(state.CurrentPlayer) - declarer + 3) % 3))
	mix(uint64((int(state.TrickStarter) - declarer + 3) % 3))
	target := 61
	if state.AnnouncedSchneider {
		target = 90
	}
	if state.AnnouncedSchwarz {
		target = 120
	}
	pointsNeeded := max(0, target-state.DeclarerCardScore()-pendingHandWinSkatPoints(state))
	if level == 1 {
		pointsNeeded /= 3
	} else if level == 2 {
		pointsNeeded /= 4
	} else if level >= 3 {
		pointsNeeded /= 8
	}
	mix(uint64(pointsNeeded))
	if state.Overbid {
		mix(1)
	} else {
		mix(0)
	}
	if level >= 2 {
		for relative := 0; relative < 3; relative++ {
			position := (declarer + relative) % 3
			trumps, points, highCards := 0, 0, 0
			nonTrumpSuits := make(map[game.Suit]bool)
			for _, card := range state.Players[position].Hand {
				points += card.Value()
				if card.IsTrump(state.Mode, state.TrumpSuit) {
					trumps++
				} else {
					nonTrumpSuits[card.Suit] = true
				}
				if card.Rank == game.Ace || card.Rank == game.Ten {
					highCards++
				}
			}
			availableSuits := 4
			if state.Mode == game.ModeSuit {
				availableSuits = 3
			}
			if level == 2 {
				mix(uint64(trumps))
				mix(uint64(points / 8))
				mix(uint64(highCards))
				mix(uint64(availableSuits - len(nonTrumpSuits)))
			} else {
				mix(uint64(trumps / 2))
				mix(uint64(points / 12))
				mix(uint64(highCards))
				mix(uint64(availableSuits - len(nonTrumpSuits)))
			}
		}
		return hash, true
	}

	// Trump summaries retain exact trump strength while bucketing point totals.
	for relative := 0; relative < 3; relative++ {
		position := (declarer + relative) % 3
		trumpCount, trumpPoints, strongest := 0, 0, 0
		for _, card := range state.Players[position].Hand {
			if card.IsTrump(state.Mode, state.TrumpSuit) {
				trumpCount++
				trumpPoints += card.Value()
				strongest = max(strongest, state.TrumpValue(card))
			}
		}
		mix(uint64(trumpCount))
		pointDivisor, strengthDivisor := 4, 2
		if level == 1 {
			pointDivisor, strengthDivisor = 8, 4
		} else if level >= 2 {
			pointDivisor, strengthDivisor = 12, 6
		}
		mix(uint64(trumpPoints / pointDivisor))
		mix(uint64(strongest / strengthDivisor))
	}

	// Non-trump suits are trick-play equivalent. Sorting their cross-player
	// summaries canonicalizes clubs/spades/hearts/diamonds without losing which
	// players share or are void in a suit.
	var suits []handWinSuitSignature
	for suit := game.Clubs; suit <= game.Diamonds; suit++ {
		if state.Mode == game.ModeSuit && suit == state.TrumpSuit {
			continue
		}
		var signature handWinSuitSignature
		for relative := 0; relative < 3; relative++ {
			position := (declarer + relative) % 3
			count, points, strongest := 0, 0, 0
			for _, card := range state.Players[position].Hand {
				if card.Rank == game.Jack || card.Suit != suit {
					continue
				}
				count++
				points += card.Value()
				strongest = max(strongest, card.Rank.SkatRank())
			}
			offset := relative * 3
			signature.values[offset] = uint8(count)
			pointDivisor, strengthDivisor := 4, 2
			if level == 1 {
				pointDivisor, strengthDivisor = 8, 4
			} else if level >= 2 {
				pointDivisor, strengthDivisor = 12, 32
			}
			signature.values[offset+1] = uint8(points / pointDivisor)
			signature.values[offset+2] = uint8(strongest / strengthDivisor)
		}
		suits = append(suits, signature)
	}
	sort.Slice(suits, func(i, j int) bool {
		for k := range suits[i].values {
			if suits[i].values[k] != suits[j].values[k] {
				return suits[i].values[k] < suits[j].values[k]
			}
		}
		return false
	})
	for _, signature := range suits {
		for _, value := range signature.values {
			mix(uint64(value))
		}
	}
	return hash, true
}
