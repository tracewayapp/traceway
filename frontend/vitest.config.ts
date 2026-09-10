import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
	plugins: [svelte({ hot: false })],
	// Plain svelte(), not sveltekit(), supplies no $app modules. $app/paths is SvelteKit's real
	// implementation rather than an identity stub, which is what kept the suite blind to #325.
	define: {
		__SVELTEKIT_PAYLOAD__: 'undefined',
		__SVELTEKIT_PATHS_BASE__: '""',
		__SVELTEKIT_PATHS_ASSETS__: '""',
		__SVELTEKIT_APP_DIR__: '"_app"',
		__SVELTEKIT_HASH_ROUTING__: 'false'
	},
	resolve: {
		conditions: ['browser'],
		alias: {
			'$app/environment': path.resolve(
				'./node_modules/@sveltejs/kit/src/runtime/app/environment/index.js'
			),
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
