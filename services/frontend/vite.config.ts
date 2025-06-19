import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	ssr: {
		noExternal: ['svelte-plotly.js', 'plotly.js-dist']
	},
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
	resolve: {
		dedupe: ['svelte']
	},
	optimizeDeps: {
		include: ['plotly.js-dist', 'svelte-plotly.js']
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
