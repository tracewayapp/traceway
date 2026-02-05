import { useState, useEffect } from "react";
import { useTheme } from "nextra-theme-docs";
import SdkSelector from "./components/SdkSelector";
import HiddenItem from "./components/HiddenItem";
import { useSdk } from "./components/SdkContext";

const SDK_VISIBILITY = {
  "gin-middleware": "go-gin",
  "chi-middleware": "go-chi",
  "fiber-middleware": "go-fiber",
  "fasthttp-middleware": "go-fasthttp",
  "http-middleware": "go-http",
  "node-sdk": "js-node",
  "react": "js-react",
  "vue": "js-vue",
  "svelte": "js-svelte",
};

export default {
  logoLink: "https://tracewayapp.com",
  logo: function Logo() {
    const { resolvedTheme } = useTheme();
    const [mounted, setMounted] = useState(false);
    useEffect(() => setMounted(true), []);
    return (
      <img
        src={
          mounted && resolvedTheme === "dark"
            ? "/traceway-logo-white.png"
            : "/traceway-logo.png"
        }
        alt="Traceway"
        style={{ height: "32px" }}
      />
    );
  },
  project: {
    link: "https://github.com/tracewayapp/traceway",
  },
  docsRepositoryBase: "https://github.com/tracewayapp/traceway/tree/main/docs",
  footer: {
    text: `${new Date().getFullYear()} Traceway. All rights reserved.`,
  },
  useNextSeoProps() {
    return {
      titleTemplate: "%s - Traceway Docs",
    };
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta
        name="description"
        content="Traceway - Error tracking and monitoring platform"
      />
    </>
  ),
  primaryHue: 205,
  darkMode: true,
  nextThemes: {
    defaultTheme: "light",
  },
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
    titleComponent({ title, type, route }) {
      if (type === "separator" && title === "sdk-selector") {
        return <SdkSelector />;
      }

      for (const [folder, requiredSdk] of Object.entries(SDK_VISIBILITY)) {
        if (route && route.includes(`/${folder}`)) {
          return <SdkGuard requiredSdk={requiredSdk}>{title}</SdkGuard>;
        }
      }

      return <>{title}</>;
    },
  },
  toc: {
    backToTop: true,
  },
  editLink: {
    text: "Edit this page on GitHub",
  },
  feedback: {
    content: null,
  },
};

function SdkGuard({ requiredSdk, children }) {
  const { sdk } = useSdk();
  if (sdk !== requiredSdk) {
    return <HiddenItem />;
  }
  return <>{children}</>;
}
