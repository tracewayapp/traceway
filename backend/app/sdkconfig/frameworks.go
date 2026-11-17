package sdkconfig

// IsBrowserFramework names the projects instrumented by the OpenTelemetry browser SDK, whose performance events
// the ingest turns into metrics.
func IsBrowserFramework(framework string) bool {
	switch framework {
	case "react", "svelte", "vuejs", "jquery":
		return true
	default:
		return false
	}
}

// SupportsFramework names the projects whose SDK takes remote collection settings over OpAMP: the browser SDKs and
// the Flutter SDK, both built on OpenTelemetry. It matches the dashboard's Data Collection controls.
func SupportsFramework(framework string) bool {
	return IsBrowserFramework(framework) || framework == "flutter"
}
