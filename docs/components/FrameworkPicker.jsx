import { useRouter } from "next/router";
import { useSdk } from "./SdkContext";

const FRAMEWORKS = [
  {
    value: "openrouter",
    label: "OpenRouter",
    description: "AI observability for OpenRouter with automatic OTLP trace export.",
    icon: "/openrouter.png",
    href: "/client/openrouter",
  },
  {
    value: "otel",
    label: "OpenTelemetry (otel)",
    description: "Send traces and metrics from any OTel-instrumented app to Traceway.",
    icon: "/otel.png",
    href: "/client/otel",
  },
  {
    value: "cloudflare",
    label: "Cloudflare Workers",
    description: "Cloudflare Workers with automatic request tracing via OTLP.",
    icon: "/cloudflare.png",
    href: "/client/cloudflare",
  },
  {
    value: "php-symfony",
    label: "Symfony",
    description: "Symfony framework with OpenTelemetry auto-instrumentation.",
    icon: "/symfony.png",
    href: "/client/symfony",
  },
  {
    value: "php-laravel",
    label: "Laravel",
    description: "Laravel framework with OpenTelemetry auto-instrumentation.",
    icon: "/laravel.png",
    href: "/client/laravel",
  },
  {
    value: "python-django",
    label: "Django",
    description: "Django framework with OpenTelemetry auto-instrumentation.",
    icon: "/django.png",
    href: "/client/django",
  },
  {
    value: "go-gin",
    label: "Go Gin",
    description:
      "Gin Gonic web framework with automatic request tracing and panic recovery.",
    icon: "/gin.png",
    href: "/client/gin-middleware",
  },
  {
    value: "go-chi",
    label: "Go Chi",
    description:
      "Lightweight Chi router with automatic request tracing and panic recovery.",
    icon: "/chi.png",
    href: "/client/chi-middleware",
  },
  {
    value: "go-fiber",
    label: "Go Fiber",
    description:
      "Express-inspired Fiber framework with request tracing and error capture.",
    icon: "/fiber.svg",
    href: "/client/fiber-middleware",
  },
  {
    value: "go-fasthttp",
    label: "Go FastHTTP",
    description:
      "High-performance FastHTTP server with request tracing and panic recovery.",
    icon: "/fasthttp.png",
    href: "/client/fasthttp-middleware",
  },
  {
    value: "go-http",
    label: "Go net/http",
    description:
      "Standard library HTTP middleware for request tracing and error capture.",
    icon: "/stdlib.png",
    href: "/client/http-middleware",
  },
  {
    value: "go-generic",
    label: "Go Generic",
    description:
      "Framework-agnostic SDK for manual instrumentation of any Go application.",
    icon: "/custom.png",
    href: "/client/sdk",
  },
  {
    value: "js-nextjs",
    label: "Next.js",
    description: "Next.js applications with OpenTelemetry auto-instrumentation.",
    icon: "/nextjs.png",
    href: "/client/nextjs",
  },
  {
    value: "js-node",
    label: "Node.js",
    description: "Node.js backend with OpenTelemetry traces and metrics.",
    icon: "/node.png",
    href: "/client/node-sdk",
  },
  {
    value: "js-nestjs",
    label: "NestJS",
    description: "NestJS framework with OpenTelemetry auto-instrumentation.",
    icon: "/nestjs.png",
    href: "/client/nestjs",
  },
  {
    value: "js-hono",
    label: "Hono",
    description: "Lightweight multi-runtime framework with OpenTelemetry.",
    icon: "/hono.png",
    href: "/client/hono",
  },
  {
    value: "js-react",
    label: "React",
    description: "React applications with error boundaries and hooks.",
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
    value: "react-native",
    label: "React Native",
    description: "React Native and Expo apps with automatic exception, fetch / XHR, and console capture. Works in Expo Go.",
    icon: "/react.png",
    href: "/client/react-native",
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
      <p className="framework-picker-subheading">
        Select the framework you're using to get started with Traceway.
      </p>
      <div className="framework-picker-grid">
        {FRAMEWORKS.map((fw) => (
          <button
            key={fw.value}
            className="framework-picker-card"
            onClick={() => handleSelect(fw)}
          >
            <img
              src={fw.icon}
              alt={fw.label}
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
