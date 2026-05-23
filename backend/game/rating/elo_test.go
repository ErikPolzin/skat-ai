package rating

import (
	"testing"

	"skat/game"
)

func TestUpdateRatingsTreatsForfeitAsLoss(t *testing.T) {
	results := []game.PlayerSessionResultState{
		{SessionID: "session", PlayerID: "leader", PlayerPoints: 200, IsWinner: false, IsForfeit: true},
		{SessionID: "session", PlayerID: "second", PlayerPoints: 100, IsWinner: true},
		{SessionID: "session", PlayerID: "third", PlayerPoints: 50, IsWinner: false},
	}
	ratings := map[string]*PlayerRating{
		"leader": {ProfileID: "leader", Rating: InitialRating, GamesPlayed: 10, PeakRating: InitialRating},
		"second": {ProfileID: "second", Rating: InitialRating, GamesPlayed: 10, PeakRating: InitialRating},
		"third":  {ProfileID: "third", Rating: InitialRating, GamesPlayed: 10, PeakRating: InitialRating},
	}

	stats := map[string]PlayerStats{
		"leader": {GamesPlayed: 1, Losses: 1},
		"second": {GamesPlayed: 2, Wins: 1, Losses: 1},
	}
	if err := UpdateRatings(results, ratings, 10, stats); err != nil {
		t.Fatalf("UpdateRatings returned error: %v", err)
	}

	if results[0].RatingChange >= 0 {
		t.Fatalf("expected forfeiting leader to lose rating, got change %d", results[0].RatingChange)
	}
	if results[1].RatingChange <= 0 {
		t.Fatalf("expected non-forfeiting player to gain rating, got change %d", results[1].RatingChange)
	}
	if ratings["leader"].Wins != 0 || ratings["leader"].Losses != 1 {
		t.Fatalf("expected forfeiting player to record a loss, got %d wins and %d losses", ratings["leader"].Wins, ratings["leader"].Losses)
	}
	if ratings["second"].GamesPlayed != 12 || ratings["third"].GamesPlayed != 10 {
		t.Fatalf("expected stats to add declarer games only, got second=%d third=%d", ratings["second"].GamesPlayed, ratings["third"].GamesPlayed)
	}
}

func TestCalculateTournamentWeightMatchesBSkAExample(t *testing.T) {
	weight := CalculateTournamentWeight(15, 72)
	if weight < 0.27 || weight > 0.29 {
		t.Fatalf("expected typical BSkA tournament weight around 0.28, got %.4f", weight)
	}
}

func TestCalculateNewRatingMatchesBSkAExampleRounded(t *testing.T) {
	weight := CalculateTournamentWeight(15, 72)
	newRating := CalculateNewRating(InitialRating, 18, 32, weight)
	if newRating != 30 {
		t.Fatalf("expected rounded rating 30, got %d", newRating)
	}
}
