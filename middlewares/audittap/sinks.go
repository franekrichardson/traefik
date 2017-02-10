package audittap

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type noopAuditSink struct {
	Summary
}

var _ AuditSink = &noopAuditSink{} // prove type conformance

func (fs *noopAuditSink) Audit(summary Summary) error {
	fs.Summary = summary
	return nil
}

//-------------------------------------------------------------------------------------------------

type fileAuditSink struct {
	w io.WriteCloser
}

var _ AuditSink = &fileAuditSink{nil} // prove type conformance

func NewFileAuditSink(file, backend string, truncate bool) (*fileAuditSink, error) {
	flag := os.O_RDWR | os.O_CREATE
	if truncate {
		flag |= os.O_TRUNC
	}
	name := determineFilename(file, backend)
	f, err := os.OpenFile(name, flag, 0644)
	if err != nil {
		return nil, err
	}
	return &fileAuditSink{f}, nil
}

func determineFilename(file, backend string) string {
	name := file
	if backend != "" {
		if strings.HasSuffix(name, ".json") {
			name = name[:len(name)-5]
		}
		name = fmt.Sprintf("%s-%s.json", name, backend)
	} else if !strings.HasSuffix(name, ".json") {
		name = name + ".json"
	}
	return name
}

var newline = []byte{'\n'}

func (fs *fileAuditSink) Audit(summary Summary) error {
	enc := summary.ToJson()
	if enc.Err != nil {
		return enc.Err
	}
	_, err := fs.w.Write(enc.Bytes)
	fs.w.Write(newline)
	return err
}

func (fs *fileAuditSink) Close() error {
	return fs.Close()
}

//-------------------------------------------------------------------------------------------------

type httpAuditSink struct {
	prototype http.Request
}

var _ AuditSink = &httpAuditSink{} // prove type conformance

func NewHttpAuditSink(method, endpoint, backend string) (sink *httpAuditSink, err error) {
	if method == "" {
		method = http.MethodPost
	}
	prototype := http.Request{}
	prototype.Method = method
	prototype.URL, err = url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Cannot access endpoint '%s': %v", endpoint, err)
	}
	return &httpAuditSink{prototype}, nil
}

func (fs *httpAuditSink) Audit(summary Summary) error {
	enc := summary.ToJson()
	if enc.Err != nil {
		return enc.Err
	}
	request := fs.prototype
	request.Body = ioutil.NopCloser(bytes.NewBuffer(enc.Bytes))
	res, err := http.DefaultClient.Do(&request)
	res.Body.Close()
	return err
}
