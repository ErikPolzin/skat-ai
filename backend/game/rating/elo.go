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
// In Skat, the declarer plays against two opponents as a team
func UpdateRatings(gameState *game.GameState, results *[3]game.PlayerResultState, ratings map[string]*PlayerRating) error {
	if gameState.Phase != game.PhaseComplete {
		return fmt.Errorf("game is not complete")
	}

	// Get game result
	declarerWon, _, _ := gameState.GetGameResult()
	gameValue := gameState.Result().Value

	// Get declarer
	if gameState.Declarer == nil {
		return fmt.Errorf("declarer not set")
	}
	declarer := gameState.Players[*gameState.Declarer]
	if declarer == nil {
		return fmt.Errorf("declarer not found")
	}

	// Count AI opponents and scale K-factor accordingly
	aiCount := 0
	for _, player := range gameState.Players {
		if player != nil && player.IsAgent {
			aiCount++
		}
	}

	// K-factor multiplier based on AI opponents: 1.0 (0 AIs), 0.5 (1 AI), 0.25 (2 AIs)
	aiMultiplier := 1.0
	if aiCount == 1 {
		aiMultiplier = 0.5
	} else if aiCount >= 2 {
		aiMultiplier = 0.25
	}

	// Get opponents and calculate average opponent rating
	var opponents []*game.PlayerState
	var opponentRatings []int
	for pos, player := range gameState.Players {
		if player != nil && game.GamePosition(pos) != *gameState.Declarer {
			opponents = append(opponents, player)
			opponentRatings = append(opponentRatings, ratings[player.ID].Rating)
		}
	}

	if len(opponents) != 2 {
		return fmt.Errorf("expected 2 opponents, got %d", len(opponents))
	}

	avgOpponentRating := (opponentRatings[0] + opponentRatings[1]) / 2

	// Calculate rating changes
	declarerActualScore := 0.0
	if declarerWon {
		declarerActualScore = 1.0
	}
	opponentActualScore := 1.0 - declarerActualScore

	// Map to store rating changes
	ratingChanges := make(map[string]int)

	// Calculate declarer's rating change
	declarerRating := ratings[declarer.ID]
	declarerChange := CalculateRatingChange(
		declarerRating.Rating,
		avgOpponentRating,
		declarerActualScore,
		gameValue,
		declarerRating.GamesPlayed,
	)
	declarerChange = int(float64(declarerChange) * aiMultiplier)
	ratingChanges[declarer.ID] = declarerChange

	// Update declarer rating
	declarerRating.Rating += declarerChange
	declarerRating.GamesPlayed++
	if declarerWon {
		declarerRating.Wins++
	} else {
		declarerRating.Losses++
	}
	if declarerRating.Rating > declarerRating.PeakRating {
		declarerRating.PeakRating = declarerRating.Rating
	}

	// Update opponent ratings
	// Note: Opponents who passed don't get wins/losses/games_played updated
	// Only their rating is adjusted to reflect the declarer's performance
	for _, opponent := range opponents {
		opponentRating := ratings[opponent.ID]

		// Opponents play as a team, so they share the result
		opponentChange := CalculateRatingChange(
			opponentRating.Rating,
			declarerRating.Rating-declarerChange, // Use old declarer rating
			opponentActualScore,
			gameValue,
			opponentRating.GamesPlayed,
		)
		opponentChange = int(float64(opponentChange) * aiMultiplier)
		ratingChanges[opponent.ID] = opponentChange

		opponentRating.Rating += opponentChange
		// Don't update games_played, wins, or losses for opponents
		// Only the declarer's stats are updated
		if opponentRating.Rating > opponentRating.PeakRating {
			opponentRating.PeakRating = opponentRating.Rating
		}
	}

	// Update results with rating information
	for i := range results {
		playerID := results[i].PlayerID
		if change, ok := ratingChanges[playerID]; ok {
			rating := ratings[playerID]
			results[i].RatingBefore = rating.Rating - change
			results[i].RatingAfter = rating.Rating
			results[i].RatingChange = change
		}
	}

	return nil
}
