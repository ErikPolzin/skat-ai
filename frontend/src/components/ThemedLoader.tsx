import CardClub from "../assets/Card_club.svg";
import CardSpade from "../assets/Card_spade.svg";
import CardHeart from "../assets/Card_heart.svg";
import CardDiamond from "../assets/Card_diamond.svg";

import {
  motion,
  useMotionValueEvent,
  useTime,
  useTransform,
} from "motion/react";
import { useState } from "react";

const ThemedLoader = ({ size }: { size?: number }) => {
  const [shape, setShape] = useState<string>(CardClub);
  const time = useTime();
  const rotateY = useTransform(time, [0, 1000], [0, 180], { clamp: false });

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
    <motion.div style={{ rotateY }}>
      <img src={shape} alt="CardClub" style={{ width: size, height: size }} />
    </motion.div>
  );
};

export default ThemedLoader;
