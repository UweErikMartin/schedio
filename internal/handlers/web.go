package handlers

import (
	"mime"
	"net/http"
	"path/filepath"
	"schedio/web"

	"k8s.io/klog/v2"
)

func HandleWebUserInterface(w http.ResponseWriter, r *http.Request) {
	content, err := web.GetContent(r.URL.Path)
	if err != nil {
		klog.Errorf("Failed to get content for path %s: %v", r.URL.Path, err)
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else if contentType := mime.TypeByExtension(filepath.Ext(r.URL.Path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	w.Write(content)
}
