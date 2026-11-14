package sdkconfig

// SupportsFramework names the projects whose SDK takes remote collection settings over OpAMP: the browser SDKs and
// the Flutter SDK, both built on OpenTelemetry. It matches the dashboard's Data Collection controls.
func SupportsFramework(framework string) bool {
	switch framework {
	case "react", "svelte", "vuejs", "jquery", "flutter":
		return true
	default:
		return false
	}
}
