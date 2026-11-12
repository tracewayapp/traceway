package main

import "github.com/tracewayapp/traceway/cli/v2/internal/state"

// resolveProfileName picks the effective profile name using the standard
// 3-tier precedence: --profile flag > st.CurrentProfile > "default".
func resolveProfileName(st *state.State) string {
	if flagProfile != "" {
		return flagProfile
	}
	if st.CurrentProfile != "" {
		return st.CurrentProfile
	}
	return "default"
}
