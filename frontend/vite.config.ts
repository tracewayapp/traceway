import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import type { Plugin } from 'vite';
import { readFileSync, existsSync } from 'fs';
import path from 'path';

const pkg = JSON.parse(readFileSync('./package.json', 'utf-8'));

const billingPath = process.env.BILLING_PATH;
const resolvedBillingPath = billingPath ? path.resolve(import.meta.dirname, billingPath) : null;
const billingExists = resolvedBillingPath && existsSync(resolvedBillingPath);

function stubBillingImports(): Plugin {
	if (billingExists) return { name: 'stub-billing-imports' };
	return {
		name: 'stub-billing-imports',
		enforce: 'pre',
		resolveId(id) {
			if (id.startsWith('$billing/')) {
				return '\0virtual:billing-stub';
			}
		},
		load(id) {
			if (id === '\0virtual:billing-stub') {
				return 'export default null';
			}
		}
	};
}

function injectBillingSource(): Plugin {
	return {
		name: 'inject-billing-source',
		enforce: 'pre',
		transform(code, id) {
			// Quote-agnostic: prettier (singleQuote) rewrites the placeholder's quotes in CSS
			const placeholder = /@source\s+(['"])\$BILLING_PATH\1;?/;
			if (id.endsWith('.css') && placeholder.test(code)) {
				if (billingExists) {
					return code.replace(placeholder, `@source "${resolvedBillingPath}";`);
				} else {
					return code.replace(placeholder, '');
				}
			}
		}
	};
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');

	return {
		plugins: [stubBillingImports(), injectBillingSource(), tailwindcss(), sveltekit()],
		define: {
			__APP_VERSION__: JSON.stringify(env.PUBLIC_APP_VERSION || pkg.version),
			__CLOUD_MODE__: env.CLOUD_MODE,
			__BILLING_AVAILABLE__: billingExists,
			__TRACEWAY_URL__: JSON.stringify(env.TRACEWAY_URL || ''),
			__TURNSTILE_SITE_KEY__: JSON.stringify(env.PUBLIC_TURNSTILE_SITE_KEY || '')
		},
		build: {
			sourcemap: 'hidden'
		},
		resolve: {
			dedupe: ['d3-scale', 'd3-array', '@lucide/svelte', 'svelte', 'svelte-sonner']
		},
		optimizeDeps: {
			// @milkdown/crepe is dynamically imported; pre-bundle it so the first
			// editor open doesn't trigger a mid-session re-optimization page reload
			// that strands the editor on its loading state.
			include: ['d3-scale', 'd3-array', '@lucide/svelte', 'svelte-sonner', '@milkdown/crepe']
		},
		server: {
			proxy: {
				'/api': {
					target: env.TRACEWAY_API_PROXY || 'http://localhost:8082',
					changeOrigin: true
					// rewrite: (path) => path.replace(/^\/api/, '')
				}
			}
		}
	};
});
