// Package handlers implements HTTP handlers for the Veyra web app.
//
// File ownership map:
//   - auth_handlers.go: login/logout/home/guide handlers and session cookie writes.
//   - media_handlers.go: media server endpoints and recently-added API-key fetch helper.
//   - dashboard_handlers.go: dashboard page composition and widget data loading.
//   - admin_handlers.go: admin pages, settings form handling, and admin integration views.
//   - seerr_handlers.go: authenticated Seerr search/request JSON endpoints.
//   - handlers.go: shared types, constructor, render helper, health endpoint, and
//     health coalescing.
//   - audit_helpers.go, cache_helpers.go, settings_helpers.go: focused reusable
//     helper groups used by the route handlers.
//
// This split keeps route-specific logic separate from reusable infrastructure code,
// making handler changes easier to review and safer to maintain.
package handlers
