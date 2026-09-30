// The timer for a timed-reps step, as pure functions of the clock. Nothing
// counts down: the phase is worked out from the time since the start, so a
// locked phone or a reload loses nothing.

export type TimedStep = {
	sets: number | null;
	reps: number | null;
	rep_seconds: number | null;
	rep_rest_seconds: number | null;
	set_rest_seconds: number | null;
	prep_seconds: number | null;
	per_side: boolean;
};

// A block is one side of one set. Each block logs one set row.
export type Phase = {
	kind: 'prep' | 'hang' | 'rest';
	block: number;
	set: number;
	side: 'Left' | 'Right' | null;
	rep: number;
	ms: number;
};

export type Clock = { startedAt: number; pausedAt: number | null; pausedMs: number; skipMs: number };

export function canTime(s: TimedStep): boolean {
	return Boolean(s.rep_seconds && s.reps && s.sets);
}

export function timeline(s: TimedStep): Phase[] {
	const sides = s.per_side ? (['Left', 'Right'] as const) : ([null] as const);
	const reps = s.reps ?? 1;
	const out: Phase[] = [];
	const push = (p: Phase) => p.ms > 0 && out.push(p);
	let block = 0;
	push({ kind: 'prep', block: 0, set: 1, side: sides[0], rep: 1, ms: (s.prep_seconds ?? 0) * 1000 });
	for (let set = 1; set <= (s.sets ?? 1); set++) {
		for (const [i, side] of sides.entries()) {
			for (let rep = 1; rep <= reps; rep++) {
				push({ kind: 'hang', block, set, side, rep, ms: (s.rep_seconds ?? 0) * 1000 });
				const last = rep === reps;
				const lastSide = i === sides.length - 1;
				const rest = !last ? s.rep_rest_seconds : !lastSide ? s.rep_rest_seconds : s.set_rest_seconds;
				if (!(last && lastSide && set === s.sets)) push({ kind: 'rest', block, set, side, rep, ms: (rest ?? 0) * 1000 });
			}
			block++;
		}
	}
	return out;
}

export function newClock(now: number): Clock {
	return { startedAt: now, pausedAt: null, pausedMs: 0, skipMs: 0 };
}

export function togglePause(c: Clock, now: number): Clock {
	if (c.pausedAt === null) return { ...c, pausedAt: now };
	return { ...c, pausedAt: null, pausedMs: c.pausedMs + now - c.pausedAt };
}

export function elapsed(c: Clock, now: number): number {
	return (c.pausedAt ?? now) - c.startedAt - c.pausedMs + c.skipMs;
}

// at finds the phase running at a time. index is phases.length once it is over.
export function at(phases: Phase[], ms: number): { index: number; left: number } {
	let end = 0;
	for (const [index, p] of phases.entries()) {
		end += p.ms;
		if (ms < end) return { index, left: end - ms };
	}
	return { index: phases.length, left: 0 };
}

export function startOf(phases: Phase[], index: number): number {
	return phases.slice(0, index).reduce((sum, p) => sum + p.ms, 0);
}

// jump moves the clock so it reads a later time.
export function jump(c: Clock, now: number, to: number): Clock {
	return { ...c, skipMs: c.skipMs + Math.max(0, to - elapsed(c, now)) };
}

// endSet cuts the current block short: its reps stop at the hangs done, and
// the clock moves to the rest after the block, or to the end.
export function endSet(phases: Phase[], c: Clock, now: number, short: Record<number, number>) {
	const ms = elapsed(c, now);
	const { index } = at(phases, ms);
	if (phases[index]?.kind === 'prep') return { clock: c, short };
	const block = phases[index]?.block ?? 0;
	const lastHang = phases.findLastIndex((p) => p.block === block && p.kind === 'hang');
	return {
		clock: jump(c, now, startOf(phases, lastHang + 1)),
		short: { ...short, [block]: hangsDone(phases, ms, block) }
	};
}

// hangsDone counts a block's hangs that ended by a time.
function hangsDone(phases: Phase[], ms: number, block: number): number {
	let end = 0;
	let n = 0;
	for (const p of phases) {
		end += p.ms;
		if (p.block === block && p.kind === 'hang' && end <= ms) n++;
	}
	return n;
}

// rows are the reps each block did by a time: every block whose hangs are
// over, and with partial the block in progress too.
export function rows(phases: Phase[], ms: number, short: Record<number, number>, partial = false): number[] {
	const blocks = new Set(phases.filter((p) => p.kind === 'hang').map((p) => p.block));
	const out: number[] = [];
	for (const block of blocks) {
		const lastHang = phases.findLastIndex((p) => p.block === block && p.kind === 'hang');
		const over = block in short || ms >= startOf(phases, lastHang + 1);
		const done = short[block] ?? hangsDone(phases, ms, block);
		if (over) out.push(done);
		else if (partial && done) out.push(done);
		if (!over) break;
	}
	return out;
}
