const REFERRAL_PARAM = "ref";
const CLOUD_ORIGIN = "https://cloud.tracewayapp.com";

// Must match the code pattern Traceway Cloud accepts (its
// cloud/models/affiliate.model.go).
const CODE_PATTERN = /^[a-z0-9][a-z0-9_-]{2,31}$/;

// The code of the affiliate link this visit started from. It lives in memory
// and in the address of the page being viewed, never in browser storage, and
// reaches Traceway Cloud on the links that lead there.
let visitCode: string | null = null;

export function captureReferral(search: string) {
  const code = (new URLSearchParams(search).get(REFERRAL_PARAM) ?? "")
    .trim()
    .toLowerCase();
  if (CODE_PATTERN.test(code)) visitCode = code;
}

// Moving around the site drops the query, and a reload or a restored tab
// knows only the address, so the code is put back into it.
export function addressWithReferral(href: string): string | null {
  if (!visitCode) return null;
  const url = new URL(href);
  if (url.searchParams.get(REFERRAL_PARAM) === visitCode) return null;
  url.searchParams.set(REFERRAL_PARAM, visitCode);
  return url.pathname + url.search + url.hash;
}

// A link to another page of this site carries the code too: a page opened in
// a new tab starts with no memory, and the code in its address is what hands
// it over. A link within the current page is left alone, since a changed
// query would turn a jump to a heading into a page load.
function leadsToAnotherPage(url: URL): boolean {
  if (typeof window === "undefined") return false;
  return (
    url.origin === window.location.origin &&
    url.pathname !== window.location.pathname
  );
}

export function withReferral(href: string): string {
  if (!visitCode) return href;
  let url: URL;
  try {
    url = new URL(href);
  } catch {
    return href;
  }
  if (url.origin !== CLOUD_ORIGIN && !leadsToAnotherPage(url)) return href;
  if (url.searchParams.has(REFERRAL_PARAM)) return href;
  url.searchParams.set(REFERRAL_PARAM, visitCode);
  return url.toString();
}
