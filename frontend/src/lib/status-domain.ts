// Set by the backend on index.html when the request's Host is a status page's
// custom domain: the app then renders only that status page, at /.
export const statusDomainSlug: string | null =
	typeof document === 'undefined'
		? null
		: (document.querySelector('meta[name="traceway-status-slug"]')?.getAttribute('content') ??
			null);
