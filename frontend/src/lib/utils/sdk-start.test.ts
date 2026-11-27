import { describe, expect, it } from 'vitest';
import type { Framework } from '$lib/state/projects.svelte';
import {
	getCodeLanguage,
	getFrameworkCode,
	getInstallCommand,
	getInstallLanguage
} from './framework-code';
import { getOtelSteps, OTEL_TARGETS } from './otel-setup';
import { getStartCode, getStartConfigFiles, usesStartApi, xcconfigValue } from './sdk-start';

const CLOUD = 'https://cloud.tracewayapp.com';
const SELF_HOSTED = 'https://traceway.acme.test';
const TOKEN = 'abc123token';

const START_FRAMEWORKS: Framework[] = [
	'react',
	'svelte',
	'vuejs',
	'jquery',
	'nextjs',
	'flutter',
	'android',
	'ios'
];

function everything(framework: Framework, backendUrl: string): string {
	return [
		getFrameworkCode(framework, TOKEN, backendUrl),
		...getStartConfigFiles(framework, TOKEN, backendUrl).map((f) => f.code)
	].join('\n');
}

describe('start snippets', () => {
	it.each(START_FRAMEWORKS)('%s uses token and no connection string', (framework) => {
		expect(usesStartApi(framework)).toBe(true);
		for (const backendUrl of [CLOUD, SELF_HOSTED]) {
			const text = everything(framework, backendUrl);
			expect(text).not.toContain('/api/report');
			expect(text).not.toMatch(/connectionString/i);
			expect(text).not.toContain(`${TOKEN}@`);
			expect(text).toContain('TRACEWAY_TOKEN');
			expect(text).toContain(TOKEN);
		}
	});

	it.each(START_FRAMEWORKS)('%s names the server only off Traceway Cloud', (framework) => {
		for (const cloud of [CLOUD, `${CLOUD}/`, `${CLOUD}/api/report`]) {
			const text = everything(framework, cloud);
			expect(text).not.toMatch(/baseUrl/);
			expect(text).not.toContain('TRACEWAY_URL');
			expect(text).not.toContain('cloud.tracewayapp.com');
		}
		const selfHosted = everything(framework, `${SELF_HOSTED}/`);
		expect(selfHosted).toMatch(/baseUrl/);
		expect(selfHosted).toContain('TRACEWAY_URL');
	});

	it('reads the browser token from the framework public variable', () => {
		const cases: [Framework, string, string][] = [
			['react', 'token={import.meta.env.VITE_TRACEWAY_TOKEN}', 'VITE_TRACEWAY_TOKEN=abc123token'],
			['vuejs', 'token: import.meta.env.VITE_TRACEWAY_TOKEN,', 'VITE_TRACEWAY_TOKEN=abc123token'],
			['jquery', 'token: import.meta.env.VITE_TRACEWAY_TOKEN,', 'VITE_TRACEWAY_TOKEN=abc123token'],
			['svelte', 'token: PUBLIC_TRACEWAY_TOKEN,', 'PUBLIC_TRACEWAY_TOKEN=abc123token'],
			[
				'nextjs',
				'token={process.env.NEXT_PUBLIC_TRACEWAY_TOKEN}',
				'NEXT_PUBLIC_TRACEWAY_TOKEN=abc123token'
			]
		];
		for (const [framework, code, env] of cases) {
			expect(getStartCode(framework, CLOUD)).toContain(code);
			const [file] = getStartConfigFiles(framework, TOKEN, CLOUD);
			expect(file.code).toBe(env);
			expect(file.secret).toBe(true);
		}
		expect(getStartConfigFiles('svelte', TOKEN, SELF_HOSTED)[0].code).toBe(
			'PUBLIC_TRACEWAY_TOKEN=abc123token\nPUBLIC_TRACEWAY_URL=https://traceway.acme.test'
		);
		expect(getStartCode('svelte', SELF_HOSTED)).toContain(
			'import { PUBLIC_TRACEWAY_TOKEN, PUBLIC_TRACEWAY_URL } from "$env/static/public";'
		);
		expect(getStartCode('react', SELF_HOSTED)).toContain(
			'baseUrl={import.meta.env.VITE_TRACEWAY_URL}'
		);
	});

	it('starts the mobile SDKs with the section 16 calls', () => {
		expect(getStartCode('flutter', CLOUD)).toContain(
			"Traceway.run(\n    token: const String.fromEnvironment('TRACEWAY_TOKEN'),"
		);
		expect(getStartCode('flutter', SELF_HOSTED)).toContain(
			"baseUrl: const String.fromEnvironment('TRACEWAY_URL'),"
		);
		expect(getStartCode('android', CLOUD)).toContain(
			'Traceway.init(\n            this,\n            TracewayOptions(\n                token = BuildConfig.TRACEWAY_TOKEN,'
		);
		expect(getStartCode('android', SELF_HOSTED)).toContain('baseUrl = BuildConfig.TRACEWAY_URL,');
		expect(getStartCode('ios', CLOUD)).toContain(
			'Traceway.initialize(\n            token: info?["TracewayToken"] as? String ?? "",'
		);
		expect(getStartCode('ios', SELF_HOSTED)).toContain(
			'baseUrl: info?["TracewayURL"] as? String ?? "",'
		);
	});

	it('wires the mobile token through files the build reads', () => {
		const flutter = getStartConfigFiles('flutter', TOKEN, SELF_HOSTED);
		expect(flutter[0].code).toBe(
			'TRACEWAY_TOKEN=abc123token\nTRACEWAY_URL=https://traceway.acme.test'
		);
		expect(flutter[1].code).toContain('--dart-define-from-file=.env');

		const android = getStartConfigFiles('android', TOKEN, SELF_HOSTED);
		expect(android.map((f) => f.title)).toEqual(['local.properties', 'app/build.gradle.kts']);
		expect(android[0].code).toBe(
			'TRACEWAY_TOKEN=abc123token\nTRACEWAY_URL=https://traceway.acme.test'
		);
		expect(android[1].code).toContain(
			'buildConfigField("String", "TRACEWAY_TOKEN", "\\"${localProperties.getProperty("TRACEWAY_TOKEN", "")}\\"")'
		);
		expect(android[1].code).toContain('"TRACEWAY_URL"');
		expect(getStartConfigFiles('android', TOKEN, CLOUD)[1].code).not.toContain('TRACEWAY_URL');

		const ios = getStartConfigFiles('ios', TOKEN, SELF_HOSTED);
		expect(ios.map((f) => f.title)).toEqual(['Traceway.xcconfig', 'Info.plist']);
		expect(ios[0].code).toContain('TRACEWAY_TOKEN = abc123token');
		// A bare "//" would cut the value to "https:" in an .xcconfig file.
		expect(ios[0].code).toContain('TRACEWAY_URL = https:/$()/traceway.acme.test');
		expect(ios[1].code).toContain('<string>$(TRACEWAY_TOKEN)</string>');
		expect(ios[1].code).toContain('<key>TracewayURL</key>');
		expect(getStartConfigFiles('ios', TOKEN, CLOUD)[1].code).not.toContain('TracewayURL');
	});

	it('uses Svelte 5 syntax', () => {
		const svelte = getStartCode('svelte', CLOUD);
		expect(svelte).toContain('let { children } = $props();');
		expect(svelte).toContain('{@render children()}');
		expect(svelte).not.toContain('<slot');
		expect(svelte).not.toContain('on:');
	});

	it('hands a bundled jQuery to the jQuery SDK', () => {
		for (const backendUrl of [CLOUD, SELF_HOSTED]) {
			const jquery = getStartCode('jquery', backendUrl);
			expect(jquery).toContain('import $ from "jquery";');
			expect(jquery).toContain('  jQuery: $,\n});');
		}
	});

	it('highlights Dart, Kotlin and Swift as themselves', () => {
		expect(getCodeLanguage('flutter')).toBe('dart');
		expect(getCodeLanguage('android')).toBe('kotlin');
		expect(getCodeLanguage('ios')).toBe('swift');
		expect(getCodeLanguage('react')).toBe('javascript');
		expect(getInstallLanguage('android')).toBe('kotlin');
		expect(getInstallLanguage('ios')).toBe('swift');
		expect(getInstallLanguage('flutter')).toBe('bash');
		expect(getInstallLanguage('react')).toBe('bash');
		expect(getStartConfigFiles('android', TOKEN, CLOUD)[1].language).toBe('kotlin');
	});

	it('falls back to a placeholder without a token', () => {
		expect(getStartConfigFiles('react', '', CLOUD)[0].code).toBe('VITE_TRACEWAY_TOKEN=YOUR_TOKEN');
	});

	it('escapes every // for xcconfig', () => {
		expect(xcconfigValue('http://localhost:8082')).toBe('http:/$()/localhost:8082');
		expect(xcconfigValue('abc')).toBe('abc');
	});
});

describe('install lines', () => {
	it('pin 2.1.0 for the browser and mobile SDKs', () => {
		expect(getInstallCommand('react')).toBe('npm install @tracewayapp/react@2.1.0');
		expect(getInstallCommand('nextjs')).toBe('npm install @tracewayapp/react@2.1.0');
		expect(getInstallCommand('svelte')).toBe('npm install @tracewayapp/svelte@2.1.0');
		expect(getInstallCommand('vuejs')).toBe('npm install @tracewayapp/vue@2.1.0');
		expect(getInstallCommand('jquery')).toBe('npm install @tracewayapp/jquery@2.1.0');
		expect(getInstallCommand('flutter')).toBe("flutter pub add 'traceway:^2.1.0'");
		expect(getInstallCommand('android')).toBe('implementation("com.tracewayapp:traceway:2.1.0")');
		expect(getInstallCommand('ios')).toBe(
			'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "2.1.0")'
		);
	});
});

describe('frameworks outside the start API', () => {
	it('keep the OTLP setup without /api/report', () => {
		for (const framework of ['laravel', 'django', 'symfony', 'opentelemetry'] as Framework[]) {
			expect(usesStartApi(framework)).toBe(false);
			expect(getFrameworkCode(framework, TOKEN, SELF_HOSTED)).not.toContain('/api/report');
		}
		expect(getFrameworkCode('laravel', TOKEN, `${SELF_HOSTED}/`)).toContain(
			'OTEL_EXPORTER_OTLP_ENDPOINT=https://traceway.acme.test/api/otel'
		);
		for (const target of OTEL_TARGETS) {
			for (const framework of [{ id: '' }, ...target.frameworks]) {
				for (const step of getOtelSteps(target.id, framework.id, SELF_HOSTED, TOKEN)) {
					expect(`${step.title} ${step.description ?? ''} ${step.code ?? ''}`).not.toContain(
						'/api/report'
					);
				}
			}
		}
	});

	it('keep the 1.x connection string only for React Native, whose SDK still posts to it', () => {
		expect(usesStartApi('react-native')).toBe(false);
		expect(getFrameworkCode('react-native', TOKEN, `${SELF_HOSTED}/`)).toContain(
			`${TOKEN}@${SELF_HOSTED}/api/report`
		);
	});

	it('have no Node.js snippet: those projects get the OpenTelemetry guide', () => {
		for (const framework of ['nestjs', 'express', 'remix'] as Framework[]) {
			expect(getFrameworkCode(framework, TOKEN, SELF_HOSTED)).toBe('');
			expect(getInstallCommand(framework)).toBe('');
		}
	});
});
