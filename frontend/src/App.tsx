import React, { useState, useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import {
  useProfileStore,
  selectUsername,
  selectPlayerId,
  selectSetUsername,
  selectSetPlayerId,
} from "./stores/profileStore";
import { createOrRetrieveProfile } from "./api/games";
import { WebSocketProvider } from "./context/WebSocketContext";
import UsernameScreen from "./screens/UsernameScreen";
import LobbyScreen from "./screens/LobbyScreen";
import GameScreen from "./screens/GameScreen";
import "./App.css";

function App() {
  const username = useProfileStore(selectUsername);
  const playerId = useProfileStore(selectPlayerId);
  const setUsername = useProfileStore(selectSetUsername);
  const setPlayerId = useProfileStore(selectSetPlayerId);
  const [isInitializing, setIsInitializing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Initialize profile when username is set but no player ID exists
  useEffect(() => {
    if (username && !playerId && !isInitializing) {
      setIsInitializing(true);
      createOrRetrieveProfile(username)
        .then((profile) => {
          setPlayerId(profile.player_id);
          if (profile.player_name !== username) {
            setUsername(profile.player_name);
          }
          setError(null);
        })
        .catch((err) => {
          console.error("Failed to create profile:", err);
          setError("Failed to connect to server. Please try again.");
        })
        .finally(() => {
          setIsInitializing(false);
        });
    }
  }, [username, playerId, isInitializing, setPlayerId, setUsername]);

  // Handle username submission
  const handleUsernameSubmit = async (newUsername: string) => {
    setError(null);
    setIsInitializing(true);

    try {
      // Try to retrieve existing profile with current playerId (if any)
      const profile = await createOrRetrieveProfile(newUsername, playerId || undefined);
      setPlayerId(profile.player_id);
      setUsername(profile.player_name);
    } catch (err) {
      console.error("Failed to create profile:", err);
      setError("Failed to connect to server. Please try again.");
      // Still set username locally so user can retry
      setUsername(newUsername);
    } finally {
      setIsInitializing(false);
    }
  };

  // Show username screen if no username or still initializing without a player ID
  if (!username || (username && !playerId && !error)) {
    return (
      <div className="App">
        <BrowserRouter>
          {isInitializing ? (
            <div className="screen">
              <div className="container">
                <h2>Connecting to server...</h2>
              </div>
            </div>
          ) : (
            <UsernameScreen onSubmit={handleUsernameSubmit} />
          )}
        </BrowserRouter>
      </div>
    );
  }

  // Show error screen if profile creation failed
  if (error && !playerId) {
    return (
      <div className="App">
        <BrowserRouter>
          <div className="screen">
            <div className="container">
              <h2>Connection Error</h2>
              <p style={{ color: "red", marginBottom: "20px" }}>{error}</p>
              <button
                className="btn btn-primary"
                onClick={() => {
                  setError(null);
                  if (username) {
                    handleUsernameSubmit(username);
                  }
                }}
              >
                Retry
              </button>
            </div>
          </div>
        </BrowserRouter>
      </div>
    );
  }

  return (
    <div className="App">
      <WebSocketProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<LobbyScreen username={username} />} />
            <Route path="/game/:gameId" element={<GameScreen />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </WebSocketProvider>
    </div>
  );
}

export default App;
