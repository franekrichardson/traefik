package middlewares

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type RequestSummary struct {
	Method  string
	URL     string
	Header  map[string]interface{} // contains strings or string slices
	BeganAt time.Time
}

type ResponseSummary struct {
	Status      int
	Header      map[string]interface{} // contains strings or string slices
	Size        int
	CompletedAt time.Time
}

type Summary struct {
	Request  RequestSummary
	Response ResponseSummary
}

type AuditResponseWriter interface {
	http.ResponseWriter
	Summarise() ResponseSummary
}

type AuditSink interface {
	Audit(summary Summary) error
}

//-------------------------------------------------------------------------------------------------

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailers", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

func flattenKey(key string) string {
	b := bytes.Buffer{}
	parts := strings.Split(key, "-")
	for i, p := range parts {
		p = strings.ToLower(p)
		if i == 0 || len(p) <= 1 {
			b.WriteString(p)
		} else {
			b.WriteString(strings.ToUpper(p[:1]))
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func flattenHeaders(hdr http.Header) map[string]interface{} {
	flat := make(map[string]interface{})
	for k, v := range hdr {
		if !isHopByHopHeader(k) {
			f := flattenKey(k)
			if len(v) == 1 {
				flat[f] = v[0]
			} else {
				flat[f] = v
			}
		}
	}
	return flat
}

//-------------------------------------------------------------------------------------------------

type ProbeConfig struct {
	LogFile  string
	Truncate bool
}

// Probe writes a summary of each request to the audit sink
type Probe struct {
	AuditSink AuditSink
}

// NewProbe returns a new Probe handler.
func NewProbe(config ProbeConfig) (*Probe, error) {
	if config.LogFile != "" {
		if strings.HasSuffix(config.LogFile, ".json") {
			fs, err := newJsonAuditSink(config.LogFile, config.Truncate)
			return &Probe{fs}, err
		}

		fs, err := newFileAuditSink(config.LogFile, config.Truncate)
		return &Probe{fs}, err
	}

	return nil, fmt.Errorf("Probe not yet implemented for %v", config)
}

func (s *Probe) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	req := RequestSummary{
		Method:  r.Method,
		URL:     r.URL.String(),
		Header:  flattenHeaders(r.Header),
		BeganAt: time.Now(),
	}
	ww := NewAuditResponseWriter(rw)
	next.ServeHTTP(ww, r)
	report := ww.Summarise()
	s.AuditSink.Audit(Summary{req, report})
}

//-------------------------------------------------------------------------------------------------

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
	return ResponseSummary{r.status, flattenHeaders(r.Header()), r.size, time.Now()}
}

//-------------------------------------------------------------------------------------------------

type fileAuditSink struct {
	w io.WriteCloser
}

var _ AuditSink = &fileAuditSink{nil} // prove type conformance

func newFileAuditSink(receiver string, truncate bool) (*fileAuditSink, error) {
	flag := os.O_RDWR | os.O_CREATE
	if truncate {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(receiver, flag, 0644)
	if err != nil {
		return nil, err
	}
	return &fileAuditSink{f}, nil
}

const formatString = "2006-01-02T15:04:05"
const blankString = "                   "

func (fs *fileAuditSink) Audit(summary Summary) error {
	s := fmt.Sprintf("%s %6s %s %d %d\n%s %+v\n%s %+v\n",
		summary.Response.CompletedAt.Format(formatString),
		summary.Request.Method, summary.Request.URL, summary.Response.Status, summary.Response.Size,
		blankString, summary.Request.Header,
		blankString, summary.Response.Header)
	_, err := fs.w.Write([]byte(s))
	return err
}

//-------------------------------------------------------------------------------------------------

type jsonAuditSink struct {
	w io.WriteCloser
}

var _ AuditSink = &jsonAuditSink{nil} // prove type conformance

func newJsonAuditSink(receiver string, truncate bool) (*jsonAuditSink, error) {
	flag := os.O_RDWR | os.O_CREATE
	if truncate {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(receiver, flag, 0644)
	if err != nil {
		return nil, err
	}
	return &jsonAuditSink{f}, nil
}

func (fs *jsonAuditSink) Audit(summary Summary) error {
	b, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = fs.w.Write(b)
	return err
}
