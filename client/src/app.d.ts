// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		interface PageState {
			// Days a new cycle left as they were, for the cycle page to say so.
			leftOut?: number;
			// When an exercise was last saved beside the Library's list, so its page
			// starts again from what was saved.
			saved?: number;
		}
		// interface Platform {}
	}
}

export {};
