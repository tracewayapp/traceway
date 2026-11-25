import { useSyncExternalStore } from "react";

// Thanksgiving decorations only show in the second half of November. The check
// runs in the browser, so a deploy made in season does not keep them all year.
function isHarvestSeason(now) {
  return now.getMonth() === 10 && now.getDate() >= 15;
}

const subscribe = () => () => {};

export default function Harvest({ src, width, height, className, style }) {
  const inSeason = useSyncExternalStore(
    subscribe,
    () => isHarvestSeason(new Date()),
    () => false,
  );
  if (!inSeason) return null;
  return (
    <img
      src={src}
      alt=""
      aria-hidden="true"
      width={width}
      height={height}
      className={className}
      style={{ userSelect: "none", ...style }}
    />
  );
}
