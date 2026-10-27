import type { ScheduledDay } from './plan';

type Options = {
	movable: (d: ScheduledDay) => boolean;
	target: (date: string, d: ScheduledDay) => boolean;
	onlift?: () => void;
	ondrop: (d: ScheduledDay, to: string) => void;
};

// A hold on a planned session lifts it; it then follows the finger, and a
// drop on a day that target allows moves it there. A move before the hold
// ends is a scroll, so the hold gives up. A mouse cannot scroll by dragging,
// so it lifts the session as soon as it moves. Each day needs data-date.
export class SessionDrag {
	lifted = $state<ScheduledDay | null>(null);
	at = $state({ x: 0, y: 0 });
	over = $state('');
	#hold: ReturnType<typeof setTimeout> | undefined;
	#start = { x: 0, y: 0 };
	#pressed: { d: ScheduledDay; el: Element; id: number; mouse: boolean } | null = null;
	// The click that ends a drag lands on the day it started from, sometimes
	// well after the drop; it must not choose that day.
	#droppedAt = -Infinity;
	#o: Options;

	constructor(o: Options) {
		this.#o = o;
	}

	get justDropped() {
		return performance.now() - this.#droppedAt < 500;
	}

	// Sessions keep touch scrolling, so a swipe that starts on one still
	// scrolls. Once one is lifted, the page must not scroll under the finger.
	// A session that leaves the page mid-drag never hears the release, so any
	// release, or Escape, lets it go. Call from an $effect.
	listen() {
		const stop = (e: TouchEvent) => this.lifted && e.preventDefault();
		const end = () => (this.lifted || this.#pressed) && this.cancel();
		const key = (e: KeyboardEvent) => e.key === 'Escape' && end();
		window.addEventListener('touchmove', stop, { passive: false });
		window.addEventListener('pointerup', end);
		window.addEventListener('pointercancel', end);
		window.addEventListener('keydown', key);
		return () => {
			window.removeEventListener('touchmove', stop);
			window.removeEventListener('pointerup', end);
			window.removeEventListener('pointercancel', end);
			window.removeEventListener('keydown', key);
		};
	}

	grab(e: PointerEvent, d: ScheduledDay) {
		if (e.button > 0 || !this.#o.movable(d)) return;
		this.#pressed = { d, el: e.currentTarget as Element, id: e.pointerId, mouse: e.pointerType === 'mouse' };
		this.#start = { x: e.clientX, y: e.clientY };
		this.at = this.#start;
		clearTimeout(this.#hold);
		// A mouse that leaves the session before it moves far enough must still
		// let go here.
		if (this.#pressed.mouse) this.#pressed.el.setPointerCapture(this.#pressed.id);
		else this.#hold = setTimeout(() => this.#lift(), 300);
	}

	#lift() {
		if (!this.#pressed) return;
		this.lifted = this.#pressed.d;
		this.over = '';
		this.#o.onlift?.();
		this.#pressed.el.setPointerCapture(this.#pressed.id);
		navigator.vibrate?.(10);
	}

	follow(e: PointerEvent) {
		if (!this.lifted) {
			const moved = Math.hypot(e.clientX - this.#start.x, e.clientY - this.#start.y);
			if (this.#pressed?.mouse && moved > 4) this.#lift();
			else if (moved > 8) clearTimeout(this.#hold);
			if (!this.lifted) return;
		}
		this.at = { x: e.clientX, y: e.clientY };
		const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>('[data-date]');
		const date = cell?.dataset.date ?? '';
		this.over = this.target(date) ? date : '';
	}

	target(date: string) {
		return !!this.lifted && date !== this.lifted.local_date && this.#o.target(date, this.lifted);
	}

	drop() {
		clearTimeout(this.#hold);
		this.#pressed = null;
		const d = this.lifted;
		const to = this.over;
		this.lifted = null;
		this.over = '';
		if (d) this.#droppedAt = performance.now();
		if (d && to) this.#o.ondrop(d, to);
	}

	cancel() {
		clearTimeout(this.#hold);
		this.#pressed = null;
		this.lifted = null;
		this.over = '';
	}
}
