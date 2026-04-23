import React from "react";
import { motion, HTMLMotionProps, TargetAndTransition } from "motion/react";

export default function Card({
  rank,
  suit,
  index,
  selected,
  animate,
  className,
  skipInitialAnimation = false,
  initial: initialProp,
  transition: transitionProp,
  style: styleProp,
  ...props
}: {
  rank?: string;
  suit?: string;
  index: number;
  selected?: boolean;
  animate: TargetAndTransition;
  className?: string;
  skipInitialAnimation?: boolean;
} & Omit<HTMLMotionProps<"div">, "className">) {
  const faceDown = !(rank && suit);
  const [hasDealt, setHasDealt] = React.useState(skipInitialAnimation);

  React.useEffect(() => {
    if (skipInitialAnimation) return;

    // Mark as dealt after initial animation
    const timer = setTimeout(
      () => setHasDealt(true),
      (index * 0.1 + 0.5) * 1000,
    );
    return () => clearTimeout(timer);
  }, [index, skipInitialAnimation]);

  // Calculate flip delay: wait for deal animation to complete, then stagger flips
  const dealDuration = skipInitialAnimation ? 0 : index * 0.1 + 0.5;
  const flipDelay = skipInitialAnimation ? 0 : dealDuration + index * 0.05;

  return (
    <motion.div
      {...props}
      animate={{ rotateY: faceDown ? 0 : 180, ...(animate || {}) }}
      initial={{
        rotateY: 0, // Always start face down for initial animation
        ...(typeof initialProp === "object" && initialProp !== null
          ? initialProp
          : {}),
      }} // Merge with passed initial
      transition={{
        rotateY: {
          duration: 0.6,
          delay: flipDelay, // Wait for deal to complete, then stagger flips
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
        ...(typeof transitionProp === "object" && transitionProp !== null
          ? transitionProp
          : {}),
      }}
      className={`${className || "motion-card"} ${selected ? "selected" : ""}`}
      style={{
        zIndex: 100 + index,
        transformStyle: "preserve-3d",
        ...(styleProp || {}),
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
          src="/res/cards/back.svg"
          alt="card back"
          className="card-back"
          style={{
            position: "absolute",
            backfaceVisibility: "hidden",
          }}
        />
        {/* Card face (visible when rotateY is 180) */}
        <img
          src={`/res/cards/${rank}${suit}.svg`}
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
