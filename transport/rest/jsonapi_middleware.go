package rest

import (
	"context"
	"net/http"
	"strings"
)

// includesContextKeyType is the key used to store includes in the context
type includesContextKeyType int

const (
	includesCtxKey includesContextKeyType = iota
)

// WithJSONAPIIncludes is middleware that extracts the 'include' query parameter from the HTTP request
// and adds it to the context for later use by JSON:API encoders
func WithJSONAPIIncludes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		includeParam := r.URL.Query().Get("include")
		var includes []string
		if includeParam != "" {
			includes = strings.Split(includeParam, ",")
			for i := range includes {
				includes[i] = strings.TrimSpace(includes[i])
			}
		}
		ctx := context.WithValue(r.Context(), includesCtxKey, includes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetJSONAPIIncludes extracts the includes from the context
func GetJSONAPIIncludes(ctx context.Context) []string {
	if includes, ok := ctx.Value(includesCtxKey).([]string); ok {
		return includes
	}
	return nil
}
