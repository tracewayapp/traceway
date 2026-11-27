package connstr

import "testing"

// Every spelling the Traceway SDKs accept.
func TestParseContractTable(t *testing.T) {
	cases := []struct{ in, token, baseURL string }{
		{"tok@https://cloud.tracewayapp.com", "tok", "https://cloud.tracewayapp.com"},
		{"tok@https://cloud.tracewayapp.com/", "tok", "https://cloud.tracewayapp.com"},
		{"tok@https://cloud.tracewayapp.com/api", "tok", "https://cloud.tracewayapp.com"},
		{"tok@https://cloud.tracewayapp.com/api/", "tok", "https://cloud.tracewayapp.com"},
		{"tok@https://cloud.tracewayapp.com/api/report", "tok", "https://cloud.tracewayapp.com"},
		{"tok@https://cloud.tracewayapp.com/api/report/", "tok", "https://cloud.tracewayapp.com"},
		{"tok@cloud.tracewayapp.com", "tok", "https://cloud.tracewayapp.com"},
		{"tok@cloud.tracewayapp.com/api/report", "tok", "https://cloud.tracewayapp.com"},
		{"  tok@https://cloud.tracewayapp.com/api/report  ", "tok", "https://cloud.tracewayapp.com"},
		{"tok@http://localhost:8082/api/report", "tok", "http://localhost:8082"},
		{"tok@localhost:8082", "tok", "http://localhost:8082"},
		{"tok@http://api", "tok", "http://api"},
		{"tok@http://api/report", "tok", "http://api/report"},
		{"tok@http://api/api/report?x", "tok", "http://api"},
		{"tok@https://corp.example/traceway/api/report", "tok", "https://corp.example/traceway"},
		{"tok@https://cloud.tracewayapp.com/api/report?x=1", "tok", "https://cloud.tracewayapp.com"},
	}
	for _, tc := range cases {
		token, baseURL, err := Parse(tc.in)
		if err != nil || token != tc.token || baseURL != tc.baseURL {
			t.Errorf("Parse(%q) = (%q, %q, %v), want (%q, %q)", tc.in, token, baseURL, err, tc.token, tc.baseURL)
		}
	}
}

func TestParseRejectsContractTableErrors(t *testing.T) {
	for _, in := range []string{"@https://cloud.tracewayapp.com", "tok", "tok@", "tok@ftp://host", "", "   "} {
		if token, baseURL, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = (%q, %q), want an error", in, token, baseURL)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://cloud.tracewayapp.com/api/report", "https://cloud.tracewayapp.com"},
		{"cloud.tracewayapp.com/", "https://cloud.tracewayapp.com"},
		{"127.0.0.1:8082/api", "http://127.0.0.1:8082"},
		{"10.0.2.2:8082", "http://10.0.2.2:8082"},
		{"[::1]:8082/api/report/", "http://[::1]:8082"},
		{"LOCALHOST:8082", "http://LOCALHOST:8082"},
		{"traceway.internal:8082", "https://traceway.internal:8082"},
		{"https://corp.example/traceway/", "https://corp.example/traceway"},
		{"https://corp.example/traceway/api/api", "https://corp.example/traceway/api"},
		{"https://corp.example#frag", "https://corp.example"},
	}
	for _, tc := range cases {
		if got, err := NormalizeBaseURL(tc.in); err != nil || got != tc.want {
			t.Errorf("NormalizeBaseURL(%q) = (%q, %v), want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "ftp://host", "http://:8080", "https://", "  "} {
		if got, err := NormalizeBaseURL(in); err == nil {
			t.Errorf("NormalizeBaseURL(%q) = %q, want an error", in, got)
		}
	}
}

func TestIsCloud(t *testing.T) {
	for in, want := range map[string]bool{
		"https://cloud.tracewayapp.com":      true,
		"https://Cloud.TracewayApp.com":      true,
		"https://cloud.tracewayapp.com:443":  true,
		"http://cloud.tracewayapp.com":       false,
		"https://cloud.tracewayapp.com/sub":  false,
		"https://traceway.acme.test":         false,
		"http://localhost:8082":              false,
		"https://cloud.tracewayapp.com.evil": false,
	} {
		if got := IsCloud(in); got != want {
			t.Errorf("IsCloud(%q) = %v, want %v", in, got, want)
		}
	}
}
