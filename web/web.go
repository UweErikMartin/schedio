package web

import (
	_ "embed"
	"net/http"
)

//go:embed js/default.js
var defaultJS []byte

//go:embed js/x-date-time-picker.js
var dateTimePickerJS []byte

//go:embed js/x-service-picker.js
var servicePickerJS []byte

//go:embed css/styles.css
var stylesCSS []byte

//go:embed html/index.html
var indexHTML []byte

//go:embed misc/favicon.ico
var favicon []byte

// assetMap maps URL paths to embedded file content.
// Initialised once at startup; rebuilt on every request would be wasteful.
var assetMap = map[string][]byte{
	"/js/default.js":            defaultJS,
	"/js/x-date-time-picker.js": dateTimePickerJS,
	"/js/x-service-picker.js":   servicePickerJS,
	"/css/styles.css":           stylesCSS,
	"/favicon.ico":              favicon,
	"/":                         indexHTML,
	"/index.html":               indexHTML,
}

func GetContent(path string) ([]byte, error) {
	content, ok := assetMap[path]
	if !ok {
		return nil, http.ErrMissingFile
	}
	return content, nil
}
