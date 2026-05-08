import {
  useRef,
  useCallback,
  useSyncExternalStore,
  useMemo,
  useState,
} from "react";

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const messageHandlersRef = useRef<Map<string, (message: any) => void>>(
    new Map(),
  );
  const subscribersRef = useRef<Set<() => void>>(new Set());
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectCountdownRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const profileIdRef = useRef<string | null>(null);
  const manualDisconnectRef = useRef(false);
  const [reconnectCountdown, setReconnectCountdown] = useState<number | null>(
    null,
  );

  // Reconnection configuration
  const MAX_RECONNECT_ATTEMPTS = 10;
  const INITIAL_RETRY_DELAY = 1000; // 1 second
  const MAX_RETRY_DELAY = 30000; // 30 seconds

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

  const scheduleReconnect = useCallback(() => {
    if (manualDisconnectRef.current) {
      console.log("Manual disconnect, not reconnecting");
      return;
    }

    if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
      console.error(
        `Max reconnection attempts (${MAX_RECONNECT_ATTEMPTS}) reached. Giving up.`,
      );
      return;
    }

    // Calculate exponential backoff delay
    const delay = Math.min(
      INITIAL_RETRY_DELAY * Math.pow(2, reconnectAttemptsRef.current),
      MAX_RETRY_DELAY,
    );

    const startTime = Date.now();
    setReconnectCountdown(delay / 1000);

    // Countdown timer
    reconnectCountdownRef.current = setInterval(() => {
      const elapsed = Date.now() - startTime;
      const remaining = Math.max(0, (delay - elapsed) / 1000);
      if (remaining > 0) {
        setReconnectCountdown(remaining);
      }
    }, 100);

    reconnectTimeoutRef.current = setTimeout(() => {
      if (reconnectCountdownRef.current) {
        clearInterval(reconnectCountdownRef.current);
        reconnectCountdownRef.current = null;
      }
      setReconnectCountdown(null);
      if (profileIdRef.current && !manualDisconnectRef.current) {
        reconnectAttemptsRef.current++;
        connect(profileIdRef.current);
      }
    }, delay);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const connect = useCallback(
    (profileId: string) => {
      if (
        wsRef.current &&
        (wsRef.current.readyState === WebSocket.CONNECTING ||
          wsRef.current.readyState === WebSocket.OPEN)
      ) {
        console.log("WebSocket already connected or connecting");
        return;
      }

      // Clear any pending reconnection attempts
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      // Store profile ID for reconnection attempts
      profileIdRef.current = profileId;
      manualDisconnectRef.current = false;

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
        // Reset reconnection attempts on successful connection
        reconnectAttemptsRef.current = 0;
        wsRef.current = ws;
        notifySubscribers();
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
        notifySubscribers();
      };

      ws.onclose = (event: CloseEvent) => {
        console.log("WebSocket closed:", event.code, event.reason);
        wsRef.current = null;
        notifySubscribers();
        if (manualDisconnectRef.current) {
          // Attempt to reconnect unless it was a manual disconnect
          scheduleReconnect();
        }
      };
    },
    [scheduleReconnect],
  );

  const disconnect = useCallback(() => {
    // Mark as manual disconnect to prevent reconnection
    manualDisconnectRef.current = true;

    // Clear any pending reconnection attempts
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // Clear countdown interval
    if (reconnectCountdownRef.current) {
      clearInterval(reconnectCountdownRef.current);
      reconnectCountdownRef.current = null;
    }

    // Reset reconnection attempts and countdown
    reconnectAttemptsRef.current = 0;
    setReconnectCountdown(null);

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
      reconnectCountdown,
    }),
    [
      connect,
      disconnect,
      sendMessage,
      isConnected,
      addMessageHandler,
      reconnectCountdown,
    ],
  );
}
export type SkatWebSocket = ReturnType<typeof useWebSocket>;
