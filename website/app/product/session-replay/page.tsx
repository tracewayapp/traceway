import type { Metadata } from "next";
export { default } from "../rum/page";

export const metadata: Metadata = {
  title: "Real User Monitoring · Traceway",
  alternates: { canonical: "https://tracewayapp.com/product/rum" },
  robots: { index: false, follow: true },
};
