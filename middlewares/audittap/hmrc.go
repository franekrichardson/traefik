package audittap

import (
	"encoding/json"
	"time"
)

type Tags struct {
	ClientIP         string `json:"clientIP"`
	ClientPort       string `json:"clientPort"`
	Path             string `json:"path"`
	SessionID        string `json:"sessionID"`
	RequestID        string `json:"requestID"`
	AkamaiReputation string `json:"Akamai-Reputation"`
	TransactionName  string `json:"transactionName"`
}

type Detail struct {
	Method            string `json:"method"`
	Host              string `json:"host"`
	Port              string `json:"port"`
	Input             string `json:"input"`
	Token             string `json:"token"`
	IPAddress         string `json:"ipAddress"`
	DeviceID          string `json:"deviceID"`
	DeviceFingerprint string `json:"deviceFingerprint"`
	UserAgentString   string `json:"userAgentString"`
	QueryString       string `json:"queryString"`
	RequestBody       string `json:"requestBody"`
	Referrer          string `json:"referrer"`
	StatusCode        string `json:"statusCode"`
	ResponseMessage   string `json:"responseMessage"`
	Authorization     string `json:"Authorization"`
}

type Hmrc struct {
	Source      string    `json:"auditSource"`
	AuditType   string    `json:"auditType"`
	EventId     string    `json:"eventId"`
	Tags        Tags      `json:"tags"`
	Detail      Detail    `json:"detail"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func textOrDash(s interface{}) string {
	str, ok := s.(string)
	if ok && str != "" {
		return str
	}

	return "-"
}

func newTags(summary Summary) Tags {
	tags := Tags{
		ClientIP:         textOrDash(summary.Request.Header["clientIP"]),
		ClientPort:       "-",
		Path:             summary.Request.Path,
		SessionID:        textOrDash(summary.Request.Header["clientIP"]),
		RequestID:        textOrDash(summary.Request.Header["clientIP"]),
		AkamaiReputation: textOrDash(summary.Request.Header["clientIP"]),
		TransactionName:  "",
	}
	return tags
}

func newDetail(summary Summary) Detail {
	detail := Detail{
		Method:            summary.Request.Method,
		Host:              summary.Request.Host,
		Port:              "",
		Input:             summary.Request.Path,
		Token:             "",
		IPAddress:         "",
		DeviceID:          "",
		DeviceFingerprint: "",
		UserAgentString:   textOrDash(summary.Request.Header["userAgent"]),
		QueryString:       "",
		RequestBody:       "",
		Referrer:          textOrDash(summary.Request.Header["referer"]), // n.b. this mis-spelling is required
		StatusCode:        "",
		ResponseMessage:   "",
		Authorization:     "",
	}
	return detail
}

func NewHmrc(summary Summary) Hmrc {
	detail := newDetail(summary)

	data := Hmrc{
		Source:      summary.Request.Source,
		AuditType:   summary.Request.AuditType,
		EventId:     "event123", // TODO
		Tags:        newTags(summary),
		Detail:      detail,
		GeneratedAt: summary.Response.CompletedAt,
	}
	return data
}

func (data Hmrc) ToJson() Encoded {
	b, err := json.Marshal(data)
	return Encoded{b, err}
}

func HmrcRenderer(summary Summary) Encoded {
	data := NewHmrc(summary)
	return data.ToJson()
}
