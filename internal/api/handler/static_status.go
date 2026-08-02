// Package handler — StaticStatusHandler serves the public bundle on the
// custom domain with two server-side injections:
//  1. <meta name="x-ogoune-license"> reflecting the current edition.
//  2. <title> reflecting the current verdict (e.g. "Acme Status — All
//     Systems Operational"). Falls back to a generic label when the
//     verdict cannot be computed.
//
// The bundle itself (status.html + assets) is read from STATIC_DIR. Asset
// requests (anything that doesn't end with "status.html" and exists on
// disk) are streamed as-is. HTML navigations are templated.
package handler

import (
	"bytes"
	"context"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisakp/ogoune/internal/dto"
	"github.com/denisakp/ogoune/internal/ee/license"
)

type StaticStatusHandler struct {
	staticDir string
	svc       PublicStatusProvider
}

func NewStaticStatusHandler(staticDir string, svc PublicStatusProvider) *StaticStatusHandler {
	return &StaticStatusHandler{staticDir: staticDir, svc: svc}
}

// ServeHTTP routes the request inside the public bundle. Anything that maps
// to a file on disk is streamed verbatim; SPA navigations (no extension,
// non-asset path) are answered with the templated status.html.
func (h *StaticStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := filepath.Clean(r.URL.Path)
	if clean == "/" || clean == "" {
		h.serveTemplated(w, r)
		return
	}
	// Asset request: try to serve from STATIC_DIR.
	target := filepath.Join(h.staticDir, clean)
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return
	}
	// SPA fallback.
	h.serveTemplated(w, r)
}

func (h *StaticStatusHandler) serveTemplated(w http.ResponseWriter, r *http.Request) {
	htmlPath := filepath.Join(h.staticDir, "status.html")
	raw, err := os.ReadFile(htmlPath)
	if err != nil {
		http.Error(w, "status bundle missing", http.StatusInternalServerError)
		return
	}
	rendered := h.inject(r.Context(), raw)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(rendered)
}

// inject mutates status.html to set the title, the license meta tag, and
// (when configured) the Umami analytics script tag.
func (h *StaticStatusHandler) inject(ctx context.Context, raw []byte) []byte {
	title := h.computeTitle(ctx)
	meta := licenseMetaTag()
	umamiTag := h.computeUmamiScript(ctx)
	out := raw

	out = replaceTag(out, []byte("<title>"), []byte("</title>"), []byte(title))

	// Inject (or replace) the license meta tag right after <head>.
	out = ensureLicenseMeta(out, meta)

	// Inject the Umami <script> tag right before </head> when the operator
	// has set umami_website_id on the status page settings.
	if umamiTag != "" {
		out = insertBeforeHeadClose(out, umamiTag)
	}
	return out
}

func (h *StaticStatusHandler) computeTitle(ctx context.Context) string {
	if h.svc == nil {
		return "Status Page"
	}
	snapshot, err := h.svc.GetCurrent(ctx)
	if err != nil || snapshot == nil {
		return "Status Page"
	}
	brand := snapshot.Branding.Name
	if brand == "" {
		brand = "Status"
	}
	verdict := verdictLabel(snapshot.Verdict)
	if verdict == "" {
		return brand
	}
	return brand + " — " + verdict
}

func verdictLabel(v dto.PublicVerdict) string {
	if v.Label != "" {
		return v.Label
	}
	switch v.Status {
	case dto.VerdictOperational:
		return "All Systems Operational"
	case dto.VerdictPartialDegradation:
		return "Partial Degradation"
	case dto.VerdictMajorOutage:
		return "Major Outage"
	}
	return ""
}

// computeUmamiScript returns the analytics script tag to inject, or empty
// when the website id is not configured.
func (h *StaticStatusHandler) computeUmamiScript(ctx context.Context) string {
	if h.svc == nil {
		return ""
	}
	snapshot, err := h.svc.GetCurrent(ctx)
	if err != nil || snapshot == nil {
		return ""
	}
	id := strings.TrimSpace(snapshot.Branding.UmamiWebsiteID)
	if id == "" {
		return ""
	}
	src := strings.TrimSpace(snapshot.Branding.UmamiScriptURL)
	if src == "" {
		src = "https://cloud.umami.is/script.js"
	}
	return `<script defer src="` + html.EscapeString(src) +
		`" data-website-id="` + html.EscapeString(id) + `"></script>`
}

// insertBeforeHeadClose appends the supplied tag just before </head>.
func insertBeforeHeadClose(in []byte, tag string) []byte {
	idx := bytes.Index(in, []byte("</head>"))
	if idx < 0 {
		return in
	}
	var buf bytes.Buffer
	buf.Write(in[:idx])
	buf.WriteString("\n  ")
	buf.WriteString(tag)
	buf.WriteString("\n")
	buf.Write(in[idx:])
	return buf.Bytes()
}

func licenseMetaTag() string {
	if license.IsEnterprise() {
		return `<meta name="x-ogoune-license" content="enterprise-suppressed">`
	}
	return `<meta name="x-ogoune-license" content="community">`
}

// replaceTag swaps the content between start and end markers (open + close
// tags). When the markers are absent the input is returned unchanged.
func replaceTag(in, open, close, body []byte) []byte {
	openIdx := bytes.Index(in, open)
	if openIdx < 0 {
		return in
	}
	closeIdx := bytes.Index(in[openIdx:], close)
	if closeIdx < 0 {
		return in
	}
	closeIdx += openIdx
	var buf bytes.Buffer
	buf.Write(in[:openIdx+len(open)])
	buf.WriteString(html.EscapeString(string(body)))
	buf.Write(in[closeIdx:])
	return buf.Bytes()
}

// ensureLicenseMeta replaces an existing x-ogoune-license tag or injects a
// fresh one right after the opening <head>.
func ensureLicenseMeta(in []byte, tag string) []byte {
	const marker = `name="x-ogoune-license"`
	if i := strings.Index(string(in), marker); i >= 0 {
		// Replace the whole existing tag.
		start := bytes.LastIndex(in[:i], []byte("<meta"))
		if start < 0 {
			return in
		}
		end := bytes.IndexByte(in[start:], '>')
		if end < 0 {
			return in
		}
		end += start
		var buf bytes.Buffer
		buf.Write(in[:start])
		buf.WriteString(tag)
		buf.Write(in[end+1:])
		return buf.Bytes()
	}
	// Insert after <head>.
	headIdx := bytes.Index(in, []byte("<head>"))
	if headIdx < 0 {
		return in
	}
	headIdx += len("<head>")
	var buf bytes.Buffer
	buf.Write(in[:headIdx])
	buf.WriteString("\n  ")
	buf.WriteString(tag)
	buf.Write(in[headIdx:])
	return buf.Bytes()
}
