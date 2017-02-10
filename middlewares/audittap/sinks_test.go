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

	err = w.Audit(summary)
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
