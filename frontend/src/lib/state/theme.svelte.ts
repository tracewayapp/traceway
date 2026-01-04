// Shared reactive theme state using Svelte 5 runes
// This module provides a single source of truth for dark mode state

export const themeState = $state({
    isDark: false
});

// Initialize theme from localStorage or system preference
export function initTheme() {
    if (typeof document !== 'undefined') {
        themeState.isDark = document.documentElement.classList.contains('dark');

        // Watch for class changes on documentElement
        const observer = new MutationObserver(() => {
            themeState.isDark = document.documentElement.classList.contains('dark');
        });
        observer.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ['class']
        });

        return () => observer.disconnect();
    }
}

// Toggle theme and persist to localStorage
export function toggleTheme() {
    themeState.isDark = !themeState.isDark;
    if (themeState.isDark) {
        document.documentElement.classList.add('dark');
        localStorage.setItem('theme', 'dark');
    } else {
        document.documentElement.classList.remove('dark');
        localStorage.setItem('theme', 'light');
    }
}
