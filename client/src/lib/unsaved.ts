import { beforeNavigate } from '$app/navigation';

// It must run while the component starts. A reload or a closed tab can only get
// the browser's own prompt.
export function guardUnsaved(changed: () => boolean) {
	beforeNavigate((navigation) => {
		if (!changed()) return;
		if (navigation.type === 'leave') navigation.cancel();
		else if (!confirm('Discard your unsaved changes?')) navigation.cancel();
	});
}
