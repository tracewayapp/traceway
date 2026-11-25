import Image from "next/image";
import { cn } from "@/lib/utils";

// Thanksgiving decorations only show in the second half of November. The site is
// a static export, so the date is checked in the browser, before first paint, to
// keep a deploy made in season from showing them all year and to keep the header
// from shifting when they appear.
export const HARVEST_SEASON_SCRIPT =
  '(function(){var d=new Date();if(d.getMonth()===10&&d.getDate()>=15)document.documentElement.classList.add("harvest-season")})()';

export function Harvest({
  src,
  width,
  height,
  className,
}: {
  src: string;
  width: number;
  height: number;
  className?: string;
}) {
  return (
    <Image
      src={src}
      alt=""
      aria-hidden
      width={width}
      height={height}
      className={cn("harvest", className)}
    />
  );
}
