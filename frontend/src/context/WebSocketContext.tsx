import React, { createContext, useContext, useEffect } from "react";
import { type SkatWebSocket, useWebSocket } from "../hooks/useWebSocket";
import { selectPlayerId, useProfileStore } from "../stores/profileStore";

const WebSocketContext = createContext<SkatWebSocket | null>(null);

export const WebSocketProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const playerId = useProfileStore(selectPlayerId);
  const value = useWebSocket();

  // Connect WebSocket when we have a profile ID
  useEffect(() => {
    console.log("PLAYER ID CHANGED", playerId);
    if (playerId) {
      value.connect(playerId);
      return () => {
        value.disconnect();
      };
    }
    // Only reconnect when playerId changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playerId]);

  return (
    <WebSocketContext.Provider value={value}>
      {children}
    </WebSocketContext.Provider>
  );
};

// eslint-disable-next-line react-refresh/only-export-components
export const useWebSocketContext = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error(
      "useWebSocketContext must be used within a WebSocketProvider",
    );
  }
  return context;
};
