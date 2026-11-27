import { describe, expect, it } from 'vitest';
import { SDK_VERSION } from './sdk-start';
import { ANDROID_SYMBOL_UPLOAD_COMMAND, getAndroidSymbolUploadFiles } from './symbol-upload';

const CLOUD = 'https://cloud.tracewayapp.com';
const SELF_HOSTED = 'https://traceway.acme.test';
const UPLOAD_TOKEN = 'upl0adt0ken';

function file(backendUrl: string, title: string, token = UPLOAD_TOKEN) {
	const found = getAndroidSymbolUploadFiles(token, backendUrl).find((f) => f.title === title);
	if (!found) throw new Error(`no ${title}`);
	return found;
}

describe('Android symbol upload snippets', () => {
	it('puts the upload token in local.properties, as a secret', () => {
		const props = file(CLOUD, 'local.properties');
		expect(props.code).toBe(`TRACEWAY_UPLOAD_TOKEN=${UPLOAD_TOKEN}`);
		expect(props.language).toBe('properties');
		expect(props.secret).toBe(true);
	});

	it('shows a placeholder before a token exists', () => {
		expect(file(CLOUD, 'local.properties', '').code).toBe(
			'TRACEWAY_UPLOAD_TOKEN=YOUR_UPLOAD_TOKEN'
		);
	});

	it.each([CLOUD, SELF_HOSTED])(
		'never writes the token into build.gradle.kts (%s)',
		(backendUrl) => {
			const gradle = file(backendUrl, 'app/build.gradle.kts');
			expect(gradle.code).not.toContain(UPLOAD_TOKEN);
			expect(gradle.secret).toBe(false);
			expect(gradle.language).toBe('kotlin');
			expect(gradle.code).toContain(
				'uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN") ?: localProperties.getProperty("TRACEWAY_UPLOAD_TOKEN")'
			);
			expect(gradle.code).toContain('rootProject.file("local.properties")');
			expect(gradle.code).toContain(`id("com.tracewayapp.symbols") version "${SDK_VERSION}"`);
			expect(gradle.code).toContain('release { isMinifyEnabled = true }');
		}
	);

	it('names no server on Traceway Cloud, where the plugin uploads by default', () => {
		const gradle = file(CLOUD, 'app/build.gradle.kts');
		expect(gradle.code).not.toMatch(/\burl\s*=/);
		expect(gradle.code).not.toContain('cloud.tracewayapp.com');
	});

	it('names a self-hosted server by its base URL', () => {
		expect(file(SELF_HOSTED, 'app/build.gradle.kts').code).toContain(`url = "${SELF_HOSTED}"`);
		expect(file(`${SELF_HOSTED}/api/report/`, 'app/build.gradle.kts').code).toContain(
			`url = "${SELF_HOSTED}"`
		);
	});

	it('uploads only the R8 mapping, no native libraries', () => {
		const text = getAndroidSymbolUploadFiles(UPLOAD_TOKEN, CLOUD)
			.map((f) => `${f.description}\n${f.code}`)
			.join('\n');
		expect(text).not.toMatch(/\.so\b|native/i);
		expect(ANDROID_SYMBOL_UPLOAD_COMMAND).toBe(
			'./gradlew assembleRelease uploadReleaseTracewaySymbols'
		);
	});
});
