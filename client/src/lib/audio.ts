// Timer tones. iOS plays Web Audio only after a tap starts the context, and
// suspends it when the app goes to the background, so it is resumed on the
// next tap and on return.
let ctx: AudioContext | null = null;

const soundKey = 'passion-sound';

export function soundOn(): boolean {
	return localStorage.getItem(soundKey) !== 'off';
}

// setSound must run inside a tap, so switching on can unlock.
export function setSound(on: boolean) {
	if (on) localStorage.removeItem(soundKey);
	else localStorage.setItem(soundKey, 'off');
	unlock();
}

// unlock must run inside a tap. Playback pauses music from other apps, so
// with the sound off it is left alone.
export function unlock() {
	if (!soundOn()) return;
	// Playback sounds on silent, as a timer on the wall must. It is set once,
	// before the context starts: set again on each tone, iOS went quiet.
	const session = (navigator as Navigator & { audioSession?: { type: string } }).audioSession;
	if (session && session.type !== 'playback') session.type = 'playback';
	ctx ??= new AudioContext();
	if (ctx.state !== 'running') {
		// WebKit has needed a sound played inside the tap, even a silent one,
		// before it lets the audio start.
		const quiet = ctx.createBufferSource();
		quiet.buffer = ctx.createBuffer(1, 1, ctx.sampleRate);
		quiet.connect(ctx.destination);
		quiet.start();
	}
	void ctx.resume();
}

function resume() {
	if (ctx && ctx.state !== 'running') void ctx.resume();
}

// Any tap unlocks, so a clock that started by itself can still sound. A
// browser counts a touch as a tap only as it lifts, and a mouse as it presses.
addEventListener('pointerup', (e) => e.pointerType !== 'mouse' && unlock());
addEventListener('pointerdown', (e) => e.pointerType === 'mouse' && unlock());
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
