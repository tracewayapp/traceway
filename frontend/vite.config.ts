import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
    plugins: [tailwindcss(), sveltekit()],
    server: {
        proxy: {
            '/api': {
                target: 'http://localhost:8082',
                changeOrigin: true,
                // rewrite: (path) => path.replace(/^\/api/, '')
            }
        }
    }
});
