// Array.prototype.at shipped in Chrome 92 and Safari 15.4. The web-vitals build inside the Traceway SDK calls it,
// so on an older browser every page view reported "at is not a function" instead of its metrics.
function at<T>(this: ArrayLike<T>, index: number): T | undefined {
	const length = this.length;
	const i = Math.trunc(index) || 0;
	return this[i < 0 ? i + length : i];
}

for (const proto of [Array.prototype, String.prototype]) {
	if (!('at' in proto)) {
		Object.defineProperty(proto, 'at', { value: at, writable: true, configurable: true });
	}
}
