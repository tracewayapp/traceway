import { describe, expect, it } from 'vitest';
import {
	appIssueType,
	blockedMs,
	formatElapsed,
	formatViewport,
	hangDetails,
	screenContext
} from './app-issue';

describe('appIssueType', () => {
	it('names the four app issue types from the first line', () => {
		expect(
			appIssueType('NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0\n    #00 pc 1c6a4 /x.so')
		).toBe('NativeCrash');
		expect(appIssueType('ApplicationNotResponding: Input dispatching timed out')).toBe(
			'ApplicationNotResponding'
		);
		expect(appIssueType('AppHang: Main thread blocked for 3.2 s')).toBe('AppHang');
		expect(appIssueType('WatchdogTermination')).toBe('WatchdogTermination');
	});

	it('leaves every other exception alone', () => {
		for (const trace of [
			'com.example.AppHang: stuck',
			'java.lang.IllegalStateException: AppHang',
			'toString: x',
			'constructor',
			'',
			null,
			42
		]) {
			expect(appIssueType(trace)).toBeNull();
		}
	});
});

describe('hangDetails', () => {
	it('reads how long the main thread was blocked and whether the app survived it', () => {
		expect(hangDetails('AppHang: Main thread blocked for 3.2 s\n#00 …', {})).toEqual({
			durationMs: 3200,
			fatal: false
		});
		expect(
			hangDetails('AppHang: Main thread blocked for 7.8 s', { 'app.hang.fatal': 'true' })
		).toEqual({ durationMs: 7800, fatal: true });
		expect(hangDetails('AppHang: Main thread blocked for 850 ms', null)?.durationMs).toBe(850);
		expect(hangDetails('AppHang: Main thread blocked for at least 2 min', null)?.durationMs).toBe(
			120_000
		);
		expect(
			hangDetails('AppHang', { 'exception.message': 'Main thread blocked for 2.5 seconds' })
		).toEqual({ durationMs: 2500, fatal: false });
		expect(hangDetails('AppHang: stuck', { 'app.hang.fatal': 'false' })).toEqual({
			durationMs: null,
			fatal: false
		});
	});

	it('is only for hangs', () => {
		expect(hangDetails('ApplicationNotResponding: blocked for 5 s', {})).toBeNull();
		expect(
			hangDetails('Error: Main thread blocked for 3.2 s', { 'app.hang.fatal': 'true' })
		).toBeNull();
		expect(blockedMs(undefined)).toBeNull();
	});
});

describe('formatElapsed', () => {
	it('reads at a glance', () => {
		expect(formatElapsed(0)).toBe('0 ms');
		expect(formatElapsed(850)).toBe('850 ms');
		expect(formatElapsed(3200)).toBe('3.2 s');
		expect(formatElapsed(2000)).toBe('2 s');
		expect(formatElapsed(42_400)).toBe('42 s');
		expect(formatElapsed(252_000)).toBe('4 min 12 s');
		expect(formatElapsed(120_000)).toBe('2 min');
		expect(formatElapsed(119_800)).toBe('2 min');
		expect(formatElapsed(999.6)).toBe('1 s');
		expect(formatElapsed(59_700)).toBe('1 min');
		expect(formatElapsed(3_599_800)).toBe('1 h');
		expect(formatElapsed(3_900_000)).toBe('1 h 5 min');
		expect(formatElapsed(-1)).toBe('-');
		expect(formatElapsed(NaN)).toBe('-');
	});
});

describe('screenContext', () => {
	it('takes the keys every SDK stamps', () => {
		expect(
			screenContext({
				'app.screen.name': '/orders/:id',
				'app.screen.route': '/orders/:id',
				'app.screen.previous': '/orders',
				'app.screen.duration_ms': '1520',
				viewport: '1440x900',
				'user.id': 'u1'
			})
		).toEqual({
			name: '/orders/:id',
			route: '/orders/:id',
			previous: '/orders',
			durationMs: 1520,
			viewport: '1440x900'
		});
		expect(screenContext({ 'app.screen.name': '/CheckoutActivity' })).toEqual({
			name: '/CheckoutActivity'
		});
	});

	it('is null when there is nothing to show', () => {
		expect(screenContext(null)).toBeNull();
		expect(screenContext({})).toBeNull();
		expect(screenContext({ 'app.screen.name': '  ', 'app.screen.duration_ms': 'soon' })).toBeNull();
		expect(screenContext({ 'app.screen.duration_ms': '-5' })).toBeNull();
		expect(screenContext('x' as never)).toBeNull();
	});

	it('writes a viewport as a size', () => {
		expect(formatViewport('411x914')).toBe('411 × 914');
		expect(formatViewport('wide')).toBe('wide');
	});
});
