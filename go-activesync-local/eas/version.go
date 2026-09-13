// Package eas defines the typed Exchange ActiveSync 14.1 commands and domain
// objects used by the client. Each domain type maps onto WBXML through
// reflection-based marshaling using struct tags of the form
// `wbxml:"Page.Tag"` resolved by the wbxml package.
package eas

// ProtocolVersion is the EAS protocol version this library implements.
const ProtocolVersion = "14.1"
