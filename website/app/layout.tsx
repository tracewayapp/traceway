import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";
import { MotionPolish } from "@/components/motion-polish";
import type { Metadata } from "next";
import { JetBrains_Mono, IBM_Plex_Mono, Inter } from "next/font/google";
import Script from "next/script";
import "./globals.css";

// Dark-only site, no theme toggle.

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const ibmPlexMono = IBM_Plex_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
});

const inter = Inter({
  variable: "--font-body",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL("https://tracewayapp.com"),
  alternates: {
    canonical: "./",
  },
  title: "Traceway · Open Source APM Built on OpenTelemetry",
  description:
    "Traceway is an open-source APM built on OpenTelemetry. Complete observability: logs, traces, metrics, session replay, and exceptions, all connected. MIT licensed. Self-host free or run on Traceway Cloud.",
  icons: {
    icon: "/favicon.ico",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${jetbrainsMono.variable} ${ibmPlexMono.variable} ${inter.variable}`}
    >
      <Script
        src="https://www.googletagmanager.com/gtag/js?id=G-KSB465GF2W"
        strategy="afterInteractive"
      />
      <Script id="gtag-init" strategy="afterInteractive">
        {`
          window.dataLayer = window.dataLayer || [];
          function gtag(){dataLayer.push(arguments);}
          gtag('js', new Date());
          gtag('config', 'G-KSB465GF2W');
        `}
      </Script>
      <body className="antialiased">
        <SiteHeader />
        {children}
        <SiteFooter />
        <MotionPolish />
      </body>
    </html>
  );
}
