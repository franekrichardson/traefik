package middlewares

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/containous/traefik/log"
	"github.com/streamrail/concurrent-map"
)

const (
	loggerReqidHeader = "X-Traefik-Reqid"
)

/*
Logger writes each request and its response to the access log.
It gets some information from the logInfoResponseWriter set up by previous middleware.
*/
type Logger struct {
	file   *os.File
	format string
}

// Logging handler to log frontend name, backend name, and elapsed time
type frontendBackendLoggingHandler struct {
	reqid       string
	writer      io.Writer
	format      string
	handlerFunc http.HandlerFunc
}

var (
	reqidCounter        uint64       // Request ID
	infoRwMap           = cmap.New() // Map of reqid to response writer
	backend2FrontendMap *map[string]string
)

// logInfoResponseWriter is a wrapper of type http.ResponseWriter
// that tracks frontend and backend names and request status and size
type logInfoResponseWriter struct {
	rw       http.ResponseWriter
	backend  string
	frontend string
	status   int
	size     int
}

// logEntry is a single log entry for use in encoding to json
type logEntry struct {
	RemoteAddr    string `json:"remoteAddr"`
	Username      string `json:"username"`
	Timestamp     string `json:"timestamp"`
	Method        string `json:"method"`
	URI           string `json:"uri"`
	Protocol      string `json:"protocol"`
	Status        int    `json:"status"`
	Size          int    `json:"size"`
	Referer       string `json:"referer"`
	UserAgent     string `json:"userAgent"`
	RequestID     string `json:"requestID"`
	Frontend      string `json:"frontend"`
	Backend       string `json:"backend"`
	ElapsedMillis int64  `json:"elapsedMillis"`
	Host          string `json:"host"`
}

// logEntry is a single log entry for use in encoding to json
// general rules:
//   http_{NAME} the request header NAME - lowercased with underscores replacing spaces
//   sent_http_{NAME} the response header NAME - lowercased with underscores replacing spaces
//   upstream_http_{NAME} the server response header NAME - lowercased with underscores replacing spaces
type mdtpLogEntry struct {
	BodyBytesSent          string `json:"body_bytes_sent"`		//number of bytes sent to a client, not counting the response header; this variable is compatible with the “%B” parameter of the mod_log_config Apache module
	Connection             string `json:"connection"`		//connection serial number (1.3.8, 1.2.5)
	BytesSent              string `json:"bytes_sent"`		//the number of bytes sent to a client
	GzipRatio              string `json:"gzip_ratio"`		//log the achieved compression ratio.
	HttpHost               string `json:"http_host"`		//The value of the HTTP request header HEADER when converted to lowercase and with 'dashes' converted to 'underscores', e.g. $http_user_agent, $http_referer...;
	HttpReferrer           string `json:"http_referrer"`		//referer header (please note the log entry has the correct spelling as opposed to the actual header)
	HttpUserAgent          string `json:"http_user_agent"`		//user_agent request header
	HttpXRequestChain      string `json:"http_x_request_chain"`	//x_request_chain request header
	HttpXSessionId         string `json:"http_x_session_id"`	//x_session_id request header
	HttpXRequestId         string `json:"http_x_request_id"`	//x_request_id request header
	RemoteAddr             string `json:"remote_addr"`		//client address
	HttpTrueClientIp       string `json:"http_true_client_ip"`	//true_client_ip header
	ProxyHost              string `json:"proxy_host"`		//name and port of a proxied server as specified in the proxy_pass directive
	RemoteUser             string `json:"remote_user"`		//user name supplied with the Basic authentication
	Request                string `json:"request"`			//full original request line
	RequestMethod          string `json:"request_method"`		//request method, usually “GET” or “POST”
	RequestTime            string `json:"request_time"`		//request processing time in seconds with a milliseconds resolution (1.3.9, 1.2.6); time elapsed since the first bytes were read from the client
	RequestLength          string `json:"request_length"`		//request length (including request line, header, and request body) (1.3.12, 1.2.7)
	SentHttpLocation       string `json:"sent_http_location"`	//location response header
	ServerName             string `json:"server_name"`		//name of the server which accepted a request
	ServerPort             string `json:"server_port"`		//port of the server which accepted a request
	Status                 string `json:"status"`			//response status (1.3.2, 1.2.2)
	TimeLocal              string `json:"time_local"`		//local time in the Common Log Format (1.3.12, 1.2.7)
	UpstreamAddr           string `json:"upstream_addr"`		//keeps the IP address and port, or the path to the UNIX-domain socket of the upstream server. If several servers were contacted during request processing, their addresses are separated by commas, e.g. “192.168.1.1:80, 192.168.1.2:80, unix:/tmp/sock”. If an internal redirect from one server group to another happens, initiated by “X-Accel-Redirect” or error_page, then the server addresses from different groups are separated by colons, e.g. “192.168.1.1:80, 192.168.1.2:80, unix:/tmp/sock : 192.168.10.1:80, 192.168.10.2:80”.
	UpstreamHttpProxyAgent string `json:"upstream_http_proxy_agent"` //proxy_agent response header
	UpstreamHttpServer     string `json:"upstream_http_server"`	//server response header
	UpstreamResponseLength string `json:"upstream_response_length"`	//keeps the length of the response obtained from the upstream server (0.7.27); the length is kept in bytes. Lengths of several responses are separated by commas and colons like addresses in the $upstream_addr variable.
	UpstreamResponseTime   string `json:"upstream_response_time"`	//keeps time spent on receiving the response from the upstream server; the time is kept in seconds with millisecond resolution. Times of several responses are separated by commas and colons like addresses in the $upstream_addr variable.
	UpstreamStatus         string `json:"upstream_status"`		//keeps status code of the response obtained from the upstream server. Status codes of several responses are separated by commas and colons like addresses in the $upstream_addr variable.
	HttpXForwardedFor      string `json:"x_forwarded_for"`		//x_forwarded_for request header
}

// logEntryPool is used as we allocate a new logEntry on every request
var logEntryPool = sync.Pool{
	New: func() interface{} {
		return &mdtpLogEntry{}
	},
}

func (fblh *frontendBackendLoggingHandler) writeJSON(e *mdtpLogEntry) {
	data, err := json.Marshal(e)
	if err != nil {
		log.Error("unable to marshal json for log entry", err)
		return
	}
	data = append(data, newLineByte)
	// must do single write, rather than two (data then newline) to avoid interleaving lines
	fblh.writer.Write(data)
}

func (fblh *frontendBackendLoggingHandler) writeText(e *mdtpLogEntry) {
	fmt.Fprintf(fblh.writer, `%s - %s [%s] "%s" %d %d "%s" "%s" %s "%s" "%s" %dms%s`,
		e.HttpHost, e.RemoteUser, e.TimeLocal, e.Request, e.Status, e.RequestLength, e.HttpReferrer, e.HttpUserAgent, e.Connection, "-", "-", e.RequestTime, "\n")
}

// newLineByte is simple "\n" as a byte
var newLineByte = []byte("\n")[0]

// NewLogger returns a new Logger instance.
func NewLogger(file, format string) *Logger {
	if len(file) > 0 {
		fi, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Error("Error opening file", err)
		}
		return &Logger{file: fi, format: format}
	}
	return &Logger{file: nil, format: format}
}

// SetBackend2FrontendMap is called by server.go to set up frontend translation
func SetBackend2FrontendMap(newMap *map[string]string) {
	backend2FrontendMap = newMap
}

func (l *Logger) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if l.file == nil {
		next(rw, r)
	} else {
		reqid := strconv.FormatUint(atomic.AddUint64(&reqidCounter, 1), 10)
		r.Header[loggerReqidHeader] = []string{reqid}
		defer deleteReqid(r, reqid)

		(&frontendBackendLoggingHandler{reqid, l.file, l.format, next}).ServeHTTP(rw, r)
	}
}

// Delete a reqid from the map and the request's headers
func deleteReqid(r *http.Request, reqid string) {
	infoRwMap.Remove(reqid)
	delete(r.Header, loggerReqidHeader)
}

// Save the backend name for the Logger
func saveBackendNameForLogger(r *http.Request, backendName string) {
	if reqidHdr := r.Header[loggerReqidHeader]; len(reqidHdr) == 1 {
		reqid := reqidHdr[0]
		if infoRw, ok := infoRwMap.Get(reqid); ok {
			infoRw.(*logInfoResponseWriter).SetBackend(backendName)
			infoRw.(*logInfoResponseWriter).SetFrontend((*backend2FrontendMap)[backendName])
		}
	}
}

// Close closes the Logger (i.e. the file).
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// Logging handler to log frontend name, backend name, and elapsed time
func (fblh *frontendBackendLoggingHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	infoRw := &logInfoResponseWriter{rw: rw}
	infoRwMap.Set(fblh.reqid, infoRw)
	fblh.handlerFunc(infoRw, req)

	username := "-"
	url := *req.URL
	if url.User != nil {
		if name := url.User.Username(); name != "" {
			username = name
		}
	}

	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		ip = req.RemoteAddr
	}

	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		port = "80"
	}

	uri := url.RequestURI()
	if qmIndex := strings.Index(uri, "?"); qmIndex > 0 {
		uri = uri[0:qmIndex]
	}

	e := logEntryPool.Get().(*mdtpLogEntry)
	defer logEntryPool.Put(e)
	time.Since(startTime).Seconds()

	e.BodyBytesSent = strconv.Itoa(infoRw.GetSize())
	e.Connection = fblh.reqid
	e.BytesSent = strconv.Itoa(infoRw.GetSize()) //todo - difference between this and body bytes sent
	e.GzipRatio = "-" //todo calculate gzip ratio
	e.HttpHost = req.Host
	e.HttpReferrer = req.Referer()
	e.HttpUserAgent = req.UserAgent()
	e.HttpXRequestChain = req.Header.Get("X-Request-Chain")
	e.HttpXSessionId = req.Header.Get("X-Session-ID")
	e.HttpXRequestId = req.Header.Get("X-Request-ID")
	e.RemoteAddr = ip
	e.HttpTrueClientIp = req.Header.Get("True-Client-IP")
	e.ProxyHost = infoRw.backend
	e.RemoteUser = username
	e.Request = fmt.Sprintf("%s %s %s", req.Method, req.RequestURI, req.Proto) 
	e.RequestMethod = req.Method
	e.RequestTime = strconv.FormatFloat(time.Since(startTime).Seconds(), 'f', 3, 64)
	e.RequestLength = "-" //todo request length
	e.SentHttpLocation = rw.Header().Get("Location")
	e.ServerName = host
	e.ServerPort = port
	e.Status = strconv.Itoa(infoRw.GetStatus())
	e.TimeLocal = startTime.Format("02/Jan/2006:15:04:05 -0700")
	e.UpstreamAddr = "-" //todo get ip of actual backend used
	e.UpstreamHttpProxyAgent = "-" //todo
	e.UpstreamHttpServer = "-" //todo
	e.UpstreamResponseLength = "-" //todo
	e.UpstreamResponseTime = "-" //todo
	e.UpstreamStatus = "-" //todo
	e.HttpXForwardedFor = req.Header.Get("X-Forwarded-For")

	//e.Username = username
	//e.Timestamp = startTime.Format("02/Jan/2006:15:04:05 -0700")
	//e.Method = req.Method
	//e.URI = uri
	//e.Protocol = req.Proto
	//e.Status = infoRw.GetStatus()
	//e.Size = infoRw.GetSize()
	//e.Referer = req.Referer()
	//e.UserAgent = req.UserAgent()
	//e.RequestID = fblh.reqid
	//e.Frontend = strings.TrimPrefix(infoRw.GetFrontend(), "frontend-")
	//e.Backend = infoRw.GetBackend()
	//e.ElapsedMillis = time.Since(startTime).Nanoseconds() / 1000000
	//e.Host = req.Host

	if fblh.format == "json" {
		fblh.writeJSON(e)
	} else {
		fblh.writeText(e)
	}
}

func (lirw *logInfoResponseWriter) Header() http.Header {
	return lirw.rw.Header()
}

func (lirw *logInfoResponseWriter) Write(b []byte) (int, error) {
	if lirw.status == 0 {
		lirw.status = http.StatusOK
	}
	size, err := lirw.rw.Write(b)
	lirw.size += size
	return size, err
}

func (lirw *logInfoResponseWriter) WriteHeader(s int) {
	lirw.rw.WriteHeader(s)
	lirw.status = s
}

func (lirw *logInfoResponseWriter) Flush() {
	f, ok := lirw.rw.(http.Flusher)
	if ok {
		f.Flush()
	}
}

func (lirw *logInfoResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return lirw.rw.(http.Hijacker).Hijack()
}

func (lirw *logInfoResponseWriter) GetStatus() int {
	return lirw.status
}

func (lirw *logInfoResponseWriter) GetSize() int {
	return lirw.size
}

func (lirw *logInfoResponseWriter) GetBackend() string {
	return lirw.backend
}

func (lirw *logInfoResponseWriter) GetFrontend() string {
	return lirw.frontend
}

func (lirw *logInfoResponseWriter) SetBackend(backend string) {
	lirw.backend = backend
}

func (lirw *logInfoResponseWriter) SetFrontend(frontend string) {
	lirw.frontend = frontend
}
