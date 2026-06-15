import { useRouter } from "next/router";
import SdkSelector from "./components/SdkSelector";
import HiddenItem from "./components/HiddenItem";
import { useSdk } from "./components/SdkContext";

const SDK_VISIBILITY = {
  "gin-middleware": "go-gin",
  "chi-middleware": "go-chi",
  "fiber-middleware": "go-fiber",
  "fasthttp-middleware": "go-fasthttp",
  "http-middleware": "go-http",
  sdk: "go-",
  "node-sdk": "js-node",
  nestjs: "js-nestjs",
  hono: "js-hono",
  react: "js-react",
  "react-native": "react-native",
  vue: "js-vue",
  svelte: "js-svelte",
  jquery: "js-jquery",
  "js-sdk": ["js-react", "js-vue", "js-svelte", "js-jquery", "js-generic"],
  openrouter: "openrouter",
  otel: "otel",
  cloudflare: "cloudflare",
  nextjs: "js-nextjs",
  symfony: "php-symfony",
  laravel: "php-laravel",
  django: "python-django",
  flutter: "flutter",
  android: "android",
};

export default {
  logoLink: "https://tracewayapp.com",
  logo: function Logo() {
    return (
      <img
        src="/traceway-logo-white.png"
        alt="Traceway"
        style={{ height: "28px" }}
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
    const { asPath } = useRouter();
    const cleanPath = asPath.split("?")[0].split("#")[0];
    return {
      titleTemplate: "%s - Traceway Docs",
      canonical: `https://docs.tracewayapp.com${cleanPath === "/" ? "" : cleanPath}`,
    };
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta
        name="description"
        content="Traceway - Error tracking and monitoring platform"
      />
      <meta name="theme-color" content="#05070c" />
      <meta name="color-scheme" content="dark" />
    </>
  ),
  // Electric Indigo (#7c5cff) ≈ hsl(253, 100%, 68%)
  primaryHue: 253,
  primarySaturation: 100,
  darkMode: false,
  nextThemes: {
    defaultTheme: "dark",
    forcedTheme: "dark",
  },
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
    titleComponent({ title, type, route }) {
      if (type === "separator" && title === "sdk-selector") {
        return <SdkSelector />;
      }

      const segment = route?.startsWith("/client")
        ? route.split("/").filter(Boolean)[1]
        : undefined;
      const requiredSdk = segment ? SDK_VISIBILITY[segment] : undefined;
      if (requiredSdk !== undefined) {
        return <SdkGuard requiredSdk={requiredSdk}>{title}</SdkGuard>;
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
  if (!sdk) {
    return <HiddenItem />;
  }
  let visible;
  if (Array.isArray(requiredSdk)) {
    visible = requiredSdk.includes(sdk);
  } else if (requiredSdk.endsWith("-")) {
    visible = sdk.startsWith(requiredSdk);
  } else {
    visible = sdk === requiredSdk;
  }
  if (!visible) {
    return <HiddenItem />;
  }
  return <>{children}</>;
}
