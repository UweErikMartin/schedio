package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// LoggingMiddleware logs CalDAV/HTTP protocol flow at different klog verbosity
// levels:
//
//	-v=1   one line per request: METHOD URL status duration remote-addr
//	-v=2   + request headers + response status line + response headers
//	-v=3   + full request body + full response body (UTF-8 safe hex-dump for binary)
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// --- capture request body so it can be logged and re-fed to handler ---
		var reqBody []byte
		if klog.V(3).Enabled() && r.Body != nil && r.ContentLength != 0 {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				klog.Warningf("middleware: failed to read request body: %v", err)
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// --- wrap response writer to capture status, headers and body ---
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)

		dur := time.Since(start)

		if !klog.V(1).Enabled() {
			return
		}

		// V(1): single summary line
		klog.Infof("%-8s %-50s %d  %s  %s",
			r.Method, r.URL.RequestURI(), lrw.status, dur.Round(time.Millisecond), r.RemoteAddr)

		if !klog.V(2).Enabled() {
			return
		}

		// V(2): request headers
		var sb strings.Builder
		fmt.Fprintf(&sb, "\n┌─ REQUEST  %s %s %s\n", r.Method, r.URL.RequestURI(), r.Proto)
		for name, vals := range r.Header {
			fmt.Fprintf(&sb, "│  %s: %s\n", name, strings.Join(vals, ", "))
		}

		// V(2): response status + headers
		fmt.Fprintf(&sb, "├─ RESPONSE %d\n", lrw.status)
		for name, vals := range lrw.Header() {
			fmt.Fprintf(&sb, "│  %s: %s\n", name, strings.Join(vals, ", "))
		}

		if klog.V(3).Enabled() {
			// V(3): bodies
			if len(reqBody) > 0 {
				fmt.Fprintf(&sb, "├─ REQ BODY (%d bytes)\n│  %s\n",
					len(reqBody), indentBody(reqBody))
			}
			if lrw.body.Len() > 0 {
				fmt.Fprintf(&sb, "├─ RSP BODY (%d bytes)\n│  %s\n",
					lrw.body.Len(), indentBody(lrw.body.Bytes()))
			}
		}

		sb.WriteString("└─")
		klog.Info(sb.String())
	})
}

// indentBody renders a body for log output: printable UTF-8 text is indented,
// binary data is hex-dumped (truncated at 4 KB to keep logs readable).
func indentBody(b []byte) string {
	const maxBytes = 4096
	truncated := false
	if len(b) > maxBytes {
		b = b[:maxBytes]
		truncated = true
	}
	text := string(b)
	if isTextual(b) {
		lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		for i, l := range lines {
			lines[i] = "│  " + l
		}
		result := strings.Join(lines, "\n")
		if truncated {
			result += "\n│  …(truncated)"
		}
		return result
	}
	// binary: hex dump
	var sb strings.Builder
	for i := 0; i < len(b); i += 16 {
		end := i + 16
		if end > len(b) {
			end = len(b)
		}
		fmt.Fprintf(&sb, "%08x: % x\n│  ", i, b[i:end])
	}
	if truncated {
		sb.WriteString("…(truncated)")
	}
	return sb.String()
}

func isTextual(b []byte) bool {
	for _, c := range b {
		if c < 0x09 || (c > 0x0d && c < 0x20 && c != 0x1b) {
			return false
		}
	}
	return true
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.status = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if klog.V(3).Enabled() {
		lrw.body.Write(b)
	}
	return lrw.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController access the underlying writer.
func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter { return lrw.ResponseWriter }
