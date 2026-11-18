export const LEGAL = {
  entity: "Traceway Tech, Inc.",
  entityDescription: "a Delaware corporation",
  address: "Dallas, TX, United States",
  country: "United States",
  governingLaw: "the State of Delaware, United States",
  venue: "the State of Delaware",
  privacyEmail: "privacy@tracewayapp.com",
  updated: "October 24, 2026",
  website: "https://tracewayapp.com",
  cloud: "https://cloud.tracewayapp.com",
  docs: "https://docs.tracewayapp.com",
};

export const GA_MEASUREMENT_ID = "G-KSB465GF2W";

export const CONSENT_COOKIE = "tw_consent";

export type Subprocessor = {
  name: string;
  entity: string;
  location: string;
  purpose: string;
  customerData: boolean;
  transfer: string;
};

export const SUBPROCESSORS: Subprocessor[] = [
  {
    name: "Amazon Web Services",
    entity: "Amazon Web Services, Inc.",
    location: "United States (us-east-1, N. Virginia)",
    purpose:
      "Hosting of Traceway Cloud: compute, PostgreSQL, ClickHouse, object storage for session replays and AI conversation payloads, backups",
    customerData: true,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
  {
    name: "Twilio SendGrid",
    entity: "Twilio Inc.",
    location: "United States",
    purpose:
      "Transactional email: invitations, password resets, alert and on-call emails, and submissions of the website contact and subscribe forms",
    customerData: true,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
  {
    name: "Twilio",
    entity: "Twilio Inc.",
    location: "United States",
    purpose:
      "SMS delivery for on-call paging, only for users who add an SMS contact method",
    customerData: true,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
  {
    name: "Stripe",
    entity: "Stripe, Inc. / Stripe Payments Europe, Ltd.",
    location: "United States / Ireland",
    purpose:
      "Subscription billing and payment processing. Card details are entered on Stripe-hosted pages and never reach our servers",
    customerData: false,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
  {
    name: "Cloudflare",
    entity: "Cloudflare, Inc.",
    location: "Global edge network, United States",
    purpose:
      "Hosting and protection of tracewayapp.com and docs.tracewayapp.com, the website form handler, and the Turnstile bot check on the sign-up form",
    customerData: false,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
  {
    name: "Google Analytics",
    entity: "Google LLC",
    location: "United States",
    purpose:
      "Aggregate usage statistics for tracewayapp.com, only with your consent in the EEA, UK and Switzerland, and unless you opt out elsewhere",
    customerData: false,
    transfer: "EU-US Data Privacy Framework, Standard Contractual Clauses",
  },
];
