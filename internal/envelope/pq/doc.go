// Package pq contains an isolated prototype for the accepted Proofline
// post-quantum evidence envelope profile.
//
// This package is intentionally not wired into API routes, storage, bundle
// manifests, viewer responses, or simulator defaults. It exists so tests can
// exercise the profile identifiers, canonical encodings, local vectors, and
// fail-closed behavior before separate runtime-default work is authorized.
package pq
