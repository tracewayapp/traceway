import { describe, expect, it } from 'vitest';
import {
	credentialValue,
	substituteToken,
	uploadTokenEnvFile,
	uploadTokenEnvVar,
	urlEnvVar,
	type SetupDraftProject,
	type SetupPlanProject
} from './setup-plan';

function project(backendUrl: string): SetupDraftProject {
	return { id: 'p1', name: 'Web', framework: 'react', token: 'tok', backendUrl, status: 'created' };
}

describe('credentialValue', () => {
	const planned: SetupPlanProject = {
		name: 'Web',
		framework: 'react',
		envVar: 'VITE_TRACEWAY_TOKEN'
	};

	it('is the token for the start API', () => {
		expect(credentialValue(planned, project('https://cloud.tracewayapp.com'))).toBe('tok');
	});

	it('keeps an old plan readable, without the 1.x suffix', () => {
		const old: SetupPlanProject = { ...planned, envFormat: 'connectionString' };
		for (const backendUrl of [
			'https://cloud.tracewayapp.com',
			'https://cloud.tracewayapp.com/',
			'https://cloud.tracewayapp.com/api/report',
			'cloud.tracewayapp.com'
		]) {
			expect(credentialValue(old, project(backendUrl))).toBe('tok@https://cloud.tracewayapp.com');
		}
		expect(credentialValue(old, project('http://localhost:8082/api'))).toBe(
			'tok@http://localhost:8082'
		);
		expect(credentialValue(old, project('https://corp.example/traceway/'))).toBe(
			'tok@https://corp.example/traceway'
		);
	});
});

describe('urlEnvVar', () => {
	it('is written only off Traceway Cloud', () => {
		const planned: SetupPlanProject = {
			name: 'Web',
			framework: 'react',
			envVar: 'VITE_TRACEWAY_TOKEN'
		};
		expect(urlEnvVar(planned, 'https://cloud.tracewayapp.com/')).toBeNull();
		expect(urlEnvVar(planned, 'https://traceway.acme.test')).toBe('VITE_TRACEWAY_URL');
		expect(urlEnvVar({ ...planned, urlEnvVar: 'SERVER' }, 'https://traceway.acme.test')).toBe(
			'SERVER'
		);
		expect(urlEnvVar({ ...planned, envVar: 'traceway.token' }, 'https://traceway.acme.test')).toBe(
			'traceway.url'
		);
		expect(
			urlEnvVar({ ...planned, envFormat: 'connectionString' }, 'https://traceway.acme.test')
		).toBeNull();
	});
});

describe('upload token wiring', () => {
	it('uses the variable each upload tool reads', () => {
		expect(uploadTokenEnvVar({ name: 'Web', framework: 'svelte' })).toBe(
			'TRACEWAY_SOURCEMAP_TOKEN'
		);
		expect(uploadTokenEnvVar({ name: 'App', framework: 'ios' })).toBe('TRACEWAY_UPLOAD_TOKEN');
		expect(uploadTokenEnvVar({ name: 'App', framework: 'flutter', uploadTokenEnvVar: 'X' })).toBe(
			'X'
		);
	});

	it('is written only when the plan asks for it', () => {
		expect(uploadTokenEnvFile({ name: 'App', framework: 'ios', envFile: 'a.xcconfig' })).toBeNull();
		expect(
			uploadTokenEnvFile({
				name: 'App',
				framework: 'ios',
				envFile: 'a.xcconfig',
				uploadTokenEnvVar: 'TRACEWAY_UPLOAD_TOKEN'
			})
		).toBe('a.xcconfig');
		expect(
			uploadTokenEnvFile({ name: 'App', framework: 'ios', uploadTokenEnvFile: 'secrets.xcconfig' })
		).toBe('secrets.xcconfig');
	});
});

describe('substituteToken', () => {
	it('fills the credential and the server URL', () => {
		expect(
			substituteToken(
				'fly secrets set TRACEWAY_TOKEN=<token> TRACEWAY_URL=<url>',
				'tok',
				'https://traceway.acme.test/api/report'
			)
		).toBe('fly secrets set TRACEWAY_TOKEN=tok TRACEWAY_URL=https://traceway.acme.test');
		expect(substituteToken('vercel env add X <token>', 'tok')).toBe('vercel env add X tok');
	});
});
