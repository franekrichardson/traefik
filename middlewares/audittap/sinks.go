package audittap

import (
	"bytes"
	"fmt"
	"github.com/Shopify/sarama"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var render Renderer = HmrcRenderer

//var render Renderer = InternalRenderer

//-------------------------------------------------------------------------------------------------

type noopAuditSink struct {
	Summary
}

var _ AuditSink = &noopAuditSink{} // prove type conformance

func (fs *noopAuditSink) Audit(summary Summary) error {
	fs.Summary = summary
	return nil
}

//-------------------------------------------------------------------------------------------------

var opener = []byte{'['}
var closer = []byte{']', '\n'}
var commaNewline = []byte{',', '\n'}
var newline = []byte{'\n'}

type fileAuditSink struct {
	w       io.WriteCloser
	lineEnd []byte
}

var _ AuditSink = &fileAuditSink{nil, nil} // prove type conformance

func NewFileAuditSink(file, backend string) (*fileAuditSink, error) {
	flag := os.O_RDWR | os.O_CREATE
	if strings.HasPrefix(file, ">>") {
		file = strings.TrimSpace(file[2:])
	} else {
		flag |= os.O_TRUNC
	}
	name := determineFilename(file, backend)
	f, err := os.OpenFile(name, flag, 0644)
	if err != nil {
		return nil, err
	}
	f.Write(opener)
	return &fileAuditSink{f, newline}, nil
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

func (fs *fileAuditSink) Audit(summary Summary) error {
	enc := render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	fs.w.Write(fs.lineEnd)
	_, err := fs.w.Write(enc.Bytes)
	fs.lineEnd = commaNewline
	return err
}

func (fs *fileAuditSink) Close() error {
	fs.w.Write(newline)
	fs.w.Write(closer)
	return fs.w.Close()
}

//-------------------------------------------------------------------------------------------------

type httpAuditSink struct {
	prototype http.Request
}

var _ AuditSink = &httpAuditSink{} // prove type conformance

func NewHttpAuditSink(method, endpoint string) (sink *httpAuditSink, err error) {
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
	enc := render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	request := fs.prototype
	request.Body = ioutil.NopCloser(bytes.NewBuffer(enc.Bytes))
	res, err := http.DefaultClient.Do(&request)
	res.Body.Close()
	return err
}

//-------------------------------------------------------------------------------------------------

type kafkaAuditSink struct {
	topic    string
	producer sarama.AsyncProducer
}

var _ AuditSink = &kafkaAuditSink{} // prove type conformance

func NewKafkaAuditSink(topic, endpoint string) (sink *kafkaAuditSink, err error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = false
	producer, err := sarama.NewAsyncProducer([]string{endpoint}, config)
	if err != nil {
		panic(err)
	}

	go func() {
		for err := range producer.Errors() {
			log.Printf("Kafka: %v", err)
		}
	}()

	return &kafkaAuditSink{topic, producer}, nil
}

func (fs *kafkaAuditSink) Audit(summary Summary) error {
	enc := render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	message := &sarama.ProducerMessage{Topic: fs.topic, Value: enc}
	fs.producer.Input() <- message
	return nil
}
