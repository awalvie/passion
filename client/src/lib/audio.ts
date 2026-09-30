// Timer tones. iOS plays Web Audio only after a tap starts the context, and
// suspends it when the app goes to the background, so it is resumed on the
// next tap and on return.
let ctx: AudioContext | null = null;

const soundKey = 'passion-sound';

export function soundOn(): boolean {
	return localStorage.getItem(soundKey) !== 'off';
}

export function setSound(on: boolean) {
	if (on) localStorage.removeItem(soundKey);
	else localStorage.setItem(soundKey, 'off');
}

// unlock must run inside a tap.
export function unlock() {
	// Plays through the silent switch, like a timer app, where Safari offers it.
	const session = (navigator as Navigator & { audioSession?: { type: string } }).audioSession;
	if (session) session.type = 'playback';
	ctx ??= new AudioContext();
	void ctx.resume();
}

function resume() {
	if (ctx && ctx.state !== 'running') void ctx.resume();
}

addEventListener('pointerdown', resume);
document.addEventListener('visibilitychange', () => {
	if (document.visibilityState === 'visible') resume();
});

export function tone(hz: number, ms: number) {
	if (!ctx || !soundOn()) return;
	const osc = ctx.createOscillator();
	const gain = ctx.createGain();
	const t = ctx.currentTime;
	osc.frequency.value = hz;
	gain.gain.setValueAtTime(0.0001, t);
	gain.gain.exponentialRampToValueAtTime(0.4, t + 0.01);
	gain.gain.exponentialRampToValueAtTime(0.0001, t + ms / 1000);
	osc.connect(gain).connect(ctx.destination);
	osc.start(t);
	osc.stop(t + ms / 1000 + 0.05);
}
