import React from "react";
import { motion, HTMLMotionProps, TargetAndTransition } from "motion/react";

export default function Card({
  rank,
  suit,
  index,
  selected,
  animate,
  className,
  ...props
}: {
  rank?: string;
  suit?: string;
  index: number;
  selected?: boolean;
  animate: TargetAndTransition;
  className?: string;
} & Omit<HTMLMotionProps<"div">, "className">) {
  const faceDown = !(rank && suit);
  const [hasDealt, setHasDealt] = React.useState(false);

  React.useEffect(() => {
    // Mark as dealt after initial animation
    const timer = setTimeout(
      () => setHasDealt(true),
      (index * 0.1 + 0.5) * 1000,
    );
    return () => clearTimeout(timer);
  }, [index]);

  return (
    <motion.div
      {...props}
      animate={{ rotateY: faceDown ? 0 : 180, ...(animate || {}) }}
      initial={{ rotateY: faceDown ? 0 : 180 }} // Start face-down
      transition={{
        rotateY: {
          duration: 0.6,
          delay: index * 0.05, // Stagger the flip
        },
        x: {
          type: "spring",
          damping: 20,
          stiffness: 100,
          delay: hasDealt ? 0 : index * 0.1, // Only stagger initial deal
        },
        y: {
          type: "spring",
          damping: 20,
          stiffness: 100,
          delay: hasDealt ? 0 : index * 0.1, // Only stagger initial deal
        },
        ...props.transition,
      }}
      className={className || `motion-card ${selected ? "selected" : ""}`}
      style={{
        zIndex: 100 + index,
        transformStyle: "preserve-3d",
        ...props.style,
      }}
    >
      <div
        style={{
          position: "relative",
          width: "100%",
          height: "100%",
          transformStyle: "preserve-3d",
        }}
      >
        {/* Card back (visible when rotateY is 0) */}
        <img
          src="/res/back.svg"
          alt="card back"
          className="card-back"
          style={{
            position: "absolute",
            backfaceVisibility: "hidden",
          }}
        />
        {/* Card face (visible when rotateY is 180) */}
        <img
          src={`/res/${rank}${suit}.svg`}
          alt={`${rank} of ${suit}`}
          className="card-face"
          style={{
            position: "absolute",
            backfaceVisibility: "hidden",
            transform: "rotateY(180deg)",
          }}
        />
      </div>
    </motion.div>
  );
}
