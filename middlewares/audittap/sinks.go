package audittap

import (
	"bytes"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/containous/traefik/log"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

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
	render  Renderer
}

var _ AuditSink = &fileAuditSink{nil, nil, nil} // prove type conformance

func NewFileAuditSink(file, backend string, renderer Renderer) (*fileAuditSink, error) {
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
	return &fileAuditSink{f, newline, renderer}, nil
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

func (fas *fileAuditSink) Audit(summary Summary) error {
	enc := fas.render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	fas.w.Write(fas.lineEnd)
	_, err := fas.w.Write(enc.Bytes)
	fas.lineEnd = commaNewline
	return err
}

func (fas *fileAuditSink) Close() error {
	fas.w.Write(newline)
	fas.w.Write(closer)
	return fas.w.Close()
}

//-------------------------------------------------------------------------------------------------

type httpAuditSink struct {
	method, endpoint string
	render           Renderer
}

var _ AuditSink = &httpAuditSink{} // prove type conformance

func NewHttpAuditSink(method, endpoint string, renderer Renderer) (sink *httpAuditSink, err error) {
	if method == "" {
		method = http.MethodPost
	}
	_, err = url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Cannot access endpoint '%s': %v", endpoint, err)
	}
	return &httpAuditSink{method, endpoint, renderer}, nil
}

func (has *httpAuditSink) Audit(summary Summary) error {
	enc := has.render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	request, err := http.NewRequest(has.method, has.endpoint, bytes.NewBuffer(enc.Bytes))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Length", fmt.Sprintf("%d", enc.Length()))

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

//-------------------------------------------------------------------------------------------------

type kafkaAuditSink struct {
	topic    string
	producer sarama.AsyncProducer
	join     *sync.WaitGroup
	render   Renderer
}

var _ AuditSink = &kafkaAuditSink{} // prove type conformance

func NewKafkaAuditSink(topic, endpoint string, renderer Renderer) (sink *kafkaAuditSink, err error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = false
	producer, err := sarama.NewAsyncProducer([]string{endpoint}, config)
	if err != nil {
		panic(err)
	}

	kas := &kafkaAuditSink{topic, producer, &sync.WaitGroup{}, renderer}
	kas.join.Add(1)

	go func() {
		// read errors and log them, until the producer is closed
		for err := range producer.Errors() {
			log.Errorf("Kafka: %v", err)
		}
		kas.join.Done()
	}()

	return kas, nil
}

func (kas *kafkaAuditSink) Audit(summary Summary) error {
	enc := kas.render(summary)
	if enc.Err != nil {
		return enc.Err
	}
	message := &sarama.ProducerMessage{Topic: kas.topic, Value: enc}
	kas.producer.Input() <- message
	return nil
}

func (kas *kafkaAuditSink) Close() error {
	kas.producer.AsyncClose()
	kas.join.Wait()
	return nil
}
