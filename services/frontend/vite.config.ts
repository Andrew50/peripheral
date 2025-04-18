import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		fs: {
			// Allow serving files from the entire project
			allow: ['..']
		},
        watch: {
            ignored: ['**/node_modules/**', '**/.git/**', '**/.svelte-kit/**'],
            usePolling: true,
            interval: 500
        }
    

	},
	build: {
		// Ensure proper MIME types for JavaScript modules
		rollupOptions: {
			output: {
				manualChunks: undefined
			}
		}
	}
});
