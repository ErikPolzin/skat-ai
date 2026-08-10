import CardClub from "../assets/Card_club.svg";
import CardSpade from "../assets/Card_spade.svg";
import CardHeart from "../assets/Card_heart.svg";
import CardDiamond from "../assets/Card_diamond.svg";

import { useTheme } from "@mui/material/styles";
import { motion, useMotionValueEvent, useTime } from "motion/react";
import { useState } from "react";

const ThemedLoader = ({ size }: { size?: number }) => {
  const theme = useTheme();
  const [shape, setShape] = useState<string>(CardClub);
  const time = useTime();
  const isDarkSuit = shape === CardClub || shape === CardSpade;
  const suitColor = isDarkSuit ? theme.themeBlackColor : theme.themeRedColor;

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
      <div
        role="img"
        aria-label="Card suit"
        style={{
          width: size,
          height: size,
          backgroundColor: suitColor,
          maskImage: `url(${shape})`,
          maskPosition: "center",
          maskRepeat: "no-repeat",
          maskSize: "contain",
          WebkitMaskImage: `url(${shape})`,
          WebkitMaskPosition: "center",
          WebkitMaskRepeat: "no-repeat",
          WebkitMaskSize: "contain",
        }}
      />
    </motion.div>
  );
};

export default ThemedLoader;
