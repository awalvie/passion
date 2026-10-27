import { afterNavigate, beforeNavigate } from '$app/navigation';

// It must run while the component starts. A reload or a closed tab can only get
// the browser's own prompt.
export function guardUnsaved(changed: () => boolean) {
	// A redirect asks again for the same move, so a yes holds until it lands.
	let agreed = false;
	beforeNavigate((navigation) => {
		if (agreed || !changed()) return;
		if (navigation.type === 'leave') navigation.cancel();
		else if (confirm('Discard your unsaved changes?')) agreed = true;
		else navigation.cancel();
	});
	afterNavigate(() => (agreed = false));
}
