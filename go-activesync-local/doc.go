// Package activesync is the umbrella documentation entry point for the
// go-activesync library, a pure-Go client for the Microsoft Exchange
// ActiveSync (EAS) protocol, revision 14.1.
//
// The library is split into focused subpackages:
//
//   - [github.com/remdev/go-activesync/wbxml]: MS-ASWBXML / WBXML 1.3
//     codec and EAS code page tables.
//   - [github.com/remdev/go-activesync/eas]: typed request/response and
//     domain models for the supported commands and PIM data classes.
//   - [github.com/remdev/go-activesync/autodiscover]: POX Autodiscover
//     client (MS-OXDISCO, MS-ASAB).
//   - [github.com/remdev/go-activesync/client]: high level EAS client
//     wiring transport, authentication, policy/sync state stores and
//     command methods together.
//
// Implementation references the following Microsoft Open Specifications:
// MS-ASHTTP, MS-ASWBXML, MS-ASCMD, MS-ASEMAIL, MS-ASCAL, MS-ASCNTC, MS-ASTASK,
// MS-ASPROV, MS-OXDISCO, MS-ASAB.
package activesync
