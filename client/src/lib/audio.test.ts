import { beforeEach, expect, test, vi } from 'vitest';

// Items live as own properties, as in a browser.
class MemoryStorage {
	[k: string]: unknown;
	getItem(k: string): string | null {
		return Object.hasOwn(this, k) ? String(this[k]) : null;
	}
	setItem(k: string, v: string) {
		this[k] = String(v);
	}
	removeItem(k: string) {
		delete this[k];
	}
}

let log: string[];
let listeners: Record<string, (e?: { pointerType: string }) => void>;
let oscillators: { frequency: { value: number }; started: boolean }[];
let contexts: FakeAudioContext[];

class FakeAudioContext {
	state = 'suspended';
	currentTime = 0;
	destination = {};
	sampleRate = 44100;
	constructor() {
		log.push('context');
		contexts.push(this);
	}
	resume() {
		log.push('resume');
		this.state = 'running';
		return Promise.resolve();
	}
	createOscillator() {
		const o = { frequency: { value: 0 }, started: false, connect: (n: unknown) => n, start() { o.started = true; }, stop() {} };
		oscillators.push(o);
		return o;
	}
	createBuffer() {
		return {};
	}
	createBufferSource() {
		return { buffer: null, connect: (n: unknown) => n, start: () => log.push('quiet') };
	}
	createGain() {
		return { gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} }, connect: (n: unknown) => n };
	}
}

beforeEach(() => {
	vi.unstubAllGlobals();
	log = [];
	listeners = {};
	oscillators = [];
	contexts = [];
	vi.stubGlobal('localStorage', new MemoryStorage());
	vi.stubGlobal('addEventListener', (name: string, fn: (e?: { pointerType: string }) => void) => (listeners[name] = fn));
	vi.stubGlobal('document', { visibilityState: 'visible', addEventListener: (name: string, fn: () => void) => (listeners[`document:${name}`] = fn) });
	vi.stubGlobal('AudioContext', FakeAudioContext);
	let type = 'auto';
	vi.stubGlobal('navigator', {
		audioSession: {
			get type() {
				return type;
			},
			set type(v: string) {
				log.push(`session ${v}`);
				type = v;
			}
		}
	});
});

const tap = () => listeners.pointerup({ pointerType: 'touch' });

// A fresh import is a reload: no audio context yet.
async function load() {
	vi.resetModules();
	return import('./audio');
}

test('sound is on until it is switched off, and the choice is kept', async () => {
	const { setSound, soundOn } = await load();
	expect(soundOn()).toBe(true);
	setSound(false);
	expect(soundOn()).toBe(false);
	expect(localStorage.getItem('passion-sound')).toBe('off');
	setSound(true);
	expect(soundOn()).toBe(true);
	expect(localStorage.getItem('passion-sound')).toBeNull();
});

test('the first tap sets playback before the audio context starts, and does it once', async () => {
	await load();
	tap();
	tap();
	expect(log).toEqual(['session playback', 'context', 'quiet', 'resume', 'resume']);
});

// A touch counts as a tap for audio only as it lifts; a mouse as it presses.
test('a touch unlocks as it lifts, and a mouse as it presses', async () => {
	await load();
	listeners.pointerdown({ pointerType: 'touch' });
	listeners.pointerup({ pointerType: 'mouse' });
	expect(contexts).toHaveLength(0);
	listeners.pointerdown({ pointerType: 'mouse' });
	expect(contexts).toHaveLength(1);
});

test('a tone plays only after a tap has unlocked the audio', async () => {
	const { tone } = await load();
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
	tap();
	tone(880, 100);
	expect(oscillators).toHaveLength(1);
	expect(oscillators[0].frequency.value).toBe(880);
	expect(oscillators[0].started).toBe(true);
});

test('a tone is silent while sound is off', async () => {
	const { setSound, tone } = await load();
	tap();
	setSound(false);
	tone(880, 100);
	expect(oscillators).toHaveLength(0);
});

test('coming back to the app resumes a suspended context, and leaves a running one alone', async () => {
	await load();
	tap();
	log.length = 0;
	listeners['document:visibilitychange']();
	expect(log).toEqual([]);
	contexts[0].state = 'suspended';
	listeners['document:visibilitychange']();
	expect(log).toEqual(['resume']);
});

// Playback stops music from other apps. With the sound off no tone will ever
// play, so a tap anywhere in the app should not take the audio session.
test('a tap does not take the audio session while sound is off', async () => {
	const { setSound } = await load();
	setSound(false);
	tap();
	expect(log).not.toContain('session playback');
});

// The tap on the switch comes before the switch turns the sound on, so the
// switch unlocks the audio itself.
test('switching the sound back on unlocks the audio', async () => {
	const { setSound, tone } = await load();
	setSound(false);
	tap();
	setSound(true);
	tone(880, 100);
	expect(log).toEqual(['session playback', 'context', 'quiet', 'resume']);
	expect(oscillators).toHaveLength(1);
});
