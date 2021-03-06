package audittap

import (
	"bytes"
	"net/http"
	"strings"
)

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailers", "transfer-encoding", "upgrade":
		return true
	}
	return false
}

func flattenKey(key string) string {
	b := bytes.Buffer{}
	parts := strings.Split(key, "-")
	for i, p := range parts {
		p = strings.ToLower(p)
		if i == 0 || len(p) <= 1 {
			b.WriteString(p)
		} else {
			b.WriteString(strings.ToUpper(p[:1]))
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func expandCookies(existing interface{}, v []string) []string {
	var all []string
	if existing != nil {
		all = existing.([]string)
	}
	for _, s := range v {
		cookies := strings.Split(s, ";")
		for _, c := range cookies {
			all = append(all, strings.TrimSpace(c))
		}
	}
	return all
}

func flattenHeaders(hdr http.Header) map[string]interface{} {
	flat := make(map[string]interface{})
	for k, v := range hdr {
		if !isHopByHopHeader(k) {
			f := flattenKey(k)
			if f == "cookie" {
				flat[f] = expandCookies(flat[f], v)

			} else {
				if len(v) == 1 {
					flat[f] = v[0]
				} else {
					flat[f] = v
				}
			}
		}
	}
	return flat
}
