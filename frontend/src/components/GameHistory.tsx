import React, { useEffect, useState } from "react";
import { getPlayerGameHistory, type GameHistoryEntry } from "../api/games";
import "./GameHistory.css";

interface GameHistoryProps {
  playerId: string | null;
}

export function GameHistory({ playerId }: GameHistoryProps) {
  const [history, setHistory] = useState<GameHistoryEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [showAIGames, setShowAIGames] = useState(true);

  useEffect(() => {
    if (!playerId) {
      setHistory([]);
      return;
    }

    const fetchHistory = async () => {
      setLoading(true);
      try {
        const data = await getPlayerGameHistory(playerId, 20);
        setHistory(data || []);  // Ensure we always set an array, never null
      } catch (error) {
        console.error("Failed to load game history:", error);
        setHistory([]);
      } finally {
        setLoading(false);
      }
    };

    fetchHistory();
  }, [playerId]);

  // Filter history based on AI games toggle
  const filteredHistory = showAIGames
    ? history
    : history?.filter(entry => !entry.vs_ai) || [];

  if (!playerId) {
    return null;
  }

  if (loading) {
    return (
      <div className="game-history">
        <h3>Recent Games</h3>
        <div className="loading">Loading history...</div>
      </div>
    );
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? "s" : ""} ago`;
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? "s" : ""} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? "s" : ""} ago`;

    return date.toLocaleDateString();
  };

  const formatOpponents = (names: string[]) => {
    if (!names || names.length === 0) return "Unknown";
    if (names.length === 1) return names[0];
    return names.join(" & ");
  };

  if (!history || history.length === 0) {
    return (
      <div className="game-history">
        <h3>Recent Games</h3>
        <div className="no-history">No games played yet</div>
      </div>
    );
  }

  return (
    <div className="game-history">
      <div className="history-header">
        <h3>Recent Games</h3>
        <div className="history-filter">
          <label className="filter-toggle">
            <input
              type="checkbox"
              checked={showAIGames}
              onChange={(e) => setShowAIGames(e.target.checked)}
            />
            <span>Show AI Games</span>
          </label>
        </div>
      </div>

      {filteredHistory.length === 0 ? (
        <div className="no-history">No {!showAIGames ? "human" : ""} games found</div>
      ) : (
        <div className="history-list">
          {filteredHistory.map((entry, index) => (
            <div
              key={`${entry.game_id}-${index}`}
              className={`history-entry ${entry.is_winner ? "winner" : "loser"} ${entry.vs_ai ? "vs-ai" : "vs-human"}`}
            >
              <div className="entry-header">
                <span className="game-code">{entry.game_code || entry.game_id.slice(0, 8)}</span>
                {entry.vs_ai && <span className="ai-badge">vs AI</span>}
                <span className="game-date">{formatDate(entry.finished_at)}</span>
              </div>

              <div className="opponents-line">
                vs {formatOpponents(entry.opponent_names || [])}
              </div>

              <div className="entry-details">
                <div className="game-mode">{entry.game_mode}</div>
                <div className="game-role">
                  {entry.is_declarer ? "Declarer" : "Defender"}
                </div>
                <div className="game-score">Score: {entry.final_score}</div>
              </div>
              <div className={`game-result ${entry.is_winner ? "win" : "loss"}`}>
                {entry.is_winner ? "Victory" : "Defeat"}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}