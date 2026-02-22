package handlers

import "net/http"

func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	dispatchHandlers(w, r, map[string]MethodHandler{
		http.MethodGet: handleGetHealthz,
	})
}

func handleGetHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
