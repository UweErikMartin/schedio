package web

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed js/default.js
var defaultJS []byte

//go:embed js/x-date-time-picker.js
var dateTimePickerJS []byte

//go:embed js/x-service-picker.js
var servicePickerJS []byte

//go:embed js/x-toast.js
var toastJS []byte

//go:embed js/booking/booking-app.js
var bookingAppJS []byte

//go:embed js/manage/booking-manager.js
var bookingManagerJS []byte

//go:embed js/manage/booking-card.js
var bookingCardJS []byte

//go:embed js/manage/reschedule-picker.js
var reschedulePickerJS []byte

//go:embed js/manage/cancel-confirm.js
var cancelConfirmJS []byte

//go:embed js/admin/admin-app.js
var adminAppJS []byte

//go:embed js/admin/login-form.js
var loginFormJS []byte

//go:embed js/admin/admin-nav.js
var adminNavJS []byte

//go:embed js/admin/admin-dashboard.js
var adminDashboardJS []byte

//go:embed js/admin/settings-form.js
var settingsFormJS []byte

//go:embed css/styles.css
var stylesCSS []byte

//go:embed css/tokens.css
var tokensCSS []byte

//go:embed html/index.html
var indexHTML []byte

//go:embed html/admin.html
var adminHTML []byte

//go:embed misc/favicon.ico
var favicon []byte

// assetMap maps URL paths to embedded file content.
// Initialised once at startup; rebuilt on every request would be wasteful.
var assetMap = map[string][]byte{
	"/js/default.js":               defaultJS,
	"/js/x-date-time-picker.js":    dateTimePickerJS,
	"/js/x-service-picker.js":      servicePickerJS,
	"/js/x-toast.js":               toastJS,
	"/js/booking/booking-app.js":       bookingAppJS,
	"/js/manage/booking-manager.js":    bookingManagerJS,
	"/js/manage/booking-card.js":       bookingCardJS,
	"/js/manage/reschedule-picker.js":  reschedulePickerJS,
	"/js/manage/cancel-confirm.js":     cancelConfirmJS,
	"/js/admin/admin-app.js":       adminAppJS,
	"/js/admin/login-form.js":      loginFormJS,
	"/js/admin/admin-nav.js":       adminNavJS,
	"/js/admin/admin-dashboard.js": adminDashboardJS,
	"/js/admin/settings-form.js":   settingsFormJS,
	"/css/styles.css":              stylesCSS,
	"/css/tokens.css":              tokensCSS,
	"/favicon.ico":                 favicon,
	"/":                            indexHTML,
	"/index.html":                  indexHTML,
}

// GetContent returns the embedded content for the given URL path.
// All paths under /admin/ (and /admin itself) that are not registered as
// assets are served as admin.html so that client-side routing works.
func GetContent(path string) ([]byte, error) {
	content, ok := assetMap[path]
	if ok {
		return content, nil
	}
	// Serve the admin SPA shell for any unrecognised /admin/* path so that
	// the JS router can handle client-side navigation (e.g. /admin/settings).
	if path == "/admin" || strings.HasPrefix(path, "/admin/") {
		return adminHTML, nil
	}
	return nil, http.ErrMissingFile
}
