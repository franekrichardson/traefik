package middlewares

import (
	"net/http"
	"bufio"
	"net"
	"fmt"
	"io"
	"os"
	"strings"
)

type ResponseHeader struct {
	Status int
	Header http.Header
	Size   int
}

type AuditSink interface {
	Audit(req http.Request, res ResponseHeader) error
}

//-------------------------------------------------------------------------------------------------

// Snooper writes a summary of each request to the receiver
type Probe struct {
	AuditSink AuditSink
}

// NewProbe returns a new Probe handler.
func NewProbe(receiver string, truncate bool) (*Probe, error) {
	if strings.HasPrefix(receiver, "/") {
		fs, err := newFileAuditSink(receiver, truncate)
		return &Probe{fs}, err
	}

	return nil, fmt.Errorf("%s: socket receiver not yet implemented", receiver)
}

func (s *Probe) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// ...could audit the request separately here...
	ww := NewRecorderResponseWriter(rw)
	next.ServeHTTP(ww, r)
	s.AuditSink.Audit(*r, ww.Summarise())
}

//-------------------------------------------------------------------------------------------------

type recorderResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func NewRecorderResponseWriter(w http.ResponseWriter) *recorderResponseWriter {
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

func (r *recorderResponseWriter) Status() int {
	return r.status
}

func (r *recorderResponseWriter) Write(b []byte) (int, error) {
	if !r.Written() {
		// The status will be StatusOK if WriteHeader has not been called yet
		r.WriteHeader(http.StatusOK)
	}
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, err
}

// Proxy method to Status to add support for gocraft
func (r *recorderResponseWriter) StatusCode() int {
	return r.Status()
}

func (r *recorderResponseWriter) Size() int {
	return r.size
}

func (r *recorderResponseWriter) Written() bool {
	return r.StatusCode() != 0
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

func (r *recorderResponseWriter) Summarise() ResponseHeader {
	return ResponseHeader{r.status, r.Header(), r.size}
}

//-------------------------------------------------------------------------------------------------

type fileAuditSink struct {
	w io.WriteCloser
}

var _ AuditSink = &fileAuditSink{nil}

func newFileAuditSink(receiver string, truncate bool) (*fileAuditSink, error) {
	flag := os.O_RDWR | os.O_CREATE
	if truncate {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(receiver, flag, 0666)
	if err != nil {
		return nil, err
	}
	return &fileAuditSink{f}, nil
}

func (fs *fileAuditSink) Audit(req http.Request, res ResponseHeader) error {
	s := fmt.Sprintf("%6s %s?%s %v\n       %d %d %v\n",
		req.Method, req.URL.Path, req.URL.Query(), req.Header,
		res.Status, res.Size, res.Header)
	_, err := fs.w.Write([]byte(s))
	return err
}

