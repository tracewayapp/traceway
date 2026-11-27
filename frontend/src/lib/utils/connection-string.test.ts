import { describe, expect, it } from 'vitest';
import {
	isCloudUrl,
	normalizeBaseUrl,
	normalizeConnectionString,
	parseConnectionString,
	selfHostedBaseUrl
} from './connection-string';

// Every spelling the Traceway SDKs accept.
const SPELLINGS: [string, string, string][] = [
	['tok@https://cloud.tracewayapp.com', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@https://cloud.tracewayapp.com/', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@https://cloud.tracewayapp.com/api', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@https://cloud.tracewayapp.com/api/', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@https://cloud.tracewayapp.com/api/report', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@https://cloud.tracewayapp.com/api/report/', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@cloud.tracewayapp.com', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@cloud.tracewayapp.com/api/report', 'tok', 'https://cloud.tracewayapp.com'],
	['  tok@https://cloud.tracewayapp.com/api/report  ', 'tok', 'https://cloud.tracewayapp.com'],
	['tok@http://localhost:8082/api/report', 'tok', 'http://localhost:8082'],
	['tok@http://api', 'tok', 'http://api'],
	['tok@http://api/report', 'tok', 'http://api/report'],
	['tok@http://api/api/report?x', 'tok', 'http://api'],
	['tok@localhost:8082', 'tok', 'http://localhost:8082'],
	['tok@https://corp.example/traceway/api/report', 'tok', 'https://corp.example/traceway'],
	['tok@https://cloud.tracewayapp.com/api/report?x=1', 'tok', 'https://cloud.tracewayapp.com']
];

describe('parseConnectionString', () => {
	it.each(SPELLINGS)('reads %j', (input, token, baseUrl) => {
		expect(parseConnectionString(input)).toEqual({ token, baseUrl });
		expect(normalizeConnectionString(input)).toBe(`${token}@${baseUrl}`);
	});

	it.each(['@https://cloud.tracewayapp.com', 'tok', 'tok@', 'tok@ftp://host', '', '   '])(
		'rejects %j',
		(input) => {
			expect(parseConnectionString(input)).toBeNull();
			expect(normalizeConnectionString(input)).toBeNull();
		}
	);

	it('rejects a missing value', () => {
		expect(parseConnectionString(undefined)).toBeNull();
		expect(parseConnectionString(null)).toBeNull();
	});
});

describe('normalizeBaseUrl', () => {
	it.each([
		['https://cloud.tracewayapp.com/api/report', 'https://cloud.tracewayapp.com'],
		['cloud.tracewayapp.com/', 'https://cloud.tracewayapp.com'],
		['127.0.0.1:8082/api', 'http://127.0.0.1:8082'],
		['10.0.2.2:8082', 'http://10.0.2.2:8082'],
		['[::1]:8082/api/report/', 'http://[::1]:8082'],
		['traceway.internal:8082', 'https://traceway.internal:8082'],
		['https://corp.example/traceway/', 'https://corp.example/traceway'],
		['https://corp.example/traceway/api/api', 'https://corp.example/traceway/api'],
		['https://corp.example#frag', 'https://corp.example']
	])('normalizes %j', (input, want) => {
		expect(normalizeBaseUrl(input)).toBe(want);
	});

	it.each(['', 'ftp://host', 'https://', 'http:///api', '  '])('rejects %j', (input) => {
		expect(normalizeBaseUrl(input)).toBeNull();
	});
});

describe('isCloudUrl and selfHostedBaseUrl', () => {
	it('recognizes Traceway Cloud in any spelling', () => {
		for (const url of [
			'https://cloud.tracewayapp.com',
			'https://cloud.tracewayapp.com/',
			'https://cloud.tracewayapp.com/api/report',
			'cloud.tracewayapp.com',
			'https://Cloud.TracewayApp.com:443'
		]) {
			expect(isCloudUrl(url)).toBe(true);
			expect(selfHostedBaseUrl(url)).toBeNull();
		}
	});

	it('gives every other server its base URL', () => {
		expect(isCloudUrl('http://cloud.tracewayapp.com')).toBe(false);
		expect(isCloudUrl('https://cloud.tracewayapp.com.evil.test')).toBe(false);
		expect(selfHostedBaseUrl('https://traceway.acme.test/')).toBe('https://traceway.acme.test');
		expect(selfHostedBaseUrl('http://localhost:8082/api/report')).toBe('http://localhost:8082');
		expect(selfHostedBaseUrl('https://corp.example/traceway')).toBe(
			'https://corp.example/traceway'
		);
	});
});
