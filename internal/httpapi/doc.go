// Package httpapi exposes separate HTTP handlers for the main API/viewer
// listener and the private-admin listener.
//
// This package implements local account/session authentication for main API
// routes and private admin routes. The main API is still meant to run behind a
// reviewed deployment boundary; adding public authentication, OAuth, JWT, or a
// public account portal here would exceed that scope.
package httpapi
