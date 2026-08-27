import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
	plugins: [svelte({ hot: false })],
	// These tests run on plain svelte(), not sveltekit(), so nothing supplies the
	// $app modules. $app/paths points at SvelteKit's real implementation rather
	// than a stub because an identity stub is what kept the suite blind to #325,
	// and would keep it blind to the next resolve() misuse. Nothing under test
	// calls resolve() today. These are the globals that module reads.
	define: {
		__SVELTEKIT_PAYLOAD__: 'undefined',
		__SVELTEKIT_PATHS_BASE__: '""',
		__SVELTEKIT_APP_DIR__: '"_app"',
		__SVELTEKIT_HASH_ROUTING__: 'false'
	},
	resolve: {
		conditions: ['browser'],
		alias: {
			$lib: path.resolve('./src/lib'),
			'$app/paths': path.resolve('./node_modules/@sveltejs/kit/src/runtime/app/paths/client.js'),
			'$app/navigation': path.resolve('./src/test/mocks/app-navigation.ts')
		}
	},
	test: {
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
