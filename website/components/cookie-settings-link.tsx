"use client";

import type { CSSProperties } from "react";

import { openConsentSettings } from "@/lib/consent";

export function CookieSettingsLink({
  className,
  style,
}: {
  className?: string;
  style?: CSSProperties;
}) {
  return (
    <button
      type="button"
      onClick={openConsentSettings}
      className={className}
      style={style}
    >
      Cookie settings
    </button>
  );
}
