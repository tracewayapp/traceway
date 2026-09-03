// goto() needs a mounted router it cannot get in jsdom, so tests replace this.
// Throwing rather than no-opping keeps an unmocked navigation from passing.
export function goto(): Promise<void> {
	throw new Error('goto() reached the $app/navigation test stub; mock it in the test');
}
