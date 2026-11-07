// Timer tones. iOS starts Web Audio only in a tap, so the first tap of a run
// makes the context. The run keeps that one context: once a tap has started
// it, WebKit lets it resume without a tap, which a new context cannot do.
let ctx: AudioContext | null = null;
let listening = false;

type Session = { type: string };
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

// unlock must run inside a tap. With the sound off it leaves the audio alone.
export function unlock() {
	if (!soundOn()) return;
	// Playback sounds on silent, as a timer on the wall must; it stops music
	// from other apps. Ambient played over music but the silent switch muted
	// it. It is set once, before the context starts: set again on each tone,
	// iOS went quiet.
	const s = session();
	if (s && s.type !== 'playback') s.type = 'playback';
	if (!ctx) {
		ctx = new AudioContext();
		// WebKit has needed a sound played inside the tap, even a silent one,
		// before it lets the audio start.
		const quiet = ctx.createBufferSource();
		quiet.buffer = ctx.createBuffer(1, 1, ctx.sampleRate);
		quiet.connect(ctx.destination);
		quiet.start();
	}
	if (ctx.state !== 'running') void ctx.resume();
}

// With playback, WebKit lets the context run on as the app hides, and iOS
// may stop its output while the app sleeps. On return the context still says
// running but plays nothing, and resume() on a running context does nothing.
// So the context is suspended as the app hides and resumed on return, and a
// context that still says running is suspended first.
function onShow() {
	if (!ctx) return;
	const c = ctx;
	if (document.visibilityState === 'hidden') {
		if (c.state === 'running') void c.suspend();
	} else if (c.state === 'running') void c.suspend().then(() => c.resume());
	else void c.resume();
}

// listen lets any tap unlock until the returned function runs, so a clock
// that started by itself can still sound. Only a run listens, so a tap
// elsewhere in the app leaves the audio alone.
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

// release closes the tones' audio and hands the session back.
function release() {
	void ctx?.close();
	ctx = null;
	const s = session();
	if (s?.type === 'playback') s.type = 'auto';
}

export function tone(hz: number, ms: number) {
	// A tone made while the app is hidden would wait in the suspended context
	// and sound late, on return.
	if (!ctx || !soundOn() || document.visibilityState === 'hidden') return;
	if (ctx.state !== 'running') void ctx.resume();
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
