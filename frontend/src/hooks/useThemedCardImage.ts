import { useCallback, useEffect, useState } from "react";

const themedCardUrls = new Map<string, Promise<string>>();

function createThemedCardUrl(
  sourceUrl: string,
  cacheKey: string,
  transform: (svg: string) => string,
): Promise<string> {
  const cachedUrl = themedCardUrls.get(cacheKey);
  if (cachedUrl) return cachedUrl;

  const urlPromise = fetch(sourceUrl)
    .then((response) => {
      if (!response.ok) throw new Error(`Unable to load ${sourceUrl}`);
      return response.text();
    })
    .then((svg) => {
      const themedSvg = transform(svg);
      return URL.createObjectURL(
        new Blob([themedSvg], { type: "image/svg+xml" }),
      );
    });

  themedCardUrls.set(cacheKey, urlPromise);
  return urlPromise;
}

function useThemedCardImage(
  sourceUrl: string,
  cacheKey: string,
  transform: (svg: string) => string,
): string {
  const [url, setUrl] = useState(sourceUrl);

  useEffect(() => {
    let active = true;
    createThemedCardUrl(sourceUrl, cacheKey, transform)
      .then((themedUrl) => {
        if (active) setUrl(themedUrl);
      })
      .catch(() => {
        if (active) setUrl(sourceUrl);
      });

    return () => {
      active = false;
    };
  }, [cacheKey, sourceUrl, transform]);

  return url;
}

export function useThemedCardFace(
  rank: string | undefined,
  suit: string | undefined,
  redColor: string,
  blackColor: string,
): string {
  const sourceUrl = `/res/cards/${rank}${suit}.svg`;
  const suitColor = suit === "♦" || suit === "♥" ? redColor : blackColor;
  const sourceSuitColor = suit === "♦" || suit === "♥" ? "#df0000" : "#000000";
  const isCourtCard = rank === "J" || rank === "Q" || rank === "K";
  const themedStyles = `<style>
    text, [id^="dl"], [id^="hl"], [id^="sl"], [id^="cl"] {
      fill: ${suitColor} !important;
    }
  </style>`;
  const applyTheme = useCallback(
    (svg: string) =>
      isCourtCard
        ? svg.replace("</svg>", `${themedStyles}</svg>`)
        : svg.replaceAll(sourceSuitColor, suitColor),
    [isCourtCard, sourceSuitColor, suitColor, themedStyles],
  );

  return useThemedCardImage(
    sourceUrl,
    `${sourceUrl}:${suitColor}`,
    applyTheme,
  );
}

export function useThemedCardBack(color: string): string {
  const sourceUrl = "/res/cards/back.svg";
  const applyTheme = useCallback(
    (svg: string) => svg.replaceAll("#fb0f0c", color),
    [color],
  );
  return useThemedCardImage(sourceUrl, `${sourceUrl}:${color}`, applyTheme);
}
