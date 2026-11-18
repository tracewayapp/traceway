import { CONSENT_COOKIE, GA_MEASUREMENT_ID } from "@/lib/legal";

export type Consent = { analytics: boolean };

export const CONSENT_SETTINGS_EVENT = "tw:cookie-settings";
export const CONSENT_CHANGED_EVENT = "tw:consent-changed";

const CONSENT_MAX_AGE_SECONDS = 182 * 24 * 60 * 60;

declare global {
  interface Window {
    dataLayer?: unknown[];
    gtag?: (...args: unknown[]) => void;
    [key: `ga-disable-${string}`]: boolean | undefined;
  }
}

let memoryChoice: string | null = null;

function cookieEntries(): string[] {
  try {
    return document.cookie.split("; ");
  } catch {
    return [];
  }
}

function setCookie(value: string) {
  try {
    document.cookie = value;
  } catch {
    // Embedded pages can be denied cookie access altogether.
  }
}

function consentCookieValue() {
  return cookieEntries()
    .find((entry) => entry.startsWith(`${CONSENT_COOKIE}=`))
    ?.slice(CONSENT_COOKIE.length + 1);
}

export function readConsent(): Consent | null {
  if (typeof document === "undefined") return null;
  let value = memoryChoice;
  if (value === null) {
    try {
      value = sessionStorage.getItem(CONSENT_COOKIE);
    } catch {}
  }
  value ??= consentCookieValue() ?? null;
  if (value === "v1:analytics=1") return { analytics: true };
  if (value === "v1:analytics=0") return { analytics: false };
  return null;
}

export function writeConsent(consent: Consent) {
  const value = `v1:analytics=${consent.analytics ? 1 : 0}`;
  const secure = location.protocol === "https:" ? "; Secure" : "";
  setCookie(`${CONSENT_COOKIE}=${value}; Max-Age=${CONSENT_MAX_AGE_SECONDS}; Path=/; SameSite=Lax${secure}`);
  // Browsers can silently discard a SameSite cookie in an embedded preview.
  memoryChoice = consentCookieValue() === value ? null : value;
  try {
    if (memoryChoice === null) sessionStorage.removeItem(CONSENT_COOKIE);
    else sessionStorage.setItem(CONSENT_COOKIE, memoryChoice);
  } catch {}
  applyConsent(consent);
  window.dispatchEvent(
    new CustomEvent<Consent>(CONSENT_CHANGED_EVENT, { detail: consent }),
  );
}

export const SERVER_SNAPSHOT = "server";

export function consentSnapshot(): string {
  if (typeof document === "undefined") return SERVER_SNAPSHOT;
  const stored = readConsent();
  return stored ? `v1:analytics=${stored.analytics ? 1 : 0}` : "";
}

export function subscribeConsent(onChange: () => void) {
  window.addEventListener(CONSENT_CHANGED_EVENT, onChange);
  return () => window.removeEventListener(CONSENT_CHANGED_EVENT, onChange);
}

export function hasGlobalPrivacyControl(): boolean {
  return (
    (navigator as Navigator & { globalPrivacyControl?: boolean })
      .globalPrivacyControl === true
  );
}

const GEO_CACHE_KEY = "tw_geo";

// Whether this visitor is somewhere analytics needs opt-in consent. Any
// failure answers true so the banner is shown rather than skipped.
export async function consentRequired(): Promise<boolean> {
  try {
    const cached = sessionStorage.getItem(GEO_CACHE_KEY);
    if (cached === "optin" || cached === "optout") return cached === "optin";
  } catch {}
  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 2500);
    const res = await fetch("/api/geo", {
      signal: controller.signal,
      cache: "no-store",
    });
    clearTimeout(timer);
    if (!res.ok) return true;
    const body = (await res.json()) as { consentRequired?: boolean };
    const required = body.consentRequired !== false;
    try {
      sessionStorage.setItem(GEO_CACHE_KEY, required ? "optin" : "optout");
    } catch {}
    return required;
  } catch {
    return true;
  }
}

export function openConsentSettings() {
  window.dispatchEvent(new Event(CONSENT_SETTINGS_EVENT));
}

export function applyConsent(consent: Consent) {
  if (consent.analytics) loadAnalytics();
  else disableAnalytics();
}

let analyticsLoaded = false;

// gtag.js ignores dataLayer entries that are not an Arguments object.
const gtag = function () {
  window.dataLayer = window.dataLayer || [];
  // eslint-disable-next-line prefer-rest-params
  window.dataLayer.push(arguments);
} as (...args: unknown[]) => void;

function loadAnalytics() {
  window[`ga-disable-${GA_MEASUREMENT_ID}`] = false;
  if (analyticsLoaded) return;
  analyticsLoaded = true;

  window.gtag = gtag;
  gtag("consent", "default", {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "denied",
  });
  gtag("consent", "update", { analytics_storage: "granted" });
  gtag("js", new Date());
  gtag("config", GA_MEASUREMENT_ID);

  const script = document.createElement("script");
  script.async = true;
  script.src = `https://www.googletagmanager.com/gtag/js?id=${GA_MEASUREMENT_ID}`;
  document.head.appendChild(script);
}

function disableAnalytics() {
  window[`ga-disable-${GA_MEASUREMENT_ID}`] = true;
  if (analyticsLoaded) {
    gtag("consent", "update", { analytics_storage: "denied" });
  }
  const expired = "Max-Age=0; Path=/";
  const hosts = [location.hostname, `.${location.hostname}`];
  const parts = location.hostname.split(".");
  if (parts.length > 2) hosts.push(`.${parts.slice(-2).join(".")}`);
  for (const name of cookieEntries().map((c) => c.split("=")[0])) {
    if (name !== "_ga" && !name.startsWith("_ga_")) continue;
    for (const host of hosts) {
      setCookie(`${name}=; ${expired}; Domain=${host}`);
    }
    setCookie(`${name}=; ${expired}`);
  }
}
