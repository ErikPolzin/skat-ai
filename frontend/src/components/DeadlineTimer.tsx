import { useDeadlineTimer } from "../hooks/useDeadlineTimer";

interface DeadlineTimerProps {
  deadline: string;
  isMyTurn: boolean;
}

export function DeadlineTimer({ deadline, isMyTurn }: DeadlineTimerProps) {
  const { secondsRemaining, isApproaching, isExpired, formattedTime } =
    useDeadlineTimer(deadline);

  // Don't show timer if no deadline or not my turn
  if (!deadline || !isMyTurn || secondsRemaining === null) {
    return null;
  }

  // Don't show if expired
  if (isExpired) {
    return null;
  }

  return (
    <div
      className={`deadline-timer ${isApproaching ? "deadline-approaching" : ""}`}
      style={{
        position: "fixed",
        top: "20px",
        right: "20px",
        padding: "12px 20px",
        borderRadius: "8px",
        backgroundColor: isApproaching ? "#ff6b6b" : "#4dabf7",
        color: "white",
        fontWeight: "bold",
        fontSize: "18px",
        boxShadow: "0 4px 8px rgba(0,0,0,0.2)",
        zIndex: 1000,
        transition: "all 0.3s ease",
        animation: isApproaching ? "pulse 1s infinite" : "none",
      }}
    >
      {isApproaching ? "⚠️ " : "⏱️ "}
      Time remaining: {formattedTime}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.9; transform: scale(1.05); }
        }
      `}</style>
    </div>
  );
}
