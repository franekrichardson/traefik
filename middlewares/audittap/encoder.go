package audittap

import (
	"encoding/json"
	"fmt"
)

const formatString = "2006-01-02T15:04:05"
const blankString = "                   "

type Encoded struct {
	Bytes []byte
	Err   error
}

func (summary Summary) ToString() Encoded {
	s := fmt.Sprintf("%s %6s %s %d %d\n%s %+v\n%s %+v\n",
		summary.Response.CompletedAt.Format(formatString),
		summary.Request.Method, summary.Request.Path, summary.Response.Status, summary.Response.Size,
		blankString, summary.Request.Header,
		blankString, summary.Response.Header)
	return Encoded{[]byte(s), nil}
}

func (summary Summary) ToJson() Encoded {
	b, err := json.Marshal(summary)
	return Encoded{b, err}
}

func (summary Encoded) Encode() ([]byte, error) {
	return summary.Bytes, summary.Err
}

func (summary Encoded) Length() int {
	return len(summary.Bytes)
}
