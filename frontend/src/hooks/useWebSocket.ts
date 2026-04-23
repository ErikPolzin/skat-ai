import { useRef, useCallback, useSyncExternalStore, useMemo } from "react";

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const messageHandlersRef = useRef<Map<string, (message: any) => void>>(
    new Map(),
  );
  const pendingActionsRef = useRef<Map<string, NodeJS.Timeout>>(new Map());
  const actionIdCounter = useRef(0);
  const subscribersRef = useRef<Set<() => void>>(new Set());

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

  const sendMessage = useCallback(
    (type: string, data: any, onAck?: () => void, onTimeout?: () => void) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        // Generate unique action ID
        const actionId = `${type}_${Date.now()}_${++actionIdCounter.current}`;

        // Set up timeout for this action (10 seconds)
        const timeoutId = setTimeout(() => {
          pendingActionsRef.current.delete(actionId);
          console.warn(`Action ${actionId} timed out after 10 seconds`);
          if (onTimeout) {
            onTimeout();
          }
        }, 10000);

        // Store the pending action with its callbacks
        pendingActionsRef.current.set(actionId, timeoutId);

        // Add acknowledgment handler
        if (onAck) {
          const ackKey = `ack_${actionId}`;
          const cleanup = addMessageHandler(ackKey, () => {
            // Clear timeout
            const timeout = pendingActionsRef.current.get(actionId);
            if (timeout) {
              clearTimeout(timeout);
              pendingActionsRef.current.delete(actionId);
            }
            // Call acknowledgment callback
            onAck();
            // Clean up this handler
            cleanup();
          });
        }

        wsRef.current.send(JSON.stringify({ type, data, action_id: actionId }));
        return actionId;
      } else {
        console.error(
          "Cannot send message, socket is",
          wsRef.current?.readyState,
          ", not open",
        );
        if (onTimeout) {
          onTimeout();
        }
        return null;
      }
    },
    [addMessageHandler],
  );

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
      notifySubscribers();
    };

    ws.onmessage = (event: MessageEvent) => {
      try {
        const message = JSON.parse(event.data);
        console.log("WebSocket message received:", message);

        // Check if this message acknowledges a pending action
        // state_update, error, or any message with action_id serves as acknowledgment
        if (message.action_id) {
          const ackKey = `ack_${message.action_id}`;
          const ackHandler = messageHandlersRef.current.get(ackKey);
          if (ackHandler) {
            ackHandler(message);
          }
        }

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
      notifySubscribers();
    };

    ws.onclose = (event: CloseEvent) => {
      console.log("WebSocket closed:", event.code, event.reason);
      wsRef.current = null;
      notifySubscribers();
    };
  }, []);

  const disconnect = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const notifySubscribers = () => {
    subscribersRef.current.forEach((callback) => callback());
  };

  const subscribe = useCallback((callback: () => void) => {
    subscribersRef.current.add(callback);
    return () => {
      subscribersRef.current.delete(callback);
    };
  }, []);

  const getSnapshot = useCallback(() => {
    return wsRef.current?.readyState === WebSocket.OPEN;
  }, []);

  const isConnected = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);

  return useMemo(
    () => ({
      connect,
      disconnect,
      sendMessage,
      isConnected,
      addMessageHandler,
    }),
    [connect, disconnect, sendMessage, isConnected, addMessageHandler],
  );
}
export type SkatWebSocket = ReturnType<typeof useWebSocket>;
