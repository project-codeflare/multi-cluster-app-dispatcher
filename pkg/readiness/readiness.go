package readiness

import (
	"fmt"
	"net/http"
)

type Handler struct {
}

func (h *Handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	fmt.Fprint(resp, "ok")
}

