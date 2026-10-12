package relay

import (
	"bufio"
	"net"
	"net/http"
	"strings"
	"time"
)

// statusWriter records the first status code written by the handler.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// statusWriterHF additionally passes through http.Hijacker and http.Flusher.
// WebSocket upgrade (gorilla) type-asserts the writer to http.Hijacker
// directly — Unwrap alone is not enough — so the middleware must expose
// Hijack whenever the underlying writer supports it, or upgrades break.
type statusWriterHF struct{ *statusWriter }

func (w statusWriterHF) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err == nil && w.status == 0 {
		// After hijack, the 101 response is written over the raw connection,
		// bypassing WriteHeader; record it here so the log shows status 101.
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

func (w statusWriterHF) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

// wrapStatusWriter preserves Hijacker+Flusher when the underlying writer has
// both (always the case behind nginx over HTTP/1.1); otherwise it falls back
// to the plain recorder.
func wrapStatusWriter(w http.ResponseWriter) (http.ResponseWriter, *statusWriter) {
	sw := &statusWriter{ResponseWriter: w}
	_, hijacker := w.(http.Hijacker)
	_, flusher := w.(http.Flusher)
	if hijacker && flusher {
		return statusWriterHF{sw}, sw
	}
	return sw, sw
}

// clientIP returns the first hop of X-Forwarded-For (the real client IP as
// seen by nginx), falling back to RemoteAddr without the port.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// accessLogMiddleware logs every HTTP request after it completes. WebSocket
// upgrade requests block in the handler for the connection's lifetime, so
// their http_req line (status 101, cost_ms = connection duration) is written
// when the socket closes; ws_open/ws_close carry the precise WS timing.
func (s *Server) accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped, sw := wrapStatusWriter(w)
		next.ServeHTTP(wrapped, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		s.accessLog.HTTPRequest(r.Method, r.URL.Path, status, time.Since(start), clientIP(r), r.UserAgent())
	})
}
