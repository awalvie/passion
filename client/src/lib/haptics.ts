// A light tick under the finger when a control changes. iPhone Safari has no
// vibration API, but a real tap on a switch ticks, so the action lays an
// invisible label for a hidden switch over the control: the tap that presses
// the control also flips the switch. A label cannot do this inside a link or a
// submit button, so it goes on type="button" controls only. Android buzzes.

const key = 'passion-haptics';

export function hapticsOn(): boolean {
	return localStorage.getItem(key) !== 'off';
}

export function setHaptics(on: boolean) {
	if (on) localStorage.removeItem(key);
	else localStorage.setItem(key, 'off');
}

const ios = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);

export function haptic(node: HTMLElement, on = true) {
	if (!on) return;
	if (!ios) {
		const buzz = () => hapticsOn() && !node.matches(':disabled') && navigator.vibrate?.(8);
		node.addEventListener('click', buzz);
		return { destroy: () => node.removeEventListener('click', buzz) };
	}

	const label = document.createElement('label');
	label.setAttribute('aria-hidden', 'true');
	label.style.cssText = 'position:absolute;inset:0;touch-action:manipulation;-webkit-tap-highlight-color:transparent';
	const toggle = document.createElement('input');
	toggle.type = 'checkbox';
	toggle.setAttribute('switch', '');
	toggle.tabIndex = -1;
	// Under the finger, the switch would take the touch and stop a scroll.
	toggle.style.cssText = 'position:absolute;width:1px;height:1px;margin:0;visibility:hidden';
	// The label passes its click on to the switch; that copy must not reach
	// the control's own click handler a second time.
	toggle.addEventListener('click', (e) => e.stopPropagation());
	label.addEventListener('click', (e) => {
		if (!hapticsOn() || node.matches(':disabled')) e.preventDefault();
	});
	label.append(toggle);
	if (getComputedStyle(node).position === 'static') node.style.position = 'relative';
	node.append(label);
	return { destroy: () => label.remove() };
}

// A check box ticks on iPhone when it is itself a switch.
export function tickBox(node: HTMLInputElement) {
	if (!ios) return haptic(node);
	if (hapticsOn()) node.setAttribute('switch', '');
}
