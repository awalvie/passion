import { loadExercises } from '$lib/exercise';

export async function load({ url }) {
	// Reading the path loads the list again on every page, so a save or a retire
	// shows in it. The answer is kept until a change, so a pick asks nothing new.
	void url.pathname;
	return { exercises: await loadExercises() };
}
