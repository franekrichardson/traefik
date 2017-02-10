package middlewares

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuditResponseWriter_no_body(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := NewAuditResponseWriter(recorder)
	w.WriteHeader(204)
	assert.Equal(t, 204, w.Summarise().Status)
	assert.Equal(t, 0, w.Summarise().Size)
}

func TestAuditResponseWriter_with_body(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := NewAuditResponseWriter(recorder)
	w.WriteHeader(200)
	w.Write([]byte("hello"))
	w.Write([]byte("world"))
	assert.Equal(t, 200, w.Summarise().Status)
	assert.Equal(t, 10, w.Summarise().Size)
}

func TestAuditResponseWriter_headers(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := NewAuditResponseWriter(recorder)

	// hop-by-hop headers should be dropped
	w.Header().Set("Keep-Alive", "true")
	w.Header().Set("Connection", "1")
	w.Header().Set("Proxy-Authenticate", "1")
	w.Header().Set("Proxy-Authorization", "1")
	w.Header().Set("TE", "1")
	w.Header().Set("Trailers", "1")
	w.Header().Set("Transfer-Encoding", "1")
	w.Header().Set("Upgrade", "1")

	// other headers should be retainedd
	w.Header().Set("Content-Length", "123")
	w.Header().Set("Request-ID", "abc123")
	w.Header().Add("Cookie", "a=1")
	w.Header().Add("Cookie", "b=2")

	assert.Equal(t, map[string]interface{}{
		"contentLength": "123",
		"requestId":     "abc123",
		"cookie":        []string{"a=1", "b=2"},
	}, w.Summarise().Header)
}
