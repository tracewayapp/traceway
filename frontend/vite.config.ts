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

function injectBillingSource(): Plugin {
	return {
		name: 'inject-billing-source',
		enforce: 'pre',
		transform(code, id) {
			if (id.endsWith('.css') && code.includes('@source "$BILLING_PATH"')) {
				if (billingExists) {
					return code.replace('@source "$BILLING_PATH"', `@source "${resolvedBillingPath}"`)
				} else {
					return code.replace('@source "$BILLING_PATH";', '')
				}
			}
		}
	}
}

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');

	return {
		plugins: [injectBillingSource(), tailwindcss(), sveltekit()],
		define: {
			__APP_VERSION__: JSON.stringify(env.PUBLIC_APP_VERSION || pkg.version),
			__CLOUD_MODE__: env.CLOUD_MODE,
			__BILLING_AVAILABLE__: billingExists
		},
		resolve: {
			dedupe: ['d3-scale', 'd3-array']
		},
		optimizeDeps: {
			include: ['d3-scale', 'd3-array']
		},
		server: {
			proxy: {
				'/api': {
					target: 'http://localhost:8082',
					changeOrigin: true
					// rewrite: (path) => path.replace(/^\/api/, '')
				}
			}
		}
	};
});
