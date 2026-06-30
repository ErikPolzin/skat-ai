import CardClub from "../assets/Card_club.svg";
import CardSpade from "../assets/Card_spade.svg";
import CardHeart from "../assets/Card_heart.svg";
import CardDiamond from "../assets/Card_diamond.svg";

import { motion, useMotionValueEvent, useTime } from "motion/react";
import { useState } from "react";

const ThemedLoader = ({ size }: { size?: number }) => {
  const [shape, setShape] = useState<string>(CardClub);
  const time = useTime();
  const isDarkSuit = shape === CardClub || shape === CardSpade;

  useMotionValueEvent(time, "change", (latest) => {
    switch (Math.floor((latest + 500) / 1000) % 4) {
      case 0:
        setShape(CardClub);
        break;
      case 1:
        setShape(CardSpade);
        break;
      case 2:
        setShape(CardHeart);
        break;
      case 3:
        setShape(CardDiamond);
        break;
      default:
        setShape(CardClub);
    }
  });

  return (
    <motion.div
      animate={{ rotateY: [0, 180] }}
      transition={{
        duration: 1,
        ease: "easeInOut",
        repeat: Infinity,
        repeatType: "reverse",
      }}
    >
      <img
        src={shape}
        alt="Card suit"
        style={{
          width: size,
          height: size,
          filter: `brightness(0) invert(${isDarkSuit ? "20%" : "35%"})`,
        }}
      />
    </motion.div>
  );
};

export default ThemedLoader;
