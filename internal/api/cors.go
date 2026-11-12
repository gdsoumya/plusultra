package api

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
)

const corsMaxAge = 7200 // seconds a browser may cache a preflight

// ParseOrigins splits a comma-separated list of browser origins, each
// scheme://host[:port] with nothing after the host. Wildcards are refused
// because the RPC routes answer with credentials.
func ParseOrigins(s string) ([]string, error) {
	var out []string
	for o := range strings.SplitSeq(s, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Scheme == "" || u.Host == "" || strings.Trim(u.Path, "/") != "" || u.RawQuery != "" ||
			u.Fragment != "" || u.User != nil || strings.Contains(u.Host, "*") {
			return nil, fmt.Errorf("origin %q must be scheme://host[:port]", o)
		}
		out = append(out, u.Scheme+"://"+u.Host)
	}
	return out, nil
}

// browserCORS allows credentialed Connect and gRPC-web calls from the listed
// origins only; an empty list allows none.
func browserCORS(origins []string) *cors.Cors {
	return cors.New(cors.Options{
		AllowOriginFunc:  func(origin string) bool { return slices.Contains(origins, origin) },
		AllowedMethods:   connectcors.AllowedMethods(),
		AllowedHeaders:   append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders:   connectcors.ExposedHeaders(),
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	})
}

// publicCORS lets any page read docs and images without credentials.
func publicCORS() *cors.Cors {
	return cors.New(cors.Options{AllowedMethods: []string{http.MethodGet}, MaxAge: corsMaxAge})
}

// originOf returns the origin of publicURL, or nothing when it is unset or not a URL.
func originOf(publicURL string) []string {
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return []string{u.Scheme + "://" + u.Host}
}
