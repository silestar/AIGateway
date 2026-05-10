// Package sdk defines public interfaces and registries for AIGateway plugins.
// System-type plugins implement interfaces defined here and register themselves
// via init() functions compiled into the main binary.
package sdk

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
)

// ConnectionDecorator allows a system plugin to decorate an outbound connection
// before the TLS handshake. The raw TCP connection and TLS configuration are
// passed in; the decorator may perform its own TLS handshake (e.g. with utls
// for fingerprint masquerading), wrap the connection, or return nil to indicate
// "passthrough" — in which case the standard tls.Dial handshake is used.
//
// Decorators are called in registration order. The first decorator returning a
// non-nil net.Conn wins; subsequent decorators are skipped.
type ConnectionDecorator interface {
	Decorate(ctx context.Context, channelID, accountID uint, config json.RawMessage,
		rawConn net.Conn, tlsCfg *tls.Config) (net.Conn, error)
}

// connectionDecorators is the global registry of system-level connection decorators.
// Plugins register themselves via init().
var connectionDecorators []ConnectionDecorator

// RegisterConnectionDecorator registers a connection decorator. Safe to call
// from init() — no locking is required during program initialization.
func RegisterConnectionDecorator(d ConnectionDecorator) {
	connectionDecorators = append(connectionDecorators, d)
}

// GetConnectionDecorators returns the list of all registered connection decorators.
func GetConnectionDecorators() []ConnectionDecorator {
	return connectionDecorators
}