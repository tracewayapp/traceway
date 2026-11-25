import { getContext, setContext, untrack } from 'svelte';

const KEY = Symbol('widget-loads');

// How many widget requests a dashboard has out, so its scope filters can wait for them to
// answer. A widget reports each request it starts and settles; one outside a page that
// provides a counter reports to nothing.
export class WidgetLoads {
	pending = $state(0);

	// Widgets start their requests inside an effect; untracked, the count does not become
	// one of that effect's dependencies (which would reload every widget on every change).
	start() {
		untrack(() => this.pending++);
	}

	end() {
		untrack(() => this.pending--);
	}
}

export function provideWidgetLoads(): WidgetLoads {
	return setContext(KEY, new WidgetLoads());
}

export function widgetLoads(): WidgetLoads | undefined {
	return getContext<WidgetLoads | undefined>(KEY);
}
