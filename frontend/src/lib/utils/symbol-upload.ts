// The Android steps of the Symbol Upload card (Connection page). The Traceway Gradle plugin uploads the build's R8
// mapping.txt; it reads the project's upload token from the environment (CI) or local.properties, never from a literal
// in build.gradle.kts, and needs the server only off Traceway Cloud, where it uploads by default.
import { selfHostedBaseUrl } from './connection-string';
import { SDK_VERSION, type StartConfigFile } from './sdk-start';

export const ANDROID_SYMBOL_UPLOAD_COMMAND =
	'./gradlew assembleRelease uploadReleaseTracewaySymbols';

/** local.properties with the upload token, and the app module's Gradle block that applies the plugin and reads it. */
export function getAndroidSymbolUploadFiles(
	uploadToken: string,
	backendUrl: string
): StartConfigFile[] {
	return [
		{
			title: 'local.properties',
			description:
				'At the project root, next to TRACEWAY_TOKEN; Android Studio keeps it out of version control. On CI, set the TRACEWAY_UPLOAD_TOKEN environment variable instead.',
			code: `TRACEWAY_UPLOAD_TOKEN=${uploadToken || 'YOUR_UPLOAD_TOKEN'}`,
			language: 'properties',
			secret: true
		},
		{
			title: 'app/build.gradle.kts',
			description:
				'Applies the plugin and reads the token from the environment, else local.properties. If the file already loads local.properties for the SDK token, keep one localProperties.',
			code: androidSymbolsGradle(selfHostedBaseUrl(backendUrl)),
			language: 'kotlin',
			secret: false
		}
	];
}

function androidSymbolsGradle(selfHostedUrl: string | null): string {
	const traceway = [
		'    uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN") ?: localProperties.getProperty("TRACEWAY_UPLOAD_TOKEN")',
		...(selfHostedUrl ? [`    url = "${selfHostedUrl}"`] : [])
	].join('\n');
	return `import java.util.Properties

plugins {
    id("com.android.application")
    id("com.tracewayapp.symbols") version "${SDK_VERSION}"
}

val localProperties = Properties().apply {
    rootProject.file("local.properties").takeIf { it.exists() }?.reader()?.use { load(it) }
}

android {
    buildTypes {
        release { isMinifyEnabled = true }
    }
}

traceway {
${traceway}
}`;
}
