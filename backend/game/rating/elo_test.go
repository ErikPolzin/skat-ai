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
		"leader": {ProfileID: "leader", Rating: 1500, GamesPlayed: 10, PeakRating: 1500},
		"second": {ProfileID: "second", Rating: 1500, GamesPlayed: 10, PeakRating: 1500},
		"third":  {ProfileID: "third", Rating: 1500, GamesPlayed: 10, PeakRating: 1500},
	}

	if err := UpdateRatings(results, ratings, 0); err != nil {
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
}
