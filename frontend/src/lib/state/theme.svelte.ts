export const themeState = $state({
    isDark: false
});

export function initTheme() {
    if (typeof document !== 'undefined') {
        // Check localStorage first
        const stored = localStorage.getItem('theme');

        if (stored === 'dark' || stored === 'light') {
            // Use stored preference
            themeState.isDark = stored === 'dark';
        } else {
            // Default to light mode
            themeState.isDark = false;
        }

        // Apply the theme
        document.documentElement.classList.toggle('dark', themeState.isDark);

        // Watch for external class changes
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
    document.documentElement.classList.toggle('dark', themeState.isDark);
    localStorage.setItem('theme', themeState.isDark ? 'dark' : 'light');
}
