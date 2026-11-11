package problem

import (
	"encoding/json"
	"net/http"

	"github.com/DujinA/spendulum/backend/internal/httpapi/gen"
)

const ContentType = "application/problem+json"

func New(status int, detail string) gen.Problem {
	p := gen.Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}

	return p
}

func Write(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(New(status, detail)) //nolint:errchkjson // the status is sent; a failed write means the client left
}
