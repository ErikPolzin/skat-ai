import { useEffect, useState } from "react";

export interface DeadlineState {
  /** Time remaining in seconds, null if no deadline */
  secondsRemaining: number | null;
  /** True if deadline is approaching (< 60 seconds) */
  isApproaching: boolean;
  /** True if deadline has passed */
  isExpired: boolean;
  /** Formatted time string (e.g., "2:30") */
  formattedTime: string;
}

/**
 * Hook to monitor a deadline and provide countdown state
 * @param deadlineISO RFC3339 timestamp string of the deadline
 * @returns DeadlineState object with countdown information
 */
export function useDeadlineTimer(deadlineISO: string): DeadlineState {
  const [secondsRemaining, setSecondsRemaining] = useState<number | null>(null);

  useEffect(() => {
    // No deadline set
    if (!deadlineISO || deadlineISO === "") {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSecondsRemaining(null);
      return;
    }

    // Parse the deadline
    const updateRemainingTime = () => {
      try {
        const deadline = new Date(deadlineISO);
        const now = new Date();
        const diffMs = deadline.getTime() - now.getTime();
        const diffSeconds = Math.floor(diffMs / 1000);

        setSecondsRemaining(diffSeconds);
      } catch (error) {
        console.error("Failed to parse deadline:", deadlineISO, error);
        setSecondsRemaining(null);
      }
    };

    // Update immediately
    updateRemainingTime();

    // Update every second
    const interval = setInterval(updateRemainingTime, 1000);

    return () => clearInterval(interval);
  }, [deadlineISO]);

  // Derive state from secondsRemaining
  const isExpired = secondsRemaining !== null && secondsRemaining <= 0;
  const isApproaching =
    secondsRemaining !== null && secondsRemaining > 0 && secondsRemaining <= 60;

  // Format time as MM:SS
  const formattedTime =
    secondsRemaining !== null && secondsRemaining > 0
      ? `${Math.floor(secondsRemaining / 60)}:${String(secondsRemaining % 60).padStart(2, "0")}`
      : "";

  return {
    secondsRemaining,
    isApproaching,
    isExpired,
    formattedTime,
  };
}
