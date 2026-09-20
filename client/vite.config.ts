import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// The Go binary embeds this directory, and go:embed cannot reach above
			// its own package, so the build has to land inside server/web.
			adapter: adapter({
				pages: '../server/web/dist',
				assets: '../server/web/dist',
				fallback: 'index.html'
			})
		})
	],

	// The dev server has no Go behind it, so the API is proxied to one started
	// separately by `make run`.
	server: {
		proxy: {
			'/api': 'http://localhost:8080'
		}
	}
});
