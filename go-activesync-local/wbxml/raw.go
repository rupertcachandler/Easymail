package wbxml

// RawElement carries the verbatim wire content of a single WBXML element.
//
// It is intended for protocol layers (notably MS-ASCMD Sync.ApplicationData)
// where the wrapper element is fixed but the inner payload is class-dependent
// and decoded later by the caller. Bytes holds everything that appears between
// the element's open tag and matching END token, exclusive on both ends, but
// including any inner SWITCH_PAGE markers needed to replay the bytes
// verbatim. Page captures the active code page at the moment the open tag was
// emitted, so an encoder can re-establish that state before WriteRaw.
//
// SPEC: MS-ASWBXML/marshal.raw
type RawElement struct {
	Page  byte
	Bytes []byte
}
