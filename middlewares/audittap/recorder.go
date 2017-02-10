package audittap

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type recorderResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func NewAuditResponseWriter(w http.ResponseWriter) AuditResponseWriter {
	return &recorderResponseWriter{w, 0, 0}
}

func (r *recorderResponseWriter) WriteHeader(code int) {
	r.ResponseWriter.WriteHeader(code)
	r.status = code
}

func (r *recorderResponseWriter) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (r *recorderResponseWriter) Write(b []byte) (int, error) {
	if r.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		r.WriteHeader(http.StatusOK)
	}
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, err
}

func (r *recorderResponseWriter) CloseNotify() <-chan bool {
	return r.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (r *recorderResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

func (r *recorderResponseWriter) Summarise() ResponseSummary {
	return ResponseSummary{
		r.status,
		flattenHeaders(r.Header()),
		r.size,
		clock.Now(),
	}
}
