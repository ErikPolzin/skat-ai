import CardClub from "../assets/Card_club.svg?raw";
import CardSpade from "../assets/Card_spade.svg?raw";
import CardHeart from "../assets/Card_heart.svg?raw";
import CardDiamond from "../assets/Card_diamond.svg?raw";

import { darken, useTheme } from "@mui/material/styles";
import { motion, useMotionValueEvent, useTime } from "motion/react";
import { useMemo, useState } from "react";

const suits = [CardClub, CardSpade, CardHeart, CardDiamond];

function colorSuit(svg: string, color: string) {
  const coloredSvg = svg.replace(
    /fill:\s*(?:#[0-9a-f]{3,8}|[a-z]+)/gi,
    `fill:${color}`,
  );

  return `data:image/svg+xml,${encodeURIComponent(coloredSvg)}`;
}

const ThemedLoader = ({ size }: { size?: number }) => {
  const theme = useTheme();
  const loaderSize = size ?? 50;
  const [suitIndex, setSuitIndex] = useState(0);
  const time = useTime();
  const isDarkSuit = suitIndex < 2;
  const suitColor = darken(
    isDarkSuit ? theme.themeBlackColor : theme.themeRedColor,
    0.15,
  );
  const suitUrl = useMemo(
    () => colorSuit(suits[suitIndex], suitColor),
    [suitColor, suitIndex],
  );

  useMotionValueEvent(time, "change", (latest) => {
    setSuitIndex(Math.floor((latest + 500) / 1000) % suits.length);
  });

  return (
    <motion.div
      animate={{ rotateY: [0, 180] }}
      style={{
        width: loaderSize,
        height: loaderSize,
        display: "inline-block",
        transformOrigin: "center center",
      }}
      transition={{
        duration: 1,
        ease: "easeInOut",
        repeat: Infinity,
        repeatType: "reverse",
      }}
    >
      <img
        src={suitUrl}
        alt="Card suit"
        style={{
          width: loaderSize,
          height: loaderSize,
          display: "block",
          objectFit: "contain",
        }}
      />
    </motion.div>
  );
};

export default ThemedLoader;
