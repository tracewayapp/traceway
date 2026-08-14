package synthetics

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/netguard"
)

// maxProbeBodyBytes caps how much of the response body is read for
// assertions; probes must never buffer arbitrarily large downloads.
const maxProbeBodyBytes = 1 << 20

func probeHttp(ctx context.Context, check *models.SyntheticCheck) Outcome {
	outcome := Outcome{Status: models.CheckResultDown, TlsDaysRemaining: -1}

	var cfg models.HttpCheckConfig
	if err := json.Unmarshal(check.Config, &cfg); err != nil {
		outcome.ErrorMsg = "invalid check config: " + err.Error()
		return outcome
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if cfg.Body != "" {
		body = strings.NewReader(cfg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, body)
	if err != nil {
		outcome.ErrorMsg = "invalid request: " + err.Error()
		return outcome
	}
	for name, value := range cfg.Headers {
		if strings.TrimSpace(name) == "" {
			continue
		}
		req.Header.Set(name, value)
	}
	if cfg.Body != "" && cfg.BodyContentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", cfg.BodyContentType)
	}
	switch cfg.Auth.Type {
	case "basic":
		req.SetBasicAuth(cfg.Auth.Username, cfg.Auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+cfg.Auth.Token)
	case "header":
		if cfg.Auth.HeaderName != "" {
			req.Header.Set(cfg.Auth.HeaderName, cfg.Auth.HeaderValue)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Traceway-Synthetics/1.0")
	}

	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: cfg.SkipTlsVerify},
		DisableKeepAlives: true,
	}
	if !AllowPrivateTargets() {
		// Guard at dial time so a hostname cannot pass save-time validation
		// and be re-pointed at a private address (DNS rebinding).
		transport.DialContext = netguard.GuardedDialContext(EffectiveTimeout(check))
	}
	client := &http.Client{Transport: transport}
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
		outcome.ErrorMsg = err.Error()
		return outcome
	}
	defer resp.Body.Close()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
	outcome.LatencyMs = float64(time.Since(started).Microseconds()) / 1000
	outcome.StatusCode = resp.StatusCode
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		outcome.TlsDaysRemaining = daysUntil(resp.TLS.PeerCertificates[0].NotAfter)
	}
	if readErr != nil {
		outcome.ErrorMsg = "failed to read response body: " + readErr.Error()
		return outcome
	}

	if failures := evaluateHttpAssertions(&cfg.Assertions, resp.StatusCode, bodyBytes, outcome.LatencyMs, outcome.TlsDaysRemaining); len(failures) > 0 {
		outcome.ErrorMsg = strings.Join(failures, "; ")
		return outcome
	}

	outcome.Status = models.CheckResultUp
	return outcome
}

func evaluateHttpAssertions(assertions *models.HttpAssertions, statusCode int, body []byte, latencyMs float64, tlsDaysRemaining int) []string {
	var failures []string

	if len(assertions.StatusCodes) > 0 {
		matched := false
		for _, code := range assertions.StatusCodes {
			if statusCode == code {
				matched = true
				break
			}
		}
		if !matched {
			failures = append(failures, fmt.Sprintf("status code %d not in expected set %v", statusCode, assertions.StatusCodes))
		}
	} else if statusCode >= 400 {
		failures = append(failures, fmt.Sprintf("status code %d", statusCode))
	}

	if assertions.BodyContains != "" && !strings.Contains(string(body), assertions.BodyContains) {
		failures = append(failures, fmt.Sprintf("body does not contain %q", assertions.BodyContains))
	}
	if assertions.BodyRegex != "" {
		re, err := regexp.Compile(assertions.BodyRegex)
		if err != nil {
			failures = append(failures, "invalid body regex: "+err.Error())
		} else if !re.Match(body) {
			failures = append(failures, fmt.Sprintf("body does not match regex %q", assertions.BodyRegex))
		}
	}
	if assertions.MaxResponseTimeMs > 0 && latencyMs > float64(assertions.MaxResponseTimeMs) {
		failures = append(failures, fmt.Sprintf("response time %.0fms exceeds %dms", latencyMs, assertions.MaxResponseTimeMs))
	}
	if assertions.TlsMinDaysRemaining > 0 {
		if tlsDaysRemaining < 0 {
			failures = append(failures, "TLS expiry assertion set but the connection was not TLS")
		} else if tlsDaysRemaining < assertions.TlsMinDaysRemaining {
			failures = append(failures, fmt.Sprintf("TLS certificate expires in %d days (minimum %d)", tlsDaysRemaining, assertions.TlsMinDaysRemaining))
		}
	}

	return failures
}

func daysUntil(t time.Time) int {
	days := int(time.Until(t).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}
