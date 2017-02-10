package audittap

import (
	"testing"

	"bufio"
	"github.com/stretchr/testify/assert"
	"os"
)

const tmpFile = "/tmp/testFileSink"

func TestFileSink1(t *testing.T) {
	w, err := NewFileAuditSink(tmpFile, "foo", true)
	assert.NoError(t, err)

	summary := Summary{}
	err = w.Audit(summary)
	assert.NoError(t, err)

	f, err := os.Open(tmpFile + "-foo.json")
	assert.NoError(t, err)

	n := 0
	scanner := bufio.NewScanner(f) // default behaviour splits on line boundaries
	for scanner.Scan() {
		n += 1
		line := scanner.Text()
		assert.True(t, len(line) > 1 && line[0] == '{' && line[len(line)-1] == '}', "Expected JSON but got '%s'", line)
	}
	if n != 1 {
		assert.Fail(t, "Wrong number of lines", "Expected 1 but got %d", n)
	}

	err = os.Remove(tmpFile + "-foo.json")
	assert.NoError(t, err)
}
