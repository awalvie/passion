// keepAwake holds the screen on until the returned function runs. The browser
// drops the lock when the app is hidden, so it is taken again on return. It
// needs a secure context, and a refusal only means the screen may dim.
export function keepAwake(): () => void {
	let lock: WakeLockSentinel | null = null;
	let held = true;

	async function take() {
		if (!held || document.visibilityState !== 'visible' || !('wakeLock' in navigator)) return;
		try {
			lock = await navigator.wakeLock.request('screen');
			if (!held) void lock.release();
		} catch {
			lock = null;
		}
	}

	document.addEventListener('visibilitychange', take);
	void take();

	return () => {
		held = false;
		document.removeEventListener('visibilitychange', take);
		void lock?.release();
	};
}
