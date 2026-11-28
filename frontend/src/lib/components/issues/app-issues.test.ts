import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';

const state = vi.hoisted(() => ({ framework: 'react' }));

vi.mock('rrweb-player', () => ({ default: class {} }));
vi.mock('$lib/api', () => ({ api: { get: vi.fn().mockResolvedValue(null), post: vi.fn() } }));
vi.mock('$lib/state/timezone.svelte', () => ({ getTimezone: () => 'UTC' }));
vi.mock('$lib/state/projects.svelte', async (importOriginal) => ({
	...(await importOriginal<typeof import('$lib/state/projects.svelte')>()),
	projectsState: {
		currentProjectId: 'project-1',
		get currentProject() {
			return { id: 'project-1', framework: state.framework };
		},
		canWriteCurrentProject: false
	}
}));

import EventCard from '../../../test/event-card.svelte';
import StackTraceCard from './stack-trace-card.svelte';
import SessionActionsTable from '../../../test/session-actions-table.svelte';

function occurrence(stackTrace: string, attributes: Record<string, string>) {
	return {
		id: 'e1',
		traceId: '',
		spanId: '',
		traceType: '' as const,
		exceptionHash: 'h1',
		stackTrace,
		recordedAt: '2026-11-27T10:00:00Z',
		attributes,
		appVersion: '4.2.0',
		serverName: '',
		isMessage: false,
		endpoint: ''
	};
}

const SCREEN = {
	'app.screen.name': '/orders/:id',
	'app.screen.route': '/orders/:id',
	'app.screen.previous': '/orders',
	'app.screen.duration_ms': '1520',
	viewport: '1440x900'
};

beforeEach(() => {
	state.framework = 'react';
});

afterEach(() => cleanup());

// A browser issue shows where the page was, like an app issue shows its screen.
describe('screen context on an issue', () => {
	it('shows the page of a browser issue', () => {
		render(EventCard, {
			occurrence: occurrence('TypeError: x is undefined', SCREEN),
			linkedTrace: null
		});
		expect(screen.getByText('Previous page')).toBeTruthy();
		// The attributes list below shows the raw values too.
		expect(screen.getAllByText('/orders')).toHaveLength(2);
		expect(screen.getByText('Route')).toBeTruthy();
		expect(screen.getByText('Time on page')).toBeTruthy();
		expect(screen.getByText('1.5 s')).toBeTruthy();
		expect(screen.getByText('1440 × 900')).toBeTruthy();
	});

	it('shows the screen of an app issue', () => {
		state.framework = 'android';
		render(EventCard, {
			occurrence: occurrence('java.lang.IllegalStateException: x', {
				'app.screen.name': '/CheckoutActivity',
				'app.screen.duration_ms': '252000'
			}),
			linkedTrace: null
		});
		expect(screen.getByText('Time on screen')).toBeTruthy();
		expect(screen.getByText('4 min 12 s')).toBeTruthy();
		expect(screen.queryByText('Route')).toBeNull();
		expect(screen.queryByText('Viewport')).toBeNull();
	});

	it('shows no empty section when the SDK sent none', () => {
		render(EventCard, {
			occurrence: occurrence('TypeError: x', { 'user.id': 'u1' }),
			linkedTrace: null
		});
		expect(screen.queryByText('Page')).toBeNull();
		expect(screen.queryByText('Time on page')).toBeNull();
		expect(screen.queryByText('App Hang')).toBeNull();
	});
});

// A hang says how long the main thread was blocked and whether the app survived it.
describe('an app hang', () => {
	it('reads its duration and that it was fatal', () => {
		state.framework = 'ios';
		render(EventCard, {
			occurrence: occurrence('AppHang: Main thread blocked for 7.8 s\n#00 …', {
				'app.hang.fatal': 'true'
			}),
			linkedTrace: null
		});
		expect(screen.getByText('App Hang')).toBeTruthy();
		expect(screen.getByText('7.8 s')).toBeTruthy();
		expect(screen.getByText('Yes, the app was terminated during the hang')).toBeTruthy();
	});

	it('says when the app recovered', () => {
		state.framework = 'android';
		render(EventCard, {
			occurrence: occurrence('AppHang: Main thread blocked for 2.4 s', {}),
			linkedTrace: null
		});
		expect(screen.getByText('2.4 s')).toBeTruthy();
		expect(screen.getByText('No, the app recovered')).toBeTruthy();
	});
});

const TOMBSTONE = `NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0000000000000000
    #00 pc 000000000001c6a4  /data/app/~~Gf3k==/com.example.shop-Hn7d==/lib/arm64/libpayments.so (pay::Card::charge(int)+52) (BuildId: 6f2c1a9e)
    #01 pc 00000000002d4044  /apex/com.android.art/lib64/libart.so (art_quick_generic_jni_trampoline+148) (BuildId: 9e7c7dcb)
    #02 pc 00000000002ca9e8  /apex/com.android.art/lib64/libart.so (art_quick_invoke_stub+568) (BuildId: 9e7c7dcb)
    #03 pc 0000000000412b4c  /data/app/~~Gf3k==/com.example.shop-Hn7d==/oat/arm64/base.odex (com.example.shop.Payments.charge+124)`;

describe('native frames in the stack view', () => {
	it('shows a tombstone frame by frame, the platform’s libraries folded', async () => {
		state.framework = 'android';
		render(StackTraceCard, { stackTrace: TOMBSTONE, showArchiveDialog: false, archiving: false });
		expect(screen.getByText('libpayments.so')).toBeTruthy();
		expect(screen.getByText('pay::Card::charge(int)+52')).toBeTruthy();
		expect(screen.getByText('com.example.shop.Payments.charge+124')).toBeTruthy();
		expect(screen.getByText('2 system frames')).toBeTruthy();
		expect(screen.queryByText('art_quick_invoke_stub+568')).toBeNull();
		await fireEvent.click(screen.getByText('libart.so'));
		expect(screen.getByText('art_quick_invoke_stub+568')).toBeTruthy();
	});

	it('shows unsymbolicated iOS frames by image and offset', () => {
		state.framework = 'ios';
		render(StackTraceCard, {
			stackTrace:
				'WatchdogTermination: Watchdog terminated the app (0x8badf00d)\n*** *** ***\nos: ios arch: arm64e\n#00 6F1C2A3B-4C5D-6E7F-8091-A2B3C4D5E6F7 0x1d2c4 libsystem_kernel.dylib\n#01 0a1b2c3d4e5f60718293a4b5c6d7e8f9 0x55e10 ShopApp',
			isIOS: true,
			showArchiveDialog: false,
			archiving: false
		});
		expect(screen.getByText('ShopApp+0x55e10')).toBeTruthy();
		expect(screen.getByText('1 system frame')).toBeTruthy();
		expect(screen.queryByText(/os: ios arch/)).toBeNull();
	});
});

// Browser rows carry the page and route template the browser SDK now stamps.
describe('the actions table of a browser session', () => {
	it('reads a click with its page and a navigation with its route template', () => {
		render(SessionActionsTable, {
			actions: [
				{
					type: 'custom',
					timestamp: '2026-11-27T10:00:01Z',
					category: 'ui',
					name: 'click',
					data: { selector: 'button#pay', tag: 'button', text: 'Pay now', screen: '/orders/:id' }
				},
				{
					type: 'navigation',
					timestamp: '2026-11-27T10:00:02Z',
					action: 'push',
					from: '/orders',
					to: '/orders/42',
					route: '/orders/:id'
				},
				{
					type: 'custom',
					timestamp: '2026-11-27T10:00:03Z',
					category: 'ui',
					name: 'click',
					data: { selector: 'a.logo' }
				}
			],
			startedAt: '2026-11-27T10:00:00Z'
		});
		expect(screen.getByText('Click button#pay “Pay now” on /orders/:id')).toBeTruthy();
		expect(screen.getByText('push: /orders → /orders/42 (/orders/:id)')).toBeTruthy();
		// Without the page, the row reads as before.
		expect(screen.getByText('ui/click {"selector":"a.logo"}')).toBeTruthy();
	});
});
