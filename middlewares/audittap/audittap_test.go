package audittap

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fixedClock time.Time

func (c fixedClock) Now() time.Time {
	return time.Time(c)
}

func TestAuditTap_noop(t *testing.T) {
	clock = fixedClock(time.Now())

	cfg := AuditTapConfig{}
	tap, err := NewAuditTap(cfg, "backend1")
	assert.NoError(t, err)

	req := httptest.NewRequest("", "/a/b/c?d=1&e=2", nil)
	req.RemoteAddr = "101.102.103.104:1234"
	req.Host = "example.co.uk"
	req.Header.Set("Request-ID", "R123")
	req.Header.Set("Session-ID", "S123")
	res := httptest.NewRecorder()

	tap.ServeHTTP(res, req, http.NotFoundHandler().(http.HandlerFunc))

	sink := tap.AuditSinks[0].(*noopAuditSink)
	assert.Equal(t,
		Summary{
			RequestSummary{
				"backend1",
				"Traefik1",
				"example.co.uk",
				"GET",
				"/a/b/c",
				"d=1&e=2",
				"101.102.103.104:1234",
				map[string]interface{}{"requestId": "R123", "sessionId": "S123"},
				clock.Now(),
			},
			ResponseSummary{
				"", "",
				404,
				map[string]interface{}{"xContentTypeOptions": "nosniff", "contentType": "text/plain; charset=utf-8"},
				19,
				clock.Now(),
			},
		},
		sink.Summary)
}
