import Image from "next/image";
import { cn } from "@/lib/utils";

// Winter holiday decorations only show from November 27 to January 7. The site
// is a static export, so the date is checked in the browser, before first paint,
// to keep a deploy made in season from showing them all year and to keep the
// layout from shifting when they appear.
export const HOLIDAY_SEASON_SCRIPT =
  '(function(){var d=new Date(),m=d.getMonth(),n=d.getDate();if(m===11||(m===10&&n>=27)||(m===0&&n<=7))document.documentElement.classList.add("holiday-season")})()';

export function Holiday({
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
      className={cn("holiday pointer-events-none select-none", className)}
    />
  );
}
