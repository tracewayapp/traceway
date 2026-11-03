package sdkconfig

// SupportsFramework matches the dashboard's browser-only Data Collection controls.
func SupportsFramework(framework string) bool {
	switch framework {
	case "react", "svelte", "vuejs", "jquery":
		return true
	default:
		return false
	}
}
