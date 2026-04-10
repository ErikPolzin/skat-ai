import React from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import {
  useProfileStore,
  selectUsername,
  selectSetUsername,
} from "./stores/profileStore";
import { WebSocketProvider } from "./context/WebSocketContext";
import UsernameScreen from "./screens/UsernameScreen";
import LobbyScreen from "./screens/LobbyScreen";
import GameScreen from "./screens/GameScreen";
import "./App.css";

function App() {
  const username = useProfileStore(selectUsername);
  const setUsername = useProfileStore(selectSetUsername);

  // Show username screen if no username
  if (!username) {
    return (
      <div className="App">
        <BrowserRouter>
          <UsernameScreen onSubmit={setUsername} />
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
