// Package sdk defines public interfaces for AIGateway plugins.
//
// Connection decoration is now handled via the sidecar proxy pattern:
// the proxy engine queries the plugin manager for running connection_decorator
// plugins and connects to them via the simplified CONNECT protocol.
// See internal/proxy/engine.go (dialViaDecorator) for the client side
// and plugins/tls-fingerprint-masquerade for the server side.
package sdk
