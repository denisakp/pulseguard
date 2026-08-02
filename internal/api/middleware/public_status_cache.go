package middleware

import (
	"net/http"
	"strconv"
)

// PublicStatusCacheRecorder is the minimal surface the middleware needs to
// emit hit/miss counters. Defined inline to avoid pulling internal/metrics
// into the middleware package.
type PublicStatusCacheRecorder interface {
	RecordHit()
	RecordMiss()
}

// PublicStatusCache adds short-lived HTTP cache headers for the public status
// page JSON endpoints. It lets reverse proxies and browsers
// serve cached responses for `maxAgeSeconds` and revalidate in the background
// for an additional `staleSeconds` window.
//
// When rec is non-nil the middleware also classifies the request as a cache
// hit (conditional headers present) or miss (no conditional headers) — this
// is the signal the SC-007 95% claim is measured from.
//
// Default tuning: max-age=15, stale-while-revalidate=30. Per FR-026 the
// public payload must not be older than 60s, which the upstream cron + cache
// chain enforces; this middleware only governs HTTP-layer caching.
func PublicStatusCache(maxAgeSeconds, staleSeconds int, rec PublicStatusCacheRecorder) func(http.Handler) http.Handler {
	if maxAgeSeconds <= 0 {
		maxAgeSeconds = 15
	}
	if staleSeconds < 0 {
		staleSeconds = 0
	}
	cc := "public, max-age=" + strconv.Itoa(maxAgeSeconds) +
		", stale-while-revalidate=" + strconv.Itoa(staleSeconds)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rec != nil {
				if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
					rec.RecordHit()
				} else {
					rec.RecordMiss()
				}
			}
			w.Header().Set("Cache-Control", cc)
			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(w, r)
		})
	}
}
