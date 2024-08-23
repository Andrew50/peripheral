import adapter from '@sveltejs/adapter-auto';
import sveltePreprocess from 'svelte-preprocess';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: sveltePreprocess({
    typescript: true,  // Enable TypeScript preprocessing
//    scss: true,  // Enable SCSS (if you need SCSS support)
    // Add other preprocessors as needed
  }),

  kit: {
    adapter: adapter(),  // Use the appropriate adapter for your environment
  },
};

export default config;

