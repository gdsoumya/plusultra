package api

import (
	"net/http"
	"strings"
	"testing"
)

const appOrigin = "http://localhost:8081"

func newCORSEnv(t *testing.T) *testEnv {
	t.Helper()
	return newEnv(t, httpsURL, func(d *Deps) { d.AllowedOrigins = []string{appOrigin} })
}

func (e *testEnv) do(t *testing.T, method, path, origin string, hdr map[string]string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, e.root+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func preflight(t *testing.T, e *testEnv, origin string) *http.Response {
	t.Helper()
	return e.do(t, http.MethodOptions, Prefix+"/plusultra.v1.AuthService/Register", origin, map[string]string{
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "authorization,connect-protocol-version,content-type", // browsers send them sorted and lowercase
	}, "")
}

func TestCORSPreflightAllowedOrigins(t *testing.T) {
	e := newCORSEnv(t)
	for _, origin := range []string{appOrigin, httpsURL} { // ALLOWED_ORIGINS and PUBLIC_URL's origin
		res := preflight(t, e, origin)
		h := res.Header
		if res.StatusCode != http.StatusNoContent || h.Get("Access-Control-Allow-Origin") != origin ||
			h.Get("Access-Control-Allow-Credentials") != "true" || h.Get("Access-Control-Max-Age") == "" ||
			!strings.Contains(h.Get("Access-Control-Allow-Methods"), http.MethodPost) {
			t.Fatalf("%s preflight = %d %v", origin, res.StatusCode, h)
		}
		for _, want := range []string{"connect-protocol-version", "authorization"} {
			if !strings.Contains(strings.ToLower(h.Get("Access-Control-Allow-Headers")), want) {
				t.Errorf("%s: Allow-Headers %q lacks %s", origin, h.Get("Access-Control-Allow-Headers"), want)
			}
		}
	}
}

func TestCORSDisallowedOriginGetsNoHeaders(t *testing.T) {
	e := newCORSEnv(t)
	for _, res := range []*http.Response{
		preflight(t, e, "https://evil.example"),
		e.do(t, http.MethodPost, Prefix+"/plusultra.v1.AuthService/Register", "https://evil.example",
			map[string]string{"Content-Type": "application/json"}, `{"username":"alice","password":"correct horse battery"}`),
	} {
		for k := range res.Header {
			if strings.HasPrefix(k, "Access-Control-") {
				t.Errorf("disallowed origin got %s: %v", k, res.Header)
			}
		}
	}
}

func TestCORSConnectCallFromBrowser(t *testing.T) {
	e := newCORSEnv(t)
	res := e.do(t, http.MethodPost, Prefix+"/plusultra.v1.AuthService/Register", appOrigin, map[string]string{
		"Content-Type": "application/json", "Connect-Protocol-Version": "1",
	}, `{"username":"alice","password":"correct horse battery"}`)
	h := res.Header
	if res.StatusCode != http.StatusOK || h.Get("Access-Control-Allow-Origin") != appOrigin ||
		h.Get("Access-Control-Allow-Credentials") != "true" || h.Get("Access-Control-Expose-Headers") == "" {
		t.Fatalf("register from browser = %d %v", res.StatusCode, h)
	}
}

// Docs, images and health are public reads: any origin, no credentials, and never a listed-origin
// echo. The app probes /healthz from its own origin before signing in.
func TestCORSPublicReads(t *testing.T) {
	e := newCORSEnv(t)
	for _, path := range []string{"/llms.txt", "/healthz", Prefix + "/healthz"} {
		res := e.do(t, http.MethodGet, path, "https://evil.example", nil, "")
		if res.StatusCode != http.StatusOK || res.Header.Get("Access-Control-Allow-Origin") != "*" ||
			res.Header.Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("%s cross-origin = %d %v", path, res.StatusCode, res.Header)
		}
	}
}

func TestParseOrigins(t *testing.T) {
	got, err := ParseOrigins(" http://localhost:8081, https://app.example/ ,")
	if err != nil || strings.Join(got, " ") != "http://localhost:8081 https://app.example" {
		t.Fatalf("ParseOrigins = %v, %v", got, err)
	}
	if got, err := ParseOrigins(""); err != nil || len(got) != 0 {
		t.Fatalf("empty = %v, %v", got, err)
	}
	for _, bad := range []string{"*", "https://*.example", "localhost:8081", "https://app.example/path", "https://app.example?x=1"} {
		if _, err := ParseOrigins(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
