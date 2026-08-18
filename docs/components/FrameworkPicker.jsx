import { useRouter } from "next/router";
import { useSdk } from "./SdkContext";

const FRAMEWORKS = [
  // Backends are deliberately one card. OpenTelemetry is the only backend
  // option in the dashboard's project picker, so the docs picker matches it.
  // Per-framework guides (Node.js, NestJS, Next.js, Hono, Cloudflare, Symfony,
  // Laravel, Python, Django) are listed on /client/otel, not here.
  {
    value: "otel",
    label: "OpenTelemetry",
    description: "Every backend, whatever the language: Go, Node.js, Python, PHP, Java, .NET, Ruby. Start here.",
    icon: "/otel.png",
    href: "/client/otel",
  },
  {
    value: "js-react",
    label: "React",
    description: "React apps in the browser, including Next.js and Remix front ends. Their server code uses OpenTelemetry.",
    icon: "/react.png",
    href: "/client/react",
  },
  {
    value: "js-vue",
    label: "Vue.js",
    description: "Vue 3 applications with plugin and composables.",
    icon: "/vue.png",
    href: "/client/vue",
  },
  {
    value: "js-svelte",
    label: "Svelte",
    description: "Svelte/SvelteKit applications with context API.",
    icon: "/svelte.png",
    href: "/client/svelte",
  },
  {
    value: "js-jquery",
    label: "jQuery",
    description: "jQuery applications with automatic AJAX error capture.",
    icon: "/jquery.png",
    href: "/client/jquery",
  },
  {
    value: "js-generic",
    label: "JS Generic",
    description: "Framework-agnostic JavaScript SDK for browsers.",
    icon: "/javascript.png",
    href: "/client/js-sdk",
  },
  {
    value: "flutter",
    label: "Flutter",
    description: "Flutter mobile apps with automatic error capture and screen recording.",
    icon: "/flutter.png",
    href: "/client/flutter",
  },
  {
    value: "react-native",
    label: "React Native",
    description: "React Native and Expo apps with automatic exception, fetch / XHR, and console capture. Works in Expo Go.",
    icon: "/react.png",
    href: "/client/react-native",
  },
  {
    value: "android",
    label: "Android",
    description: "Native Android (Kotlin/Java) apps with automatic exception capture, logs, HTTP, and navigation breadcrumbs.",
    icon: "/android.png",
    href: "/client/android",
  },
  {
    value: "ios",
    label: "iOS",
    description: "Native iOS (Swift) apps with automatic crash and exception capture via Swift Package Manager.",
    icon: "/ios.png",
    iconClassName: "framework-picker-icon--adaptive",
    href: "/client/ios",
  },
  {
    value: "openrouter",
    label: "OpenRouter",
    description: "AI observability for OpenRouter with automatic OTLP trace export.",
    icon: "/openrouter.png",
    href: "/client/openrouter",
  },
];

export default function FrameworkPicker() {
  const router = useRouter();
  const { setSdk } = useSdk();

  function handleSelect(fw) {
    setSdk(fw.value);
    router.push(fw.href);
  }

  return (
    <div className="framework-picker">
      <h2 className="framework-picker-heading">Choose your framework</h2>
      <div className="framework-picker-grid">
        {FRAMEWORKS.map((fw) => (
          <button
            key={fw.href}
            className="framework-picker-card"
            onClick={() => handleSelect(fw)}
          >
            <img
              src={fw.icon}
              alt=""
              className={`framework-picker-icon${fw.iconClassName ? ` ${fw.iconClassName}` : ""}`}
            />
            <span className="framework-picker-label">{fw.label}</span>
            <span className="framework-picker-desc">{fw.description}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
