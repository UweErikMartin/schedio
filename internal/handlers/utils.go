package handlers

import (
	"net/http"
	"sort"
	"strings"
)

type MethodHandler func(http.ResponseWriter, *http.Request)

// dispatchHandlers routes the request to the handler matching r.Method.
// The allowed methods and their order in the Allow header are derived from
// the keys of the handlers map, so callers do not need to repeat them.
func dispatchHandlers(w http.ResponseWriter, r *http.Request, handlers map[string]MethodHandler) {
	if h, ok := handlers[r.Method]; ok {
		h(w, r)
		return
	}
	methods := make([]string, 0, len(handlers))
	for m := range handlers {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
