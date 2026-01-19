import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'fs';

const pkg = JSON.parse(readFileSync('./package.json', 'utf-8'));

export default defineConfig({
    plugins: [tailwindcss(), sveltekit()],
    define: {
        __APP_VERSION__: JSON.stringify(process.env.PUBLIC_APP_VERSION || pkg.version)
    },
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
