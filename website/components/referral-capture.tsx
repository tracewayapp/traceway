"use client";

import { Suspense, useEffect } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import {
  addressWithReferral,
  captureReferral,
  withReferral,
} from "@/lib/referral";

function ReferralAddress() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  useEffect(() => {
    captureReferral(window.location.search);
    const address = addressWithReferral(window.location.href);
    if (address) window.history.replaceState(null, "", address);
  }, [pathname, searchParams]);

  return null;
}

export function ReferralCapture() {
  useEffect(() => {
    const decorate = (event: Event) => {
      if (!(event.target instanceof Element)) return;
      const anchor = event.target.closest("a[href]");
      if (!(anchor instanceof HTMLAnchorElement)) return;
      const href = withReferral(anchor.href);
      if (href === anchor.href) return;
      // A link within this site stays a relative one, as it was written.
      const url = new URL(href);
      anchor.href =
        url.origin === window.location.origin
          ? url.pathname + url.search + url.hash
          : href;
    };

    // pointerdown as well as click: a link opened from the context menu or
    // with the middle button never fires click.
    document.addEventListener("pointerdown", decorate, true);
    document.addEventListener("click", decorate, true);
    return () => {
      document.removeEventListener("pointerdown", decorate, true);
      document.removeEventListener("click", decorate, true);
    };
  }, []);

  // useSearchParams needs a Suspense boundary on a statically exported page.
  return (
    <Suspense fallback={null}>
      <ReferralAddress />
    </Suspense>
  );
}
