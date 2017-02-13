package audittap

import (
	"encoding/json"
	"fmt"
)

const formatString = "2006-01-02T15:04:05"
const blankString = "                   "

// See https://godoc.org/github.com/Shopify/sarama#Encoder
type Encoder interface {
	Encode() ([]byte, error)
	Length() int
}

type Encoded struct {
	Bytes []byte
	Err   error
}

func (enc Encoded) Encode() ([]byte, error) {
	return enc.Bytes, enc.Err
}

func (enc Encoded) Length() int {
	return len(enc.Bytes)
}

//-------------------------------------------------------------------------------------------------

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

func InternalRenderer(summary Summary) Encoded {
	return summary.ToJson()
}
