package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var export = fstest.MapFS{
	"index.html":                        {Data: []byte("<html>app shell</html>")},
	"favicon.ico":                       {Data: []byte("\x00\x00\x01\x00icon")},
	"_expo/static/js/web/entry-ab12.js": {Data: []byte("console.log(1)")},
	"assets/icons.9f8e.ttf":             {Data: []byte("font")},
}

func serve(t *testing.T, h http.Handler, method, path string) *http.Response {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	req, err := http.NewRequestWithContext(t.Context(), method, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestIndexAndFallback(t *testing.T) {
	h := newHandler(export)
	for _, p := range []string{"/", "/settings/server", "/no/such/file.js"} {
		res := serve(t, h, http.MethodGet, p)
		if res.StatusCode != http.StatusOK || body(t, res) != "<html>app shell</html>" ||
			res.Header.Get("Content-Type") != "text/html; charset=utf-8" || res.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("GET %s = %d %v", p, res.StatusCode, res.Header)
		}
	}
	if res := serve(t, h, http.MethodHead, "/"); res.StatusCode != http.StatusOK || body(t, res) != "" {
		t.Errorf("HEAD / = %d", res.StatusCode)
	}
}

func TestAssets(t *testing.T) {
	h := newHandler(export)
	const immutable = "public, max-age=31536000, immutable"
	for _, tc := range []struct{ path, typ, cache string }{
		{"/_expo/static/js/web/entry-ab12.js", "text/javascript; charset=utf-8", immutable},
		{"/assets/icons.9f8e.ttf", "font/ttf", immutable},
		{"/favicon.ico", "image/", ""}, // x-icon or vnd.microsoft.icon, depending on the host's mime table
	} {
		res := serve(t, h, http.MethodGet, tc.path)
		if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), tc.typ) || res.Header.Get("Cache-Control") != tc.cache {
			t.Errorf("GET %s = %d %v", tc.path, res.StatusCode, res.Header)
		}
	}
}

func TestReservedPathsAndMethods(t *testing.T) {
	h := newHandler(export)
	for _, p := range []string{"/api/v1/plusultra.v1.AuthService/Login", "/api/v2/x", "/mcp", "/healthz", "/llms.txt", "/llms/program-language.md"} {
		if res := serve(t, h, http.MethodGet, p); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.StatusCode)
		}
	}
	if res := serve(t, h, http.MethodPost, "/"); res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("POST / = %d %v", res.StatusCode, res.Header)
	}
}

// Without an export (only the .gitkeep) the placeholder page is served.
func TestPlaceholderWithoutExport(t *testing.T) {
	h := newHandler(fstest.MapFS{".gitkeep": {}})
	res := serve(t, h, http.MethodGet, "/anything")
	if res.StatusCode != http.StatusOK || !strings.Contains(body(t, res), "The server is running") {
		t.Errorf("placeholder = %d", res.StatusCode)
	}
}

// Handler reads the embedded dist directory, which holds the export or only .gitkeep.
func TestHandlerEmbeds(t *testing.T) {
	res := serve(t, Handler(), http.MethodGet, "/")
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("GET / = %d %v", res.StatusCode, res.Header)
	}
}
