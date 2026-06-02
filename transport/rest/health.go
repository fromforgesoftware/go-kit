package rest

import "net/http"

const healthCheckPath = "/healthz"

func NewHealthEndpoint() Endpoint {
	return NewEndpoint(
		http.MethodGet, healthCheckPath+"/",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
}
