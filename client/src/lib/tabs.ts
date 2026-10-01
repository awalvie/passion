import type { IconName } from './Icon.svelte';

// roots are a tab's own top pages; anything under one belongs to the tab.
export const tabs: { href: string; label: string; icon: IconName; roots: string[] }[] = [
	{ href: '/', label: 'Today', icon: 'peak', roots: ['/'] },
	{ href: '/plan', label: 'Plan', icon: 'calendar', roots: ['/plan'] },
	{ href: '/templates', label: 'Library', icon: 'stack', roots: ['/templates', '/exercises'] },
	{ href: '/history', label: 'History', icon: 'clock', roots: ['/history'] }
];

export const under = (path: string, root: string) => path === root || (root !== '/' && path.startsWith(root + '/'));

// tabOf is the tab a page belongs to, or -1 for a page outside the tabs.
export const tabOf = (path: string) => tabs.findIndex((t) => t.roots.some((r) => under(path, r)));
