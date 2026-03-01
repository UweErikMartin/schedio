// Package admin implements the authenticated HTTP handlers for the admin SPA.
//
// All endpoints require a valid staff session (RequireRole middleware from
// internal/auth). Route prefix: /admin/api/v1/ (registered by the server router).
package admin
