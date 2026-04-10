import React, { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { createGame, joinGame, getGames, type GameState } from "../api/games";
import { useProfileStore } from "../stores/profileStore";
import { GameHistory } from "../components/GameHistory";
import "./LobbyScreen.css";

interface LobbyScreenProps {
  username: string;
}

export default function LobbyScreen({ username }: LobbyScreenProps) {
  const navigate = useNavigate();
  const profilePlayerId = useProfileStore((state) => state.playerId);
  const setUsername = useProfileStore((state) => state.setUsername);
  const setPlayerId = useProfileStore((state) => state.setPlayerId);
  const [gameId, setGameId] = useState("");
  const [games, setGames] = useState<GameState[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchGames();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchGames = async () => {
    try {
      const data = await getGames();
      setGames(data);
    } catch (error) {
      console.error("Failed to fetch games:", error);
    }
  };

  const handleJoinOrCreate = async () => {
    let currentGameId = gameId.trim();

    try {
      setError(null);

      // Create game if no ID provided
      if (!currentGameId) {
        const data = await createGame();
        currentGameId = data.game_id;
      }

      // Get player credentials - send existing ID if we have one
      const data = await joinGame(
        currentGameId,
        username,
        profilePlayerId || undefined,
      );

      // Store player ID and name from server
      setPlayerId(data.player_id);
      if (data.player_name !== username) {
        setUsername(data.player_name);
      }

      // Navigate to the game
      navigate(`/game/${currentGameId}`);
    } catch (error) {
      console.error("Error in handleJoinOrCreate:", error);
      setError((error as Error).message);
    }
  };

  const handleQuickJoin = (id: string) => {
    setGameId(id);
    setTimeout(() => handleJoinOrCreate(), 0);
  };

  return (
    <div className="screen">
      <div className="container lobby-container">
        <div className="lobby-header">
          <h1>Welcome, {username}!</h1>
        </div>

        {error && (
          <div
            className="error-message"
            style={{ color: "red", padding: "10px", marginBottom: "10px" }}
          >
            Error: {error}
          </div>
        )}

        <div className="section">
          <h2>Join or Create Game</h2>
          <div className="input-group">
            <input
              type="text"
              placeholder="Enter game code or leave empty to create"
              value={gameId}
              onChange={(e) => setGameId(e.target.value.toUpperCase())}
              maxLength={8}
              style={{ textTransform: "uppercase", letterSpacing: "2px" }}
            />
            <button className="btn btn-primary" onClick={handleJoinOrCreate}>
              {gameId ? "Join Game" : "Create New Game"}
            </button>
          </div>
        </div>

        <div className="section">
          <h3>Available Games</h3>
          <div className="games-list">
            {games.length === 0 ? (
              <p className="no-games">No active games</p>
            ) : (
              games.map((game) => (
                <div key={game.id} className="game-item">
                  <div>
                    <strong>{game.id}</strong> - {game.players.length}/ 3
                    players
                    {game.phase !== "waiting" && " (In Progress)"}
                  </div>
                  <button
                    className="btn"
                    onClick={() => handleQuickJoin(game.id)}
                    disabled={
                      game.phase !== "waiting" || game.players.length >= 3
                    }
                  >
                    Join
                  </button>
                </div>
              ))
            )}
          </div>
          <button className="btn" onClick={fetchGames}>
            Refresh Games
          </button>
        </div>

        <GameHistory playerId={profilePlayerId} />
      </div>
    </div>
  );
}
