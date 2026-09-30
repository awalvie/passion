const key = 'passion-token';

// The token lives in localStorage rather than a cookie because Capacitor drops
// persistent cookies on iOS app restart, and the same code has to work in both
// the browser and the wrapped app.
export function token(): string | null {
	return localStorage.getItem(key);
}

export function setToken(value: string) {
	localStorage.setItem(key, value);
}

export function clearToken() {
	localStorage.removeItem(key);
}

// clearRunStorage drops the running timers and unsent writes, which belong to
// the account that signs out.
export function clearRunStorage() {
	for (const k of Object.keys(localStorage)) {
		if (k.startsWith('passion-timer:') || k.startsWith('passion-pending:')) localStorage.removeItem(k);
	}
}
