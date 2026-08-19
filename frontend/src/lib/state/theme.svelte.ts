export const themeState = $state({
	isDark: true
});

// app.html ships <html> with an anti-FOUC inline `background: #05070c` so the
// first paint is dark. That inline value overrides the CSS `--background`, so
// it must be cleared when switching to light (matching app.html's bootstrap
// script) or a stale dark backdrop bleeds through wherever content does not
// paint over <html>.
function applyTheme(isDark: boolean) {
	document.documentElement.classList.toggle('dark', isDark);
	document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
	document.documentElement.style.background = isDark ? '#05070c' : '';
}

export function initTheme() {
	if (typeof document !== 'undefined') {
		const stored = localStorage.getItem('traceway_theme');

		if (stored === 'dark' || stored === 'light') {
			themeState.isDark = stored === 'dark';
		} else {
			themeState.isDark = true;
		}

		applyTheme(themeState.isDark);

		const observer = new MutationObserver(() => {
			themeState.isDark = document.documentElement.classList.contains('dark');
		});
		observer.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ['class']
		});

		return () => {
			observer.disconnect();
		};
	}
}

export function toggleTheme() {
	themeState.isDark = !themeState.isDark;
	applyTheme(themeState.isDark);
	localStorage.setItem('traceway_theme', themeState.isDark ? 'dark' : 'light');
}
