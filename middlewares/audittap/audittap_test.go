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

	req := httptest.NewRequest("", "localhost:9092", nil)
	req.Header.Set("Foo", "123")
	res := httptest.NewRecorder()

	tap.ServeHTTP(res, req, http.NotFoundHandler().(http.HandlerFunc))

	sink := tap.AuditSink.(*noopAuditSink)
	assert.Equal(t,
		Summary{
			RequestSummary{
				"GET",
				"localhost:9092",
				map[string]interface{}{"foo": "123"},
				clock.Now(),
			},
			ResponseSummary{
				404,
				map[string]interface{}{"xContentTypeOptions": "nosniff", "contentType": "text/plain; charset=utf-8"},
				19,
				clock.Now(),
			},
		},
		sink.Summary)
}
