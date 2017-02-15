package audittap

import (
	"github.com/containous/traefik/types"
	"net/http"
	"time"
)

//-------------------------------------------------------------------------------------------------

type RequestSummary struct {
	Source     string                 `json:"auditSource,omitempty"`
	AuditType  string                 `json:"auditType,omitempty"`
	Host       string                 `json:"host"`
	Method     string                 `json:"method"`
	Path       string                 `json:"path"`
	Query      string                 `json:"query"`
	RemoteAddr string                 `json:"remoteAddr"`
	Header     map[string]interface{} `json:"header"` // contains strings or string slices
	BeganAt    time.Time              `json:"beganAt"`
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

type Renderer func(Summary) Encoded

//-------------------------------------------------------------------------------------------------

// AuditTap writes a enc of each request to the audit sink
type AuditTap struct {
	AuditSinks    []AuditSink
	Backend       string
	SizeThreshold int64
}

// NewAuditTap returns a new AuditTap handler.
func NewAuditTap(config *types.AuditTap, backend string) (*AuditTap, error) {
	var renderer Renderer = InternalRenderer
	if config.Format == "HMRC" {
		renderer = HmrcRenderer
	}

	sinks, err := selectSinks(config, backend, renderer)
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

	return &AuditTap{sinks, backend, th}, nil
}

func selectSinks(config *types.AuditTap, backend string, renderer Renderer) ([]AuditSink, error) {
	var sinks []AuditSink

	if config.LogFile != "" {
		fas, err := NewFileAuditSink(config.LogFile, backend, renderer)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, fas)
	}

	if config.Endpoint != "" {
		if config.Topic != "" {
			kas, err := NewKafkaAuditSink(config.Topic, config.Endpoint, renderer)
			if err != nil {
				return nil, err
			}
			sinks = append(sinks, kas)
		} else {
			has, err := NewHttpAuditSink(config.Method, config.Endpoint, renderer)
			if err != nil {
				return nil, err
			}
			sinks = append(sinks, has)
		}
	}

	if sinks == nil {
		sinks = append(sinks, &noopAuditSink{})
	}

	return sinks, nil
}

func (s *AuditTap) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	req := RequestSummary{
		Source:     s.Backend,
		AuditType:  "Traefik1",
		Host:       r.Host,
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		RemoteAddr: r.RemoteAddr,
		Header:     flattenHeaders(r.Header),
		BeganAt:    clock.Now(),
	}

	ww := NewAuditResponseWriter(rw)
	next.ServeHTTP(ww, r)

	summary := Summary{req, ww.Summarise()}
	for _, sink := range s.AuditSinks {
		sink.Audit(summary)
	}
}
