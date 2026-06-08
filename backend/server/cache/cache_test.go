package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"skat/game"
	"skat/server/db"
)

func TestSyncWorkerSkipsOlderRevision(t *testing.T) {
	database := &recordingDatabase{saved: make(chan game.GameState, 1)}
	store := NewMemoryBackend(4)
	queue := newGameQueue(2)

	gs := game.NewGame()
	gs.ID = "game-1"
	gs.SessionID = "session-1"
	gs.Phase = game.PhaseBidding
	gs.Players = [3]*game.PlayerState{
		{ID: "dealer", Name: "Dealer"},
		{ID: "listener", Name: "Listener"},
		{ID: "speaker", Name: "Speaker"},
	}

	stale := *gs.Clone()
	stale.CacheRevision = 1

	declarer := game.Listener
	gs.Declarer = &declarer
	gs.Phase = game.PhaseSkatExchange
	gs.CurrentPlayer = declarer
	latest := *gs.Clone()
	latest.CacheRevision = 2

	if err := store.Set(context.Background(), "game:"+latest.ID, mustEncodeGameState(t, latest), time.Minute); err != nil {
		t.Fatalf("write latest cache state: %v", err)
	}
	queue.items <- stale
	queue.items <- latest

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartSyncWorker(ctx, database, queue, store)

	select {
	case saved := <-database.saved:
		if saved.CacheRevision != latest.CacheRevision {
			t.Fatalf("expected revision %d, got %d", latest.CacheRevision, saved.CacheRevision)
		}
		if saved.Declarer == nil || *saved.Declarer != declarer {
			t.Fatalf("expected declarer %d to be saved, got %v", declarer, saved.Declarer)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sync worker to save latest snapshot")
	}
}

func TestSaveGameRejectsStaleRevision(t *testing.T) {
	database := &recordingDatabase{saved: make(chan game.GameState, 10)}
	store := NewMemoryBackend(4)
	cache := NewDistributedCache(database, store, nil, time.Minute)

	gs := game.NewGame()
	gs.ID = "game-1"
	gs.SessionID = "session-1"
	gs.Phase = game.PhaseBidding
	gs.Players = [3]*game.PlayerState{
		{ID: "dealer", Name: "Dealer"},
		{ID: "listener", Name: "Listener"},
		{ID: "speaker", Name: "Speaker"},
	}
	if err := cache.SaveGame(gs); err != nil {
		t.Fatalf("save initial game: %v", err)
	}
	if gs.CacheRevision != 1 {
		t.Fatalf("expected caller revision 1, got %d", gs.CacheRevision)
	}

	loaded, err := cache.GetGameByID(gs.ID)
	if err != nil {
		t.Fatalf("load initial cached game: %v", err)
	}
	if loaded.CacheRevision != 1 {
		t.Fatalf("expected initial revision 1, got %d", loaded.CacheRevision)
	}

	declarer := game.Listener
	latest := *loaded.Clone()
	latest.Declarer = &declarer
	latest.Phase = game.PhaseSkatExchange
	latest.CurrentPlayer = declarer
	if err := cache.SaveGame(&latest); err != nil {
		t.Fatalf("save latest game: %v", err)
	}

	stale := *loaded.Clone()
	stale.Phase = game.PhasePlaying
	stale.CurrentPlayer = game.Listener
	err = cache.SaveGame(&stale)
	if !errors.Is(err, ErrStaleGameState) {
		t.Fatalf("expected stale game state error, got %v", err)
	}

	cached, err := cache.GetGameByID(gs.ID)
	if err != nil {
		t.Fatalf("load final cached game: %v", err)
	}
	if cached.Declarer == nil || *cached.Declarer != declarer {
		t.Fatalf("expected cached declarer %d, got %v", declarer, cached.Declarer)
	}
	if cached.Phase != game.PhaseSkatExchange {
		t.Fatalf("expected cached phase %s, got %s", game.PhaseSkatExchange, cached.Phase)
	}
}

func TestSaveGameEnqueuesSnapshotWithAllocatedRevision(t *testing.T) {
	database := &recordingDatabase{saved: make(chan game.GameState, 10)}
	store := NewMemoryBackend(4)
	queue := newGameQueue(1)
	cache := NewDistributedCache(database, store, queue, time.Minute)

	gs := game.NewGame()
	gs.ID = "game-1"
	gs.SessionID = "session-1"
	gs.Phase = game.PhaseBidding
	gs.Players = [3]*game.PlayerState{
		{ID: "dealer", Name: "Dealer"},
		{ID: "listener", Name: "Listener"},
		{ID: "speaker", Name: "Speaker"},
	}

	if err := cache.SaveGame(gs); err != nil {
		t.Fatalf("save game: %v", err)
	}
	if gs.CacheRevision != 1 {
		t.Fatalf("expected caller revision 1, got %d", gs.CacheRevision)
	}

	select {
	case queued := <-queue.items:
		if queued.CacheRevision != gs.CacheRevision {
			t.Fatalf("expected queued revision %d, got %d", gs.CacheRevision, queued.CacheRevision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued game save")
	}
}

func mustEncodeGameState(t *testing.T, gs game.GameState) []byte {
	t.Helper()
	data, err := encodeGameState(gs)
	if err != nil {
		t.Fatalf("encode game state: %v", err)
	}
	return data
}

type gameQueue struct {
	items chan game.GameState
}

func newGameQueue(size int) *gameQueue {
	return &gameQueue{items: make(chan game.GameState, size)}
}

func (q *gameQueue) EnqueueGameSave(ctx context.Context, gs game.GameState) error {
	select {
	case q.items <- gs:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *gameQueue) DequeueGameSave(ctx context.Context) (*game.GameState, error) {
	select {
	case gs := <-q.items:
		return &gs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, ErrQueueEmpty
	}
}

func (q *gameQueue) Close() error {
	return nil
}

type recordingDatabase struct {
	saved chan game.GameState
}

func (d *recordingDatabase) SaveGame(gs game.GameState) error {
	d.saved <- gs
	return nil
}

func (d *recordingDatabase) Close() error      { return nil }
func (d *recordingDatabase) InitSchema() error { return nil }
func (d *recordingDatabase) GetProfile(profileID string) (*db.ProfileEntry, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetProfileByName(name string) (*db.ProfileEntry, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) SaveProfile(profile db.ProfileEntry) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) SaveGameSession(session game.GameSessionState) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) GetGameSession(sessionID string) (*game.GameSessionState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetGameByID(gameID string) (*game.GameState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) DeleteGame(gameID string) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) RemovePlayer(gameID, playerID string) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) ListOpenSessions() ([]game.GameSessionState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) ListPlayers(gameID string) ([3]*game.PlayerState, error) {
	return [3]*game.PlayerState{}, errors.New("not implemented")
}
func (d *recordingDatabase) SavePlayerResults(result []game.PlayerResultState) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) SavePlayerSessionResults(result []game.PlayerSessionResultState) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) GetPlayerSessionResults(playerID string, limit int) ([]game.PlayerSessionResultState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetSessionPlayerResults(sessionID string) ([]game.PlayerSessionResultState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) CountGamesInSession(sessionID string) (int, error) {
	return 0, errors.New("not implemented")
}
func (d *recordingDatabase) GetPlayerResultsForSession(sessionID string) ([]game.PlayerResultState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetFormattedSessionResults(sessionID string) ([]game.SessionGameResult, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) ListAgentProfiles() ([]db.ProfileEntry, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) CleanupStaleGames(inactiveMinutes int, onlinePlayerIDs []string) (int, error) {
	return 0, errors.New("not implemented")
}
func (d *recordingDatabase) GetActiveGamesByPlayer(playerID string) ([]game.GameState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetSpectatableGames(excludePlayerID string) ([]game.GameState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetAllExpiredGames() ([]game.GameState, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetPlayerRating(profileID string) (*db.PlayerRating, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) SavePlayerRating(rating db.PlayerRating) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) GetLeaderboard(limit int) ([]db.PlayerRating, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) GetAgentConfig(profileID string) (*db.AgentConfig, error) {
	return nil, errors.New("not implemented")
}
func (d *recordingDatabase) SaveAgentConfig(config db.AgentConfig) error {
	return errors.New("not implemented")
}
func (d *recordingDatabase) DeleteAgentConfig(profileID string) error {
	return errors.New("not implemented")
}
