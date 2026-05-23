package rating

import (
	"fmt"
	"math"

	"skat/game"
)

const InitialRating = 20

type PlayerRating struct {
	ProfileID   string
	Rating      int
	GamesPlayed int
	Wins        int
	Losses      int
	PeakRating  int
}

type PlayerStats struct {
	GamesPlayed int
	Wins        int
	Losses      int
}

// CalculateTournamentWeight returns the BSkA tournament weight:
// W = 1 - exp(-sqrt(P*H)/100), where P is player count and H is hands per player.
func CalculateTournamentWeight(playerCount, handsPerPlayer int) float64 {
	if playerCount <= 0 || handsPerPlayer <= 0 {
		return 0
	}
	playerHands := float64(playerCount * handsPerPlayer)
	return 1 - math.Exp(-math.Sqrt(playerHands)/100)
}

// CalculateNewRating applies the BSkA formula:
// (1-W)*R + 100*W*M/T.
func CalculateNewRating(previousRating int, matchPoints, totalMatchPoints, weight float64) int {
	if totalMatchPoints <= 0 || weight <= 0 {
		return previousRating
	}
	scorePercentage := 100 * matchPoints / totalMatchPoints
	return int(math.Round((1-weight)*float64(previousRating) + weight*scorePercentage))
}

// UpdateRatings updates ratings for all players and populates rating fields in results.
// Session standings are converted to match points by awarding one point for each
// opponent beaten and half a point for each tied opponent.
func UpdateRatings(results []game.PlayerSessionResultState, ratings map[string]*PlayerRating, handsPerPlayer int, stats map[string]PlayerStats) error {
	if len(results) < 2 {
		return fmt.Errorf("expected at least 2 session results, got %d", len(results))
	}
	weight := CalculateTournamentWeight(len(results), handsPerPlayer)
	totalMatchPoints := float64(len(results) - 1)

	ratingChanges := make(map[string]int)
	for _, result := range results {
		playerRating, ok := ratings[result.PlayerID]
		if !ok {
			return fmt.Errorf("missing rating for player %s", result.PlayerID)
		}

		matchPoints := 0.0
		for _, opponent := range results {
			if opponent.PlayerID == result.PlayerID {
				continue
			}
			switch {
			case result.IsForfeit:
			case opponent.IsForfeit:
				matchPoints += 1.0
			case result.PlayerPoints > opponent.PlayerPoints:
				matchPoints += 1.0
			case result.PlayerPoints == opponent.PlayerPoints:
				matchPoints += 0.5
			}
		}

		newRating := CalculateNewRating(playerRating.Rating, matchPoints, totalMatchPoints, weight)
		ratingChanges[result.PlayerID] = newRating - playerRating.Rating
	}

	for i := range results {
		playerRating := ratings[results[i].PlayerID]
		change := ratingChanges[results[i].PlayerID]
		results[i].RatingBefore = playerRating.Rating
		playerRating.Rating += change
		playerStats := stats[results[i].PlayerID]
		playerRating.GamesPlayed += playerStats.GamesPlayed
		playerRating.Wins += playerStats.Wins
		playerRating.Losses += playerStats.Losses
		if playerRating.Rating > playerRating.PeakRating {
			playerRating.PeakRating = playerRating.Rating
		}
		results[i].RatingAfter = playerRating.Rating
		results[i].RatingChange = change
	}

	return nil
}
