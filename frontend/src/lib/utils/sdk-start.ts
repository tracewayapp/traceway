// The start snippets of the browser and mobile SDKs: the project token in TRACEWAY_TOKEN (with the
// framework's public prefix in browsers), and the server in TRACEWAY_URL / baseUrl only off Traceway Cloud, where
// the SDKs default to it.
import type { Framework } from '$lib/state/projects.svelte';
import { selfHostedBaseUrl } from './connection-string';

export const SDK_VERSION = '2.1.0';

export type StartLanguage = 'bash' | 'ini' | 'properties' | 'xml' | 'kotlin';

export interface StartConfigFile {
	title: string;
	description: string;
	code: string;
	language: StartLanguage;
	secret: boolean;
}

type BrowserEnv = {
	file: string;
	prefix: string;
	description: string;
};

const BROWSER_ENV: Partial<Record<Framework, BrowserEnv>> = {
	react: { file: '.env', prefix: 'VITE_', description: 'Vite exposes VITE_ variables to the app.' },
	vuejs: { file: '.env', prefix: 'VITE_', description: 'Vite exposes VITE_ variables to the app.' },
	jquery: {
		file: '.env',
		prefix: 'VITE_',
		description: 'Vite exposes VITE_ variables to the app.'
	},
	svelte: {
		file: '.env',
		prefix: 'PUBLIC_',
		description: 'SvelteKit exposes PUBLIC_ variables through $env/static/public.'
	},
	nextjs: {
		file: '.env.local',
		prefix: 'NEXT_PUBLIC_',
		description: 'Next.js exposes NEXT_PUBLIC_ variables to the browser.'
	}
};

/** Whether the framework starts with `token` (+ `baseUrl`), the start API of the 2.1 SDKs. */
export function usesStartApi(framework: Framework): boolean {
	return framework in BROWSER_ENV || isMobileStartFramework(framework);
}

function isMobileStartFramework(framework: Framework): boolean {
	return framework === 'flutter' || framework === 'android' || framework === 'ios';
}

/** An .xcconfig file reads `//` as the start of a comment; `/$()/` expands to `//` in the build setting. */
export function xcconfigValue(value: string): string {
	return value.replaceAll('//', '/$()/');
}

function envLines(entries: [string, string | null][]): string {
	return entries
		.filter((entry): entry is [string, string] => entry[1] !== null)
		.map(([key, value]) => `${key}=${value}`)
		.join('\n');
}

/** The env and build files that carry the token (and the server URL off Cloud), with the real values. */
export function getStartConfigFiles(
	framework: Framework,
	token: string,
	backendUrl: string
): StartConfigFile[] {
	const value = token || 'YOUR_TOKEN';
	const url = selfHostedBaseUrl(backendUrl);

	const browser = BROWSER_ENV[framework];
	if (browser) {
		return [
			{
				title: browser.file,
				description: browser.description,
				code: envLines([
					[`${browser.prefix}TRACEWAY_TOKEN`, value],
					[`${browser.prefix}TRACEWAY_URL`, url]
				]),
				language: 'ini',
				secret: true
			}
		];
	}

	switch (framework) {
		case 'flutter':
			return [
				{
					title: '.env',
					description: 'Keep it out of version control.',
					code: envLines([
						['TRACEWAY_TOKEN', value],
						['TRACEWAY_URL', url]
					]),
					language: 'ini',
					secret: true
				},
				{
					title: 'Run and build with it',
					description: 'The values become compile-time constants for String.fromEnvironment.',
					code: 'flutter run --dart-define-from-file=.env\nflutter build apk --dart-define-from-file=.env',
					language: 'bash',
					secret: false
				}
			];
		case 'android':
			return [
				{
					title: 'local.properties',
					description: 'At the project root; Android Studio keeps it out of version control.',
					code: envLines([
						['TRACEWAY_TOKEN', value],
						['TRACEWAY_URL', url]
					]),
					language: 'properties',
					secret: true
				},
				{
					title: 'app/build.gradle.kts',
					description: 'Turns the values into BuildConfig fields.',
					code: androidBuildConfig(url !== null),
					language: 'kotlin',
					secret: false
				}
			];
		case 'ios':
			return [
				{
					title: 'Traceway.xcconfig',
					description:
						"Set it as the app target's configuration file, or #include it from yours. Keep it out of version control.",
					code: [
						'// "//" starts a comment here, so a URL is written as https:/$()/host.',
						`TRACEWAY_TOKEN = ${value}`,
						...(url ? [`TRACEWAY_URL = ${xcconfigValue(url)}`] : [])
					].join('\n'),
					language: 'ini',
					secret: true
				},
				{
					title: 'Info.plist',
					description: 'Passes the build settings to the app.',
					code: [
						'<key>TracewayToken</key>',
						'<string>$(TRACEWAY_TOKEN)</string>',
						...(url ? ['<key>TracewayURL</key>', '<string>$(TRACEWAY_URL)</string>'] : [])
					].join('\n'),
					language: 'xml',
					secret: false
				}
			];
		default:
			return [];
	}
}

function androidBuildConfig(selfHosted: boolean): string {
	const field = (name: string) =>
		`        buildConfigField("String", "${name}", "\\"\${localProperties.getProperty("${name}", "")}\\"")`;
	return `import java.util.Properties

val localProperties = Properties().apply {
    rootProject.file("local.properties").takeIf { it.exists() }?.reader()?.use { load(it) }
}

android {
    buildFeatures { buildConfig = true }
    defaultConfig {
${[field('TRACEWAY_TOKEN'), ...(selfHosted ? [field('TRACEWAY_URL')] : [])].join('\n')}
    }
}`;
}

/** The SDK start code; it reads the token (and the server URL off Cloud) from the files above. */
export function getStartCode(framework: Framework, backendUrl: string): string {
	const selfHosted = selfHostedBaseUrl(backendUrl) !== null;
	const viteToken = 'import.meta.env.VITE_TRACEWAY_TOKEN';
	const viteUrl = 'import.meta.env.VITE_TRACEWAY_URL';

	switch (framework) {
		case 'react':
			return `import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
${reactProvider(viteToken, viteUrl, selfHosted, '    ')}
      <YourApp />
    </TracewayProvider>
  );
}

export default App;`;

		case 'nextjs':
			return `// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
${reactProvider('process.env.NEXT_PUBLIC_TRACEWAY_TOKEN', 'process.env.NEXT_PUBLIC_TRACEWAY_URL', selfHosted, '    ')}
      {children}
    </TracewayProvider>
  );
}

// app/layout.tsx
import { TracewayClientProvider } from "./traceway-provider";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        <TracewayClientProvider>{children}</TracewayClientProvider>
      </body>
    </html>
  );
}`;

		case 'svelte':
			return `<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";
  import { ${selfHosted ? 'PUBLIC_TRACEWAY_TOKEN, PUBLIC_TRACEWAY_URL' : 'PUBLIC_TRACEWAY_TOKEN'} } from "$env/static/public";

  let { children } = $props();

  setupTraceway({
    token: PUBLIC_TRACEWAY_TOKEN,${selfHosted ? '\n    baseUrl: PUBLIC_TRACEWAY_URL,' : ''}
  });
</script>

{@render children()}`;

		case 'vuejs':
			return `import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  token: ${viteToken},${selfHosted ? `\n  baseUrl: ${viteUrl},` : ''}
}));

app.mount("#app");`;

		case 'jquery':
			return `import $ from "jquery";
import { init } from "@tracewayapp/jquery";

// A bundled jQuery is not on window, so pass the one you import.
init({
  token: ${viteToken},${selfHosted ? `\n  baseUrl: ${viteUrl},` : ''}
  jQuery: $,
});

// Failed $.ajax() calls are captured automatically
// Distributed trace headers are injected into $.ajax() requests`;

		case 'flutter':
			return `import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

void main() {
  Traceway.run(
    token: const String.fromEnvironment('TRACEWAY_TOKEN'),${selfHosted ? "\n    baseUrl: const String.fromEnvironment('TRACEWAY_URL')," : ''}
    options: TracewayOptions(
      screenCapture: true,
      version: '1.0.0',
    ),
    child: MyApp(),
  );
}`;

		case 'android':
			return `import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            this,
            TracewayOptions(
                token = BuildConfig.TRACEWAY_TOKEN,${selfHosted ? '\n                baseUrl = BuildConfig.TRACEWAY_URL,' : ''}
                version = "1.0.0",
                screenCapture = true,
            ),
        )
    }
}`;

		case 'ios':
			return `import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        let info = Bundle.main.infoDictionary
        Traceway.initialize(
            token: info?["TracewayToken"] as? String ?? "",${selfHosted ? '\n            baseUrl: info?["TracewayURL"] as? String ?? "",' : ''}
            options: TracewayOptions(
                version: "1.0.0",
                screenCapture: true
            )
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}`;

		default:
			return '';
	}
}

function reactProvider(token: string, url: string, selfHosted: boolean, indent: string): string {
	if (!selfHosted) return `${indent}<TracewayProvider token={${token}}>`;
	return `${indent}<TracewayProvider\n${indent}  token={${token}}\n${indent}  baseUrl={${url}}\n${indent}>`;
}
