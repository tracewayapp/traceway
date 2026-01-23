import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { readFileSync, existsSync } from 'fs';
import path from 'path';

const pkg = JSON.parse(readFileSync('./package.json', 'utf-8'));

const billingPath = process.env.BILLING_PATH;
const resolvedBillingPath = path.resolve(import.meta.dirname, billingPath);
const billingExists = existsSync(resolvedBillingPath);

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), '');

	return {
		plugins: [tailwindcss(), sveltekit()],
		define: {
			__APP_VERSION__: JSON.stringify(env.PUBLIC_APP_VERSION || pkg.version),
			__CLOUD_MODE__: env.CLOUD_MODE,
			__BILLING_AVAILABLE__: billingExists
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
