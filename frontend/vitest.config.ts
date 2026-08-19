import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';
import path from 'path';

export default defineConfig({
	plugins: [svelte({ hot: false })],
	resolve: {
		conditions: ['browser'],
		alias: {
			$lib: path.resolve('./src/lib'),
			'$app/paths': path.resolve('./src/test/mocks/app-paths.ts')
		}
	},
	test: {
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
