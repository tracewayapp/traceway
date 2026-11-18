"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { Cookie, X } from "lucide-react";

import {
  CONSENT_SETTINGS_EVENT,
  SERVER_SNAPSHOT,
  applyConsent,
  consentRequired,
  consentSnapshot,
  hasGlobalPrivacyControl,
  readConsent,
  subscribeConsent,
  writeConsent,
} from "@/lib/consent";

type Regime = "pending" | "optin" | "optout";

export function CookieConsent() {
  const snapshot = useSyncExternalStore(
    subscribeConsent,
    consentSnapshot,
    () => SERVER_SNAPSHOT,
  );
  const [regime, setRegime] = useState<Regime>("pending");
  const regimeRef = useRef<Regime>("pending");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [analytics, setAnalytics] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const stored = readConsent();
    if (stored) {
      applyConsent(stored);
    } else {
      consentRequired().then((required) => {
        if (cancelled) return;
        const next: Regime = required ? "optin" : "optout";
        regimeRef.current = next;
        setRegime(next);
        if (!required && !hasGlobalPrivacyControl()) {
          applyConsent({ analytics: true });
        }
      });
    }

    const onOpen = () => {
      const implied =
        regimeRef.current === "optout" && !hasGlobalPrivacyControl();
      setAnalytics(readConsent()?.analytics ?? implied);
      setSettingsOpen(true);
    };
    window.addEventListener(CONSENT_SETTINGS_EVENT, onOpen);
    return () => {
      cancelled = true;
      window.removeEventListener(CONSENT_SETTINGS_EVENT, onOpen);
    };
  }, []);

  function decide(next: boolean) {
    writeConsent({ analytics: next });
    setAnalytics(next);
    setSettingsOpen(false);
  }

  if (snapshot === SERVER_SNAPSHOT) return null;
  const hasChoice = snapshot !== "";
  const view = settingsOpen
    ? "settings"
    : hasChoice || regime !== "optin"
      ? "hidden"
      : "banner";
  if (view === "hidden") return null;

  const optOutRegime = regime === "optout";

  return (
    <div
      className="cookie-banner"
      role="dialog"
      aria-labelledby="cookie-banner-title"
      aria-describedby="cookie-banner-desc"
    >
      <div className="cookie-banner-head">
        <span className="eyebrow">Cookies</span>
        {hasChoice || optOutRegime ? (
          <button
            type="button"
            className="cookie-banner-close"
            onClick={() => setSettingsOpen(false)}
            aria-label="Close cookie settings"
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      <h2 id="cookie-banner-title" className="cookie-banner-title">
        <Cookie className="h-4 w-4" aria-hidden />
        {view === "settings" ? "Cookie settings" : "We use one optional cookie"}
      </h2>

      <p id="cookie-banner-desc" className="cookie-banner-text">
        This site sets a strictly necessary cookie to remember your choice.{" "}
        {optOutRegime
          ? "Google Analytics counts visits in aggregate and you can turn it off here at any time."
          : "Google Analytics is loaded only if you allow it, to count visits in aggregate."}{" "}
        No advertising, no cross-site tracking.{" "}
        <Link href="/privacy-policy#cookies">How we use cookies</Link>
      </p>

      {view === "settings" ? (
        <div className="cookie-banner-options">
          <ConsentRow
            title="Strictly necessary"
            detail="Remembers your cookie choice for six months. Always on."
            checked
            disabled
          />
          <ConsentRow
            title="Analytics"
            detail="Google Analytics 4. Sets _ga cookies for up to two years."
            checked={analytics}
            onChange={setAnalytics}
          />
        </div>
      ) : null}

      <div className="cookie-banner-actions">
        {view === "settings" ? (
          <>
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={() => decide(false)}
            >
              Reject all
            </button>
            <button
              type="button"
              className="btn btn-accent btn-sm"
              onClick={() => decide(analytics)}
            >
              Save choices
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={() => decide(false)}
            >
              Reject all
            </button>
            <button
              type="button"
              className="btn btn-accent btn-sm"
              onClick={() => decide(true)}
            >
              Accept all
            </button>
            <button
              type="button"
              className="cookie-banner-link"
              onClick={() => setSettingsOpen(true)}
            >
              Customize
            </button>
          </>
        )}
      </div>
    </div>
  );
}

function ConsentRow({
  title,
  detail,
  checked,
  disabled,
  onChange,
}: {
  title: string;
  detail: string;
  checked: boolean;
  disabled?: boolean;
  onChange?: (next: boolean) => void;
}) {
  return (
    <div className="cookie-banner-row">
      <div className="min-w-0">
        <div className="cookie-banner-row-title">{title}</div>
        <div className="cookie-banner-row-detail">{detail}</div>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={title}
        disabled={disabled}
        className="cookie-switch"
        data-on={checked}
        onClick={() => onChange?.(!checked)}
      >
        <span className="cookie-switch-thumb" />
      </button>
    </div>
  );
}
