import { useSyncExternalStore } from "react";

// Winter holiday decorations only show from November 27 to January 7. The check
// runs in the browser, so a deploy made in season does not keep them all year.
function isHolidaySeason(now) {
  const month = now.getMonth();
  const day = now.getDate();
  return month === 11 || (month === 10 && day >= 27) || (month === 0 && day <= 7);
}

const subscribe = () => () => {};

export default function Holiday({ src, width, height, className, style }) {
  const inSeason = useSyncExternalStore(
    subscribe,
    () => isHolidaySeason(new Date()),
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
