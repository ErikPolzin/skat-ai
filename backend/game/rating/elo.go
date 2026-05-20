package rating

import (
	"fmt"
	"math"

	"skat/game"
)

type PlayerRating struct {
	ProfileID   string
	Rating      int
	GamesPlayed int
	Wins        int
	Losses      int
	PeakRating  int
}

// CalculateExpectedScore calculates the expected score for a player
// using the standard ELO formula
func CalculateExpectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10.0, float64(ratingB-ratingA)/400.0))
}

// GetKFactor returns the K-factor based on games played
// New players (<30 games): 32
// Intermediate players (30-100 games): 24
// Established players (>100 games): 16
func GetKFactor(gamesPlayed int) int {
	if gamesPlayed < 30 {
		return 32
	} else if gamesPlayed < 100 {
		return 24
	}
	return 16
}

// CalculateGameValueMultiplier returns a multiplier based on the game value
// Higher stakes games should have more impact on ratings
// Uses sqrt(|game_value| / 60) to normalize the impact
func CalculateGameValueMultiplier(gameValue int) float64 {
	absValue := math.Abs(float64(gameValue))
	// Normalize around typical game value of 60
	// sqrt gives diminishing returns for very high values
	return math.Sqrt(absValue / 60.0)
}

// CalculateRatingChange calculates the rating change for a player
func CalculateRatingChange(rating, opponentRating int, actualScore float64, gameValue int, gamesPlayed int) int {
	expectedScore := CalculateExpectedScore(rating, opponentRating)
	kFactor := GetKFactor(gamesPlayed)
	gameMultiplier := CalculateGameValueMultiplier(gameValue)

	// Base rating change
	change := float64(kFactor) * (actualScore - expectedScore)

	// Apply game value multiplier
	change *= gameMultiplier

	return int(math.Round(change))
}

// UpdateRatings updates ratings for all players and populates rating fields in results
func UpdateRatings(results []game.PlayerSessionResultState, ratings map[string]*PlayerRating, aiCount int) error {
	if len(results) < 2 {
		return fmt.Errorf("expected at least 2 session results, got %d", len(results))
	}

	aiMultiplier := 1.0
	if aiCount == 1 {
		aiMultiplier = 0.5
	} else if aiCount >= 2 {
		aiMultiplier = 0.25
	}

	topScore := results[0].PlayerPoints
	bottomScore := results[0].PlayerPoints
	for _, result := range results {
		if result.PlayerPoints > topScore {
			topScore = result.PlayerPoints
		}
		if result.PlayerPoints < bottomScore {
			bottomScore = result.PlayerPoints
		}
	}
	scoreSpread := topScore - bottomScore
	if scoreSpread < 60 {
		scoreSpread = 60
	}

	ratingChanges := make(map[string]int)
	for _, result := range results {
		playerRating, ok := ratings[result.PlayerID]
		if !ok {
			return fmt.Errorf("missing rating for player %s", result.PlayerID)
		}

		actualTotal := 0.0
		expectedTotal := 0.0
		opponents := 0
		for _, opponent := range results {
			if opponent.PlayerID == result.PlayerID {
				continue
			}
			opponentRating, ok := ratings[opponent.PlayerID]
			if !ok {
				return fmt.Errorf("missing rating for opponent %s", opponent.PlayerID)
			}
			switch {
			case result.IsForfeit:
			case opponent.IsForfeit:
				actualTotal += 1.0
			case result.PlayerPoints > opponent.PlayerPoints:
				actualTotal += 1.0
			case result.PlayerPoints == opponent.PlayerPoints:
				actualTotal += 0.5
			}
			expectedTotal += CalculateExpectedScore(playerRating.Rating, opponentRating.Rating)
			opponents++
		}
		if opponents == 0 {
			continue
		}

		actualScore := actualTotal / float64(opponents)
		expectedScore := expectedTotal / float64(opponents)
		kFactor := GetKFactor(playerRating.GamesPlayed)
		change := float64(kFactor) * (actualScore - expectedScore)
		change *= CalculateGameValueMultiplier(scoreSpread)
		change = change * aiMultiplier
		ratingChanges[result.PlayerID] = int(math.Round(change))
	}

	for i := range results {
		playerRating := ratings[results[i].PlayerID]
		change := ratingChanges[results[i].PlayerID]
		results[i].RatingBefore = playerRating.Rating
		playerRating.Rating += change
		playerRating.GamesPlayed++
		if results[i].IsWinner {
			playerRating.Wins++
		} else {
			playerRating.Losses++
		}
		if playerRating.Rating > playerRating.PeakRating {
			playerRating.PeakRating = playerRating.Rating
		}
		results[i].RatingAfter = playerRating.Rating
		results[i].RatingChange = change
	}

	return nil
}
