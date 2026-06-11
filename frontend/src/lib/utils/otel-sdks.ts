export type OtelSdkId = 'nodejs' | 'go' | 'python' | 'java' | 'dotnet' | 'php';

export type OtelSdk = {
	id: OtelSdkId;
	label: string;
	installCommand: string;
};

export const OTEL_SDKS: OtelSdk[] = [
	{
		id: 'nodejs',
		label: 'Node.js',
		installCommand:
			'npm install @opentelemetry/sdk-node @opentelemetry/exporter-trace-otlp-http @opentelemetry/exporter-metrics-otlp-http'
	},
	{
		id: 'go',
		label: 'Go',
		installCommand: 'go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp'
	},
	{
		id: 'python',
		label: 'Python',
		installCommand: 'pip install opentelemetry-sdk opentelemetry-exporter-otlp-proto-http'
	},
	{
		id: 'java',
		label: 'Java',
		installCommand: "implementation 'io.opentelemetry:opentelemetry-exporter-otlp'"
	},
	{
		id: 'dotnet',
		label: '.NET',
		installCommand: 'dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol'
	},
	{
		id: 'php',
		label: 'PHP',
		installCommand:
			'composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter'
	}
];
