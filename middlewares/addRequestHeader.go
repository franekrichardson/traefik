package middlewares

import (
	"net/http"
	"github.com/google/uuid"
	"fmt"
)

// AddHeader is a middleware used to add a request header to a request
type AddRequestHeader struct {
	Handler http.Handler
}

func (s *AddRequestHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uuidVal, err := uuid.NewUUID()
	uuidStr := uuidVal.String()
	if err != nil {
		uuidStr = fmt.Sprintf("UUID-ERROR-%s", err.Error())
	}
	r.Header.Add("X-Request-ID", fmt.Sprintf("govuk-ia-tax-%s", uuidStr))
	s.Handler.ServeHTTP(w, r)
}

// SetHandler sets handler
func (s *AddRequestHeader) SetHandler(Handler http.Handler) {
	s.Handler = Handler
}
