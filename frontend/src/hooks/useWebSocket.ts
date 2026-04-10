import { useRef, useCallback } from "react";

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const messageHandlersRef = useRef<Map<string, (message: any) => void>>(
    new Map(),
  );

  // Add a message handler - returns a cleanup function
  const addMessageHandler = useCallback(
    (key: string, handler: (message: any) => void) => {
      messageHandlersRef.current.set(key, handler);
      // Return cleanup function
      return () => {
        messageHandlersRef.current.delete(key);
      };
    },
    [],
  );

  const sendMessage = useCallback((type: string, data: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type, data }));
    } else {
      console.error(
        "Cannot send message, socket is",
        wsRef.current?.readyState,
        ", not open",
      );
    }
  }, []);

  const connect = useCallback((profileId: string) => {
    if (
      wsRef.current &&
      (wsRef.current.readyState === WebSocket.CONNECTING ||
        wsRef.current.readyState === WebSocket.OPEN)
    ) {
      console.log("WebSocket already connected or connecting");
      return;
    }

    const wsUrl = process.env.REACT_APP_WS_URL
      ? `${process.env.REACT_APP_WS_URL}/ws`
      : `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/ws`;

    // Add profile_id as query parameter
    const urlWithProfile = `${wsUrl}?profile_id=${encodeURIComponent(profileId)}`;

    console.log("Connecting WebSocket with profile:", profileId);
    const ws = new WebSocket(urlWithProfile);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WebSocket connected for profile:", profileId);
      wsRef.current = ws;
    };

    ws.onmessage = (event: MessageEvent) => {
      try {
        const message = JSON.parse(event.data);
        console.log("WebSocket message received:", message);

        // Call all registered handlers
        messageHandlersRef.current.forEach((handler) => {
          try {
            handler(message);
          } catch (error) {
            console.error("Error in message handler:", error);
          }
        });
      } catch (error) {
        console.error("Error parsing WebSocket message:", error);
      }
    };

    ws.onerror = (error: Event) => {
      console.error("WebSocket error:", error);
    };

    ws.onclose = (event: CloseEvent) => {
      console.log("WebSocket closed:", event.code, event.reason);
      wsRef.current = null;
    };
  }, []);

  const disconnect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  return {
    connect,
    disconnect,
    sendMessage,
    isConnected: wsRef.current?.readyState === WebSocket.OPEN,
    addMessageHandler,
  };
}
export type SkatWebSocket = ReturnType<typeof useWebSocket>;
