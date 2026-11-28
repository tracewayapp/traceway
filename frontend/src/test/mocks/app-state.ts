// $app/state needs SvelteKit's client router, which jsdom cannot run, so tests replace this. Like the
// $app/navigation stub, reading it unmocked throws rather than passing on an empty page.
export const page: object = new Proxy(
	{},
	{
		get() {
			throw new Error('page reached the $app/state test stub; mock it in the test');
		}
	}
);
