package wbxml

import (
	"fmt"
	"io"
)

// Encoder is a streaming WBXML 1.3 encoder. It tracks the active code page
// and inserts SWITCH_PAGE bytes whenever the next tag is on a different page.
type Encoder struct {
	w        io.Writer
	page     byte
	pageInit bool
}

// NewEncoder returns a new encoder writing to w. The active code page is
// initialised to 0 (AirSync) per OMA-WBXML 1.3 §5.5: the document starts
// with code page 0 active without requiring a SWITCH_PAGE prefix.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, page: 0, pageInit: true}
}

// WriteHeader serialises h to the underlying writer.
func (e *Encoder) WriteHeader(h Header) error {
	return h.Write(e.w)
}

// StartTag emits a tag byte for (page, identity) on the active code page,
// preceded by a SWITCH_PAGE token if the page differs from the active one.
func (e *Encoder) StartTag(page, identity byte, hasAttrs, hasContent bool) error {
	if _, ok := PageByID(page); !ok {
		return fmt.Errorf("wbxml: unknown code page %d", page)
	}
	if !e.pageInit || e.page != page {
		if _, err := e.w.Write([]byte{SwitchPage, page}); err != nil {
			return err
		}
		e.page = page
		e.pageInit = true
	}
	tag := EncodeTag(identity, hasAttrs, hasContent)
	_, err := e.w.Write([]byte{tag})
	return err
}

// EndTag emits the END token, closing the most recent open element.
func (e *Encoder) EndTag() error {
	_, err := e.w.Write([]byte{End})
	return err
}

// StrI emits an inline string token (STR_I + UTF-8 bytes + NUL).
func (e *Encoder) StrI(s string) error {
	if _, err := e.w.Write([]byte{StrI}); err != nil {
		return err
	}
	if _, err := io.WriteString(e.w, s); err != nil {
		return err
	}
	_, err := e.w.Write([]byte{0x00})
	return err
}

// Opaque emits the OPAQUE token followed by the mb_u_int32-prefixed payload.
func (e *Encoder) Opaque(b []byte) error {
	hdr := AppendMbUint32([]byte{Opaque}, uint32(len(b)))
	if _, err := e.w.Write(hdr); err != nil {
		return err
	}
	_, err := e.w.Write(b)
	return err
}

// ForceSwitchPage emits a SWITCH_PAGE token to p and updates the encoder's
// active page. Unlike StartTag, the token is emitted unconditionally so
// callers can align the encoder state with externally produced raw bytes
// before calling WriteRaw.
func (e *Encoder) ForceSwitchPage(p byte) error {
	if _, ok := PageByID(p); !ok {
		return fmt.Errorf("wbxml: unknown code page %d", p)
	}
	if _, err := e.w.Write([]byte{SwitchPage, p}); err != nil {
		return err
	}
	e.page = p
	e.pageInit = true
	return nil
}

// WriteRaw writes b to the underlying stream verbatim. After the call the
// encoder's active page is marked as unknown, so the next StartTag is
// guaranteed to be preceded by a SWITCH_PAGE. b is expected to be a
// well-formed sequence of WBXML tokens that does not contain a trailing END
// for any caller-managed element; balancing those END tokens is the caller's
// responsibility.
func (e *Encoder) WriteRaw(b []byte) error {
	if _, err := e.w.Write(b); err != nil {
		return err
	}
	e.pageInit = false
	return nil
}
