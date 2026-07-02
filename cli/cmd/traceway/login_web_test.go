package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/cli/internal/state"
)

func TestLoginWeb_deviceFlow_success(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/device/authorize":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"AAAA-BBBB","verification_uri":"http://x/device","verification_uri_complete":"http://x/device?user_code=AAAA-BBBB","expires_in":600,"interval":1}`))
		case "/api/auth/device/token":
			polls++
			if polls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":900,"email":"dev@example.com"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	_, _, err := runCmd(t, "", "login", "--url", srv.URL, "--no-browser")
	if err != nil {
		t.Fatalf("login web: %v", err)
	}

	st, _ := state.Load()
	sp, ok := st.Profiles["default"]
	if !ok {
		t.Fatal("default profile not saved")
	}
	if sp.CredentialKind != state.KindDevice {
		t.Errorf("kind = %q, want device", sp.CredentialKind)
	}
	if sp.JWT != "at" {
		t.Errorf("jwt = %q", sp.JWT)
	}
	if sp.RefreshToken != "rt" {
		t.Errorf("refresh token = %q", sp.RefreshToken)
	}
	if sp.TokenExpiresAt == 0 {
		t.Error("token expiry not recorded")
	}
}
