package main

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

var mobileFrameworks = map[string]bool{"flutter": true, "android": true, "ios": true, "react-native": true}

// setupPageURL is the dashboard page that shows the proposal for approval and, after it, the deployment commands. Its
// mode=add keeps it open for an organization that already has projects.
func setupPageURL(base string, organizationID int) string {
	if organizationID > 0 {
		return fmt.Sprintf("%s/setup?mode=add&organizationId=%d", base, organizationID)
	}
	return base + "/setup?mode=add"
}

type mobileServerURL struct {
	project string
	url     string
}

// writesServerURL reports whether the project received a separate URL assignment or a legacy connection string.
func writesServerURL(p client.SetupPlanProject, writes []setupEnvWrite) bool {
	return (len(writes) > 1 && writes[1].result != "") || (setupEnvFormat(p) == envFormatConnectionString && len(writes) == 1)
}

type serverReach struct {
	loopback  bool
	cleartext bool
	port      string
}

func serverReachability(raw string) serverReach {
	u, err := url.Parse(raw)
	if err != nil {
		return serverReach{}
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	port := u.Port()
	if port == "" && u.Scheme == "http" {
		port = "80"
	}
	return serverReach{
		loopback:  host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsLoopback() || ip.IsUnspecified())),
		cleartext: u.Scheme == "http",
		port:      port,
	}
}

// renderLocalServerNote explains, once, why a mobile app on an emulator or a phone may not reach the server URL that
// was written: a loopback host is the device itself, and both mobile platforms block plain http by default. The
// written value stays as it is; a desktop or JVM test run reaches it fine.
func renderLocalServerNote(w io.Writer, written []mobileServerURL) {
	var names []string
	var loopback, cleartext []string
	ports := map[string]bool{}
	for _, m := range written {
		reach := serverReachability(m.url)
		if !reach.loopback && !reach.cleartext {
			continue
		}
		names = append(names, fmt.Sprintf("%s (%s)", m.project, m.url))
		if reach.loopback {
			loopback = append(loopback, m.url)
			if reach.port != "" {
				ports[reach.port] = true
			}
		}
		if reach.cleartext {
			cleartext = append(cleartext, m.url)
		}
	}
	if len(names) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nNote: the server URL written for %s works for a desktop or JVM test run; on an emulator or a phone:\n", strings.Join(names, ", "))
	if len(loopback) > 0 {
		reverse := ""
		if len(ports) == 1 {
			for port := range ports {
				reverse = fmt.Sprintf(" (or run `adb reverse tcp:%s tcp:%s`)", port, port)
			}
		}
		_, _ = fmt.Fprintf(w, "  - localhost is the device itself. An Android emulator reaches this computer as 10.0.2.2%s, a phone needs the computer's LAN address; the iOS Simulator shares the computer's localhost.\n", reverse)
	}
	if len(cleartext) > 0 {
		_, _ = fmt.Fprintln(w, "  - Plain http:// is blocked by default: Android 9+ refuses cleartext traffic unless a network security config allows the host, and iOS App Transport Security needs an NSAppTransportSecurity exception in Info.plist.")
	}
	_, _ = fmt.Fprintln(w, "  Point such a test build at a reachable address in its own build config; the written file is left as it is.")
}
