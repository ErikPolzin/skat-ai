import { useDeadlineTimer } from "../hooks/useDeadlineTimer";

interface CircularTimerProps {
  deadline: string;
  isCurrentPlayer: boolean;
  isAI: boolean;
  size?: number;
}

/**
 * Circular timer overlay that displays as a red pie arc around an avatar
 * Only shows for human players when it's their turn
 */
export function CircularTimer({
  deadline,
  isCurrentPlayer,
  isAI,
  size = 70,
}: CircularTimerProps) {
  const { secondsRemaining } = useDeadlineTimer(deadline);

  // Don't show for AI players or if not their turn
  if (isAI || !isCurrentPlayer || !deadline || secondsRemaining === null) {
    return null;
  }

  // Don't show if expired
  if (secondsRemaining <= 0) {
    return null;
  }

  // Calculate progress (0-1, where 1 is full time, 0 is no time left)
  const totalSeconds = 2 * 60; // 2 minutes
  const progress = Math.max(0, Math.min(1, secondsRemaining / totalSeconds));

  // SVG pie calculation
  const center = size / 2;
  const radius = size / 2;

  // Calculate the angle for the pie slice
  const angle = progress * 360;

  // Convert angle to radians and calculate end point
  const endAngle = (angle - 90) * (Math.PI / 180);
  const x = center + radius * Math.cos(endAngle);
  const y = center + radius * Math.sin(endAngle);

  // Large arc flag (1 if > 180 degrees)
  const largeArcFlag = angle > 180 ? 1 : 0;

  // Color transitions from green to yellow to red as time runs out
  let color;
  if (progress > 0.66) {
    color = "#4caf50"; // Green
  } else if (progress > 0.33) {
    color = "#ff9800"; // Orange
  } else {
    color = "#f44336"; // Red
  }

  return (
    <div
      style={{
        position: "absolute",
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
        width: size,
        height: size,
        pointerEvents: "none",
        zIndex: 0,
      }}
    >
      <svg
        width={size}
        height={size}
        style={{
          overflow: "visible",
        }}
      >
        {/* Filled pie slice */}
        <path
          d={`M ${center} ${center} L ${center} 0 A ${radius} ${radius} 0 ${largeArcFlag} 1 ${x} ${y} Z`}
          fill={color}
          opacity="0.7"
          style={{
            transition: "fill 0.3s ease",
          }}
        />
      </svg>
    </div>
  );
}
