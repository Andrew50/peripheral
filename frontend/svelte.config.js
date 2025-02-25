/*import adapter from '@sveltejs/adapter-auto';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
// @type {import('@sveltejs/kit').Config}
const config = {
	// Consult https://kit.svelte.dev/docs/integrations#preprocessors
	// for more information about preprocessors
	preprocess: vitePreprocess(),

	kit: {
		// adapter-auto only supports some environments, see https://kit.svelte.dev/docs/adapter-auto for a list.
		// If your environment is not supported, or you settled on a specific environment, switch out the adapter.
		// See https://kit.svelte.dev/docs/adapters for more information about adapters.
		adapter: adapter()
	}
};*/
import adapter from '@sveltejs/adapter-node';

import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
export default {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter()
	}
};

/*import adapter from '@sveltejs/adapter-node'; // For production
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

// @type {import('@sveltejs/kit').Config}
const config = {
	preprocess: vitePreprocess(),

	kit: {
		adapter: adapter(), // Only used in production builds
		vite: {
			// Only applied in development
			server: {
				watch: {
					usePolling: true, // Enable polling if using Docker
				},
				hmr: {
					// Configure hot module reloading
					clientPort: process.env.HMR_HOST || 5173,
				},
			},
		},
	}
};

export default config;
*/
