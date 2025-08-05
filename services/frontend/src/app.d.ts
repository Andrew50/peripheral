// See https://kit.svelte.dev/docs/types#app
// for information about these interfaces
// and what to do when importing types
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface Platform {}
	}
}

export {};

// Add missing module declarations for SvelteKit
declare module '$app/environment' {
	export const browser: boolean;
	export const dev: boolean;
	export const building: boolean;
	export const version: string;
}

declare module '$app/navigation' {
	export function goto(
		url: string | URL,
		opts?: {
			replaceState?: boolean;
			noScroll?: boolean;
			keepFocus?: boolean;
			invalidateAll?: boolean;
		}
	): Promise<void>;
	export function invalidate(url: string | URL): Promise<void>;
	export function invalidateAll(): Promise<void>;
	export function prefetch(url: string | URL): Promise<void>;
	export function prefetchRoutes(urls?: string[]): Promise<void>;
}
