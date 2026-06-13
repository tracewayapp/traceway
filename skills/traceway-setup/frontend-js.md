# JS Frontend SDKs Reference

Browser-side setup for every supported framework, plus the bundler plugin and source map upload that complete each integration. The connection string everywhere is `<project-token>@https://<instance>/api/report`.

## Packages

| Package | Use for |
|---|---|
| `@tracewayapp/frontend` | Plain JS, vanilla apps, frameworks without official bindings |
| `@tracewayapp/react` | React: provider, error boundary, hooks |
| `@tracewayapp/vue` | Vue 3: plugin and composables |
| `@tracewayapp/svelte` | Svelte and SvelteKit: context-based setup |
| `@tracewayapp/jquery` | jQuery: automatic AJAX error capture |

All framework packages wrap the core SDK and accept the same options object (see Options below). Every SDK captures uncaught errors and unhandled promise rejections, batches uploads with retry, and ships session replay: the last ~30s of DOM events (rrweb) accompany every captured exception by default.

## Plain JS (`@tracewayapp/frontend`)

```javascript
import { init, captureException, captureMessage } from "@tracewayapp/frontend";

init("your-token@https://traceway.example.com/api/report");

try {
  riskyOperation();
} catch (error) {
  captureException(error);
}

captureMessage("User completed checkout");
```

Via CDN (exposes `window.Traceway`, no build step):

```html
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/frontend@1/dist/traceway.iife.global.js"></script>
<script>
  Traceway.init("your-token@https://traceway.example.com/api/report");
</script>
```

## React (`@tracewayapp/react`)

Wrap the app in `TracewayProvider`. It doubles as an error boundary: render-time exceptions anywhere in the tree are captured and re-thrown, so app behavior is unchanged. `TracewayErrorBoundary` is still exported for the case where a crashed subtree should show a fallback UI instead of propagating; it is not required for capture.

```jsx
import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider connectionString="your-token@https://traceway.example.com/api/report">
      <YourApp />
    </TracewayProvider>
  );
}
```

Manual capture uses the `useTraceway` hook:

```jsx
import { useTraceway } from "@tracewayapp/react";

function MyComponent() {
  const { captureException } = useTraceway();

  async function handleSubmit() {
    try {
      await submitForm();
    } catch (error) {
      captureException(error);
    }
  }
}
```

Options go on the provider: `<TracewayProvider connectionString="..." options={{ debug: true, version: "1.0.0" }}>`.

## Vue 3 (`@tracewayapp/vue`)

The plugin installs a global error handler for uncaught errors:

```javascript
import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);
app.use(createTracewayPlugin({
  connectionString: "your-token@https://traceway.example.com/api/report",
  options: { version: "1.0.0" },
}));
app.mount("#app");
```

Manual capture uses the `useTraceway` composable:

```vue
<script setup>
import { useTraceway } from "@tracewayapp/vue";
const { captureException } = useTraceway();
</script>
```

## Svelte / SvelteKit (`@tracewayapp/svelte`)

Call `setupTraceway` in the root layout. The `if (browser)` guard keeps the rrweb recorder out of the SSR bundle:

```svelte
<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";
  import { browser } from "$app/environment";

  if (browser) {
    setupTraceway({
      connectionString: "your-token@https://traceway.example.com/api/report",
    });
  }
</script>

<slot />
```

For a non-SvelteKit Svelte app, drop the guard and the `$app/environment` import. Manual capture in child components uses `getTraceway()`:

```svelte
<script>
  import { getTraceway } from "@tracewayapp/svelte";
  const { captureException } = getTraceway();
</script>
```

## jQuery (`@tracewayapp/jquery`)

```javascript
import { init, captureException } from "@tracewayapp/jquery";

init("your-token@https://cloud.tracewayapp.com/api/report");
```

Via CDN (exposes `window.TracewayJQuery`):

```html
<script src="https://code.jquery.com/jquery-3.7.1.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/@tracewayapp/jquery@1/dist/traceway-jquery.iife.global.js"></script>
<script>
  TracewayJQuery.init("your-token@https://cloud.tracewayapp.com/api/report");
</script>
```

Beyond the standard capture, the jQuery SDK hooks `$(document).ajaxError()` to capture failed `$.ajax()` calls (URL, method, status, message) and injects trace headers into `$.ajax()` (XHR) as well as `fetch`.

## Options (shared by all JS SDKs)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `debug` | `boolean` | `false` | Log captured events to the browser console |
| `debounceMs` | `number` | `1500` | Batch delay in milliseconds |
| `retryDelayMs` | `number` | `10000` | Retry delay for failed uploads |
| `version` | `string` | `undefined` | App version, shown on exceptions for filtering by build (unrelated to source maps) |
| `ignoreErrors` | `Array<string \| RegExp>` | default patterns | Error patterns to ignore; pass `[]` to capture everything |
| `beforeCapture` | `(exception) => boolean` | `undefined` | Return `false` to suppress an error |
| `sessionRecording` | `boolean` | `true` | rrweb session recorder |
| `sessionRecordingSegmentDuration` | `number` | `30000` | rrweb segment length in ms |
| `recordAllSessions` | `boolean` | `false` | Always-on session recording instead of error-triggered |
| `captureLogs` | `boolean` | `true` | Mirror `console.*` calls into the rolling log buffer |
| `captureNetwork` | `boolean` | `true` | Record `fetch` / XHR calls as network actions |
| `captureNavigation` | `boolean` | `true` | Record History API transitions |

### Error filtering

By default 4xx HTTP errors, network errors, and timeouts are NOT captured. Opt in with `ignoreErrors: []`, and filter selectively with `beforeCapture`:

```javascript
init("your-token@https://<instance>/api/report", {
  ignoreErrors: [],
  beforeCapture: (exception) => {
    const status = Number(exception.attributes?.status);
    if (status >= 400 && status < 500) return false;
    return true;
  },
});
```

### Custom attributes

Attach app-level identifiers to every session and exception. Layering on each event: defaults < global scope < per-call.

```javascript
import { setAttribute, setAttributes, clearAttributes } from "@tracewayapp/jquery";

setAttribute("userId", "u_42");
setAttributes({ tenant: "acme", plan: "pro" });
clearAttributes();
```

(Same exports from each framework package.)

### Distributed tracing

The SDK instruments `fetch` (and XHR in the jQuery SDK) to propagate a `traceway-trace-id` header on same-origin requests, linking frontend errors to the backend requests that caused them. No configuration. For Axios:

```javascript
import { createAxiosInterceptor } from "@tracewayapp/frontend";

api.interceptors.request.use(createAxiosInterceptor());
```

## Bundler Plugin (debug IDs)

`@tracewayapp/bundler-plugin` embeds a deterministic 128-bit debug ID into each emitted chunk and its `.map` (ECMA-426 debug ID format, interoperable with Sentry tooling). The backend then matches stack frames to the exact map from the same build, immune to filename collisions and concurrent deploys. Without it, matching falls back to filename (fine with content-hashed bundle names, broken with stable names like `app.js`).

```bash
npm install -D @tracewayapp/bundler-plugin
```

Vite:

```ts
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: { sourcemap: true },
  plugins: [tracewayDebugIds()],
});
```

Rollup: same import from `@tracewayapp/bundler-plugin/rollup`, with `output: { sourcemap: true }`.

webpack (requires webpack 5):

```js
const { TracewayDebugIdsWebpackPlugin } = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};
```

esbuild is not covered; bundles processed by Sentry's esbuild plugin still work (the SDK reads `_sentryDebugIds` too), otherwise esbuild falls back to filename matching.

## Source Map Upload

Uploads authenticate with a dedicated upload token (Connection page > Source Maps > Generate Upload Token), NOT the project token. It is a CI secret; never commit it. `readonly` members cannot generate one.

```bash
npm install -D @tracewayapp/sourcemap-upload

traceway-sourcemaps --url https://<instance> --token <upload-token> --directory ./dist
```

Env vars `TRACEWAY_URL` and `TRACEWAY_SOURCEMAP_TOKEN` replace the flags. The CLI uploads all `*.map` files plus the sibling `.js`/`.cjs`/`.mjs` bundles (the bundle is what enables function-name resolution). Limit: 50 MB per file. Uploads take effect immediately.

Wire it as a postbuild script:

```json
{
  "scripts": {
    "build": "vite build",
    "postbuild": "traceway-sourcemaps --directory ./dist"
  }
}
```

Or as a CI step after build:

```yaml
- name: Upload source maps
  run: npx traceway-sourcemaps --directory ./dist
  env:
    TRACEWAY_URL: ${{ secrets.TRACEWAY_URL }}
    TRACEWAY_SOURCEMAP_TOKEN: ${{ secrets.TRACEWAY_SOURCEMAP_TOKEN }}
```

Self-hosted instances must have blob storage (S3 or a persistent volume) configured, or uploaded maps disappear when the container is recreated.

## Verify

Trigger a test error (`captureException(new Error("Test error"))` behind a button) and check the Issues page: the error appears, and with the bundler plugin plus upload in place the stack trace resolves to original files and lines.
