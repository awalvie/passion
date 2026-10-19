<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	// onback puts a back button before the title, for a page opened inside the
	// sheet. footer stays pinned to the bottom while the rest scrolls.
	let {
		open = $bindable(false),
		title,
		eyebrow,
		onback,
		footer,
		children
	}: { open?: boolean; title: string; eyebrow?: string; onback?: () => void; footer?: Snippet; children: Snippet } = $props();

	let dialog: HTMLDialogElement;
	let hidden = $state(true);
	let closing: ReturnType<typeof setTimeout> | undefined;

	// The sheet starts below the screen and slides up once it is shown, and
	// leaves the top layer only after it slid down again.
	$effect(() => {
		if (open) {
			clearTimeout(closing);
			if (!dialog.open) {
				dialog.showModal();
				void dialog.offsetHeight;
			}
			hidden = false;
		} else if (dialog.open) {
			hidden = true;
			closing = setTimeout(() => dialog.close(), 250);
		}
	});

	// A drag down from the top area moves the sheet with the finger. Past a
	// quarter of its height it closes; short of that it springs back.
	let drag = $state<number | null>(null);
	// A swipe that closes keeps its inline transform, so the slide down starts
	// where the finger let go instead of jumping back up first.
	let flung = $state(false);
	let startY = 0;

	function grab(e: PointerEvent) {
		if ((e.target as Element).closest('button')) return;
		startY = e.clientY;
		drag = 0;
		(e.currentTarget as Element).setPointerCapture(e.pointerId);
	}

	// iOS leaves a field focused when the tap or swipe that closes the sheet
	// lands outside it, so blur first and let its change handler save.
	function close() {
		(document.activeElement as HTMLElement | null)?.blur();
		open = false;
	}

	function release() {
		if (drag === null) return;
		if (drag > Math.min(120, dialog.offsetHeight / 4)) {
			flung = true;
			close();
		} else drag = null;
	}
</script>

<!-- A tap on the scrim lands on the dialog itself; the sheet fills it otherwise. -->
<dialog
	bind:this={dialog}
	aria-label={title}
	data-hidden={hidden || undefined}
	style:transform={drag === null ? undefined : flung ? 'translateY(100%)' : `translateY(${drag}px)`}
	style:transition={drag === null || flung ? undefined : 'none'}
	class="fixed inset-x-0 top-auto bottom-0 m-0 mx-auto max-h-[calc(100dvh-3rem)] w-full max-w-[430px] overflow-y-auto overscroll-contain [scrollbar-width:none] rounded-t-[32px] bg-ground text-ink shadow-[0_-20px_40px_-10px_rgba(0,0,0,0.35)] backdrop:bg-[var(--scrim)]"
	onclose={() => {
		open = false;
		drag = null;
		flung = false;
	}}
	oncancel={(e) => {
		e.preventDefault();
		close();
	}}
	onclick={(e) => {
		if (e.target === dialog) close();
	}}
>
	<div class="px-4 pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
		<div
			class="touch-none pt-2.5"
			role="presentation"
			onpointerdown={grab}
			onpointermove={(e) => {
				if (drag !== null) drag = Math.max(0, e.clientY - startY);
			}}
			onpointerup={release}
			onpointercancel={() => (drag = null)}
		>
			<div class="mx-auto mb-3.5 h-[5px] w-10 rounded-full bg-ink-3/50"></div>
			<header class="flex items-start justify-between gap-3 px-1 pb-3">
				{#if onback}
					<button
						type="button"
						class="flex size-11 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
						aria-label="Back"
						onclick={onback}
					>
						<Icon name="chevron-left" size="1.25rem" />
					</button>
				{/if}
				<div class="min-w-0 flex-1">
					{#if eyebrow}<p class="text-xs font-semibold text-ink-3">{eyebrow}</p>{/if}
					<h2 class="text-[32px] leading-[1.1] font-extrabold tracking-tight break-words">{title}</h2>
				</div>
				<button
					type="button"
					class="flex size-11 shrink-0 items-center justify-center rounded-full bg-surface text-ink shadow-card-sm"
					aria-label="Close"
					onclick={close}
				>
					<Icon name="x" size="1.25rem" />
				</button>
			</header>
		</div>
		{@render children()}
		{#if footer}
			<div class="sticky bottom-0 -mx-4 mt-4 -mb-[calc(1.5rem+env(safe-area-inset-bottom))] bg-ground px-4 pt-3 pb-[calc(1.5rem+env(safe-area-inset-bottom))] shadow-[0_-1px_0_var(--line)]">
				{@render footer()}
			</div>
		{/if}
	</div>
</dialog>

<style>
	dialog {
		transition: transform 250ms cubic-bezier(0.2, 0.8, 0.2, 1);
	}
	dialog::backdrop {
		transition: opacity 250ms ease;
	}
	dialog[data-hidden] {
		transform: translateY(100%);
	}
	dialog[data-hidden]::backdrop {
		opacity: 0;
	}
	/* With Reduce Motion on, the sheet fades in and out where it stands. */
	@media (prefers-reduced-motion: reduce) {
		dialog {
			transition: opacity 200ms ease;
		}
		dialog[data-hidden] {
			transform: none;
			opacity: 0;
		}
	}
</style>
