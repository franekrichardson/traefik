package audittap

import (
	"github.com/containous/traefik/types"
	"net/http"
	"time"
)

//-------------------------------------------------------------------------------------------------

type RequestSummary struct {
	Source    string                 `json:"auditSource,omitempty"`
	AuditType string                 `json:"auditType,omitempty"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Header    map[string]interface{} `json:"header"` // contains strings or string slices
	BeganAt   time.Time              `json:"beganAt"`
}

type ResponseSummary struct {
	Source      string                 `json:"auditSource,omitempty"`
	AuditType   string                 `json:"auditType,omitempty"`
	Status      int                    `json:"status"`
	Header      map[string]interface{} `json:"header"` // contains strings or string slices
	Size        int                    `json:"size"`
	CompletedAt time.Time              `json:"completedAt"`
}

type Summary struct {
	Request  RequestSummary  `json:"request"`
	Response ResponseSummary `json:"reqponse"`
}

type AuditResponseWriter interface {
	http.ResponseWriter
	Summarise() ResponseSummary
}

type AuditSink interface {
	Audit(summary Summary) error
}

//-------------------------------------------------------------------------------------------------

type AuditTapConfig struct {
	SizeThreshold string // split bodies greater than this (units are allowed)
	LogFile       string
	Truncate      bool
	Endpoint      string
	Method        string
	Topic         string
}

// AuditTap writes a summary of each request to the audit sink
type AuditTap struct {
	AuditSink     AuditSink
	Backend       string
	SizeThreshold int64
}

// NewAuditTap returns a new AuditTap handler.
func NewAuditTap(config AuditTapConfig, backend string) (*AuditTap, error) {
	sink, err := selectSink(config, backend)
	if err != nil {
		return nil, err
	}

	var th int64 = 1000000
	if config.SizeThreshold != "" {
		th, _, err = types.AsSI(config.SizeThreshold)
		if err != nil {
			return nil, err
		}
	}

	return &AuditTap{sink, backend, th}, nil
}

func selectSink(config AuditTapConfig, backend string) (AuditSink, error) {
	if config.LogFile != "" {
		fs, err := NewFileAuditSink(config.LogFile, backend, config.Truncate)
		return fs, err
	}

	if config.Topic != "" {
		//TODO
	}

	if config.Endpoint != "" {
		fs, err := NewHttpAuditSink(config.Method, config.Endpoint, backend)
		return fs, err
	}

	return &noopAuditSink{}, nil
}

func (s *AuditTap) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	req := RequestSummary{
		Source:    s.Backend,
		AuditType: "RequestReceived",
		Method:    r.Method,
		Path:      r.URL.String(),
		Header:    flattenHeaders(r.Header),
		BeganAt:   clock.Now(),
	}
	ww := NewAuditResponseWriter(rw)
	next.ServeHTTP(ww, r)
	s.AuditSink.Audit(Summary{req, ww.Summarise()})
}
