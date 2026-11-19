import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";
import { MotionPolish } from "@/components/motion-polish";
import { CookieConsent } from "@/components/cookie-consent";
import type { Metadata } from "next";
import "@fontsource-variable/jetbrains-mono/wght.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "@fontsource/ibm-plex-mono/600.css";
import "@fontsource-variable/inter/wght.css";
import "./globals.css";

// Dark-only site, no theme toggle.

export const metadata: Metadata = {
  metadataBase: new URL("https://tracewayapp.com"),
  alternates: {
    canonical: "./",
  },
  title: "Traceway · Your OpenTelemetry home",
  description:
    "Bring your data. Get the whole picture. Open-source observability. Self-host or run on Traceway Cloud.",
  icons: {
    icon: [
      { url: "/favicon.svg", type: "image/svg+xml" },
      { url: "/favicon.ico", sizes: "any" },
    ],
    apple: "/apple-touch-icon.png",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">
        <SiteHeader />
        {children}
        <SiteFooter />
        <MotionPolish />
        <CookieConsent />
      </body>
    </html>
  );
}
