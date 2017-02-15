package audittap

import (
	"testing"

	"bufio"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
)

const tmpFile = "/tmp/testFileSink"

var testData = Summary{
	RequestSummary{
		Source:     "request1",
		AuditType:  "audit-type-1",
		Host:       "host.com",
		Method:     "method",
		Path:       "/a/b/c",
		Query:      "?z=00",
		RemoteAddr: "10.11.12.13",
		Header:     nil,
		BeganAt:    clock.Now(),
	},
	ResponseSummary{
		Source:      "response1",
		AuditType:   "audit-type-1",
		Status:      200,
		Header:      nil,
		Size:        123,
		CompletedAt: clock.Now(),
	},
}

func TestFileSink(t *testing.T) {
	w, err := NewFileAuditSink(tmpFile, "foo", InternalRenderer)
	assert.NoError(t, err)

	err = w.Audit(testData)
	assert.NoError(t, err)

	err = w.Audit(testData)
	assert.NoError(t, err)

	err = w.Close()
	assert.NoError(t, err)

	f, err := os.Open(tmpFile + "-foo.json")
	assert.NoError(t, err)

	scanner := bufio.NewScanner(f) // default behaviour splits on line boundaries

	// line 1
	assert.True(t, scanner.Scan())
	assert.Equal(t, "[", scanner.Text())

	// line 2
	assert.True(t, scanner.Scan())
	line := scanner.Text()
	assert.True(t, len(line) > 1 && line[0] == '{' && line[len(line)-2] == '}' && line[len(line)-1] == ',', "Expected JSON but got '%s'", line)

	// line 3
	assert.True(t, scanner.Scan())
	line = scanner.Text()
	assert.True(t, len(line) > 1 && line[0] == '{' && line[len(line)-1] == '}', "Expected JSON but got '%s'", line)

	// line 4
	assert.True(t, scanner.Scan())
	assert.Equal(t, "]", scanner.Text())

	// end of file
	assert.False(t, scanner.Scan())

	err = os.Remove(tmpFile + "-foo.json")
	assert.NoError(t, err)
}

func TestHttpSink(t *testing.T) {
	var got string

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		assert.NoError(t, err)
		got = string(body)
	}))
	defer stub.Close()

	// default format
	w1, err := NewHttpAuditSink("PUT", stub.URL, InternalRenderer)
	assert.NoError(t, err)

	err = w1.Audit(testData)
	assert.NoError(t, err)

	assert.Equal(t, string(InternalRenderer(testData).Bytes), got)

	// HMRC format
	w2, err := NewHttpAuditSink("PUT", stub.URL, HmrcRenderer)
	assert.NoError(t, err)

	err = w2.Audit(testData)
	assert.NoError(t, err)

	assert.Equal(t, string(HmrcRenderer(testData).Bytes), got)
}
