// Timer tones. iOS plays Web Audio only after a tap starts the context, and
// suspends it when the app goes to the background, so it is resumed on the
// next tap and on return.
let ctx: AudioContext | null = null;
let listening = false;

type Session = { type: string; state?: string };
const session = () => (navigator as Navigator & { audioSession?: Session }).audioSession;

const soundKey = 'passion-sound';

export function soundOn(): boolean {
	return localStorage.getItem(soundKey) !== 'off';
}

// setSound must run inside a tap, so switching on can unlock. Switching on
// beeps, so the person knows the phone will sound.
export function setSound(on: boolean) {
	if (on) localStorage.removeItem(soundKey);
	else localStorage.setItem(soundKey, 'off');
	unlock();
	if (on) tone(1046, 200);
	// Outside a run nothing else will sound, so the beep hands the audio back.
	if (!listening) setTimeout(release, 400);
}

// unlock must run inside a tap. Playback pauses music from other apps, so
// with the sound off it is left alone.
export function unlock() {
	if (!soundOn()) return;
	// Playback sounds on silent, as a timer on the wall must. It is set once,
	// before the context starts: set again on each tone, iOS went quiet.
	const s = session();
	if (s && s.type !== 'playback') s.type = 'playback';
	// After an interruption, such as a call or another app's sound, iOS often
	// will not resume the old context, so a tap makes a new one.
	if (ctx && (ctx.state !== 'running' || s?.state === 'interrupted')) {
		void ctx.close();
		ctx = null;
	}
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

function onShow() {
	if (document.visibilityState === 'visible') resume();
}

// listen lets any tap unlock until the returned function runs, so a clock
// that started by itself can still sound. Only a run listens: playback stops
// music from other apps, so a tap elsewhere in the app must leave it alone.
// A browser counts a touch as a tap only as it lifts, and a mouse as it presses.
export function listen(): () => void {
	const touch = (e: PointerEvent) => e.pointerType !== 'mouse' && unlock();
	const mouse = (e: PointerEvent) => e.pointerType === 'mouse' && unlock();
	listening = true;
	addEventListener('pointerup', touch);
	addEventListener('pointerdown', mouse);
	document.addEventListener('visibilitychange', onShow);
	return () => {
		listening = false;
		removeEventListener('pointerup', touch);
		removeEventListener('pointerdown', mouse);
		document.removeEventListener('visibilitychange', onShow);
		release();
	};
}

// release hands the audio back, so music from other apps can play again.
function release() {
	void ctx?.close();
	ctx = null;
	const s = session();
	if (s?.type === 'playback') s.type = 'auto';
}

export function tone(hz: number, ms: number) {
	if (!ctx || !soundOn()) return;
	resume();
	const osc = ctx.createOscillator();
	const gain = ctx.createGain();
	const t = ctx.currentTime;
	// A sine was too quiet on a phone speaker. A square wave at full gain is as
	// loud as Web Audio plays without clipping.
	osc.type = 'square';
	osc.frequency.value = hz;
	gain.gain.setValueAtTime(0.0001, t);
	gain.gain.exponentialRampToValueAtTime(1, t + 0.01);
	gain.gain.exponentialRampToValueAtTime(0.0001, t + ms / 1000);
	osc.connect(gain).connect(ctx.destination);
	osc.start(t);
	osc.stop(t + ms / 1000 + 0.05);
}
