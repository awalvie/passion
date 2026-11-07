// Timer tones. iOS plays Web Audio only after a tap starts the context, and
// suspends it when the app goes to the background, so it is resumed on the
// next tap and on return.
let ctx: AudioContext | null = null;
let listening = false;

type Session = { type: string; state?: string; addEventListener?: (name: string, fn: () => void) => void };
const session = () => (navigator as Navigator & { audioSession?: Session }).audioSession;

// A log of what the audio does on the phone, read in Profile, to find why the
// tones stop. Temporary.
const logKey = 'passion-audio-log';
export function trace(what: string) {
	try {
		const s = session();
		const line = `${new Date().toTimeString().slice(0, 8)} ${what} · context ${ctx?.state ?? 'none'} · session ${s ? `${s.type} ${s.state ?? ''}` : 'none'} · ${document.visibilityState}`;
		const log = audioLog();
		log.push(line);
		localStorage.setItem(logKey, JSON.stringify(log.slice(-300)));
	} catch {
		// The log is only a help.
	}
}
export function audioLog(): string[] {
	try {
		return JSON.parse(localStorage.getItem(logKey) ?? '[]');
	} catch {
		return [];
	}
}
export const clearAudioLog = () => localStorage.removeItem(logKey);

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
	trace('tap');
	// Playback sounds on silent, as a timer on the wall must; it stops music
	// from other apps. Ambient played over music but the silent switch muted
	// it. It is set once, before the context starts: set again on each tone,
	// iOS went quiet.
	const s = session();
	if (s && s.type !== 'playback') s.type = 'playback';
	// After an interruption, such as a call or another app's sound, iOS often
	// will not resume the old context, so a tap makes a new one.
	if (ctx && (ctx.state !== 'running' || s?.state === 'interrupted')) {
		void ctx.close();
		ctx = null;
	}
	if (!ctx) {
		const made = new AudioContext();
		made.onstatechange = () => trace(`context now ${made.state}`);
		ctx = made;
		trace('new context');
	}
	if (ctx.state !== 'running') {
		// WebKit has needed a sound played inside the tap, even a silent one,
		// before it lets the audio start.
		const quiet = ctx.createBufferSource();
		quiet.buffer = ctx.createBuffer(1, 1, ctx.sampleRate);
		quiet.connect(ctx.destination);
		quiet.start();
	}
	start(ctx);
}

function resume() {
	if (ctx && ctx.state !== 'running') start(ctx);
}

function start(c: AudioContext) {
	c.resume().then(
		() => trace('resume done'),
		(e: Error) => trace(`resume refused: ${e.name}`)
	);
}

function onShow() {
	trace(document.visibilityState === 'visible' ? 'app shown' : 'app hidden');
	if (document.visibilityState === 'visible') resume();
}

// listen lets any tap unlock until the returned function runs, so a clock
// that started by itself can still sound. Only a run listens, so a tap
// elsewhere in the app leaves the audio alone.
// A browser counts a touch as a tap only as it lifts, and a mouse as it presses.
export function listen(): () => void {
	const touch = (e: PointerEvent) => e.pointerType !== 'mouse' && unlock();
	const mouse = (e: PointerEvent) => e.pointerType === 'mouse' && unlock();
	listening = true;
	trace('run opened');
	session()?.addEventListener?.('statechange', () => trace('session changed'));
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
	trace('run left');
	void ctx?.close();
	ctx = null;
	const s = session();
	if (s?.type === 'playback') s.type = 'auto';
}

export function tone(hz: number, ms: number) {
	trace(`tone ${hz}`);
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

