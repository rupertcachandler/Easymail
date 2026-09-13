package wbxml

import (
	"bytes"
	"errors"
	"testing"
)

// SPEC: MS-ASWBXML/encoder.tag
// SPEC: MS-ASWBXML/encoder.end
func TestEncoder_StartEndTag_SamePage(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	// AirSync.Sync (page 0, identity 0x05) with content
	if err := enc.StartTag(PageAirSync, 0x05, false, true); err != nil {
		t.Fatalf("StartTag: %v", err)
	}
	if err := enc.EndTag(); err != nil {
		t.Fatalf("EndTag: %v", err)
	}
	want := []byte{0x03, 0x01, 0x6A, 0x00, 0x45, 0x01}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % X, want % X", buf.Bytes(), want)
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_SwitchPage(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if err := enc.StartTag(PageAirSync, 0x05, false, true); err != nil {
		t.Fatalf("StartTag AirSync: %v", err)
	}
	// AirSyncBase is page 17, so encoder must inject SWITCH_PAGE 0x00 0x11
	if err := enc.StartTag(PageAirSyncBase, 0x0A, false, true); err != nil {
		t.Fatalf("StartTag AirSyncBase: %v", err)
	}
	if err := enc.EndTag(); err != nil {
		t.Fatalf("EndTag inner: %v", err)
	}
	if err := enc.EndTag(); err != nil {
		t.Fatalf("EndTag outer: %v", err)
	}
	want := []byte{0x03, 0x01, 0x6A, 0x00, 0x45, 0x00, 0x11, 0x4A, 0x01, 0x01}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % X, want % X", buf.Bytes(), want)
	}
}

// SPEC: MS-ASWBXML/encoder.string
func TestEncoder_StrI(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.StrI("ok"); err != nil {
		t.Fatalf("StrI: %v", err)
	}
	want := []byte{0x03, 'o', 'k', 0x00}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % X, want % X", buf.Bytes(), want)
	}
}

// SPEC: MS-ASWBXML/encoder.opaque
func TestEncoder_Opaque(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Opaque([]byte{0xDE, 0xAD, 0xBE, 0xEF}); err != nil {
		t.Fatalf("Opaque: %v", err)
	}
	want := []byte{0xC3, 0x04, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("bytes = % X, want % X", buf.Bytes(), want)
	}
}

// SPEC: MS-ASWBXML/encoder.tag
func TestEncoder_RejectsUnknownPage(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.StartTag(0xFE, 0x05, false, true); err == nil {
		t.Fatalf("StartTag with unknown page: expected error, got nil")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_StartTagUnknownPage(t *testing.T) {
	enc := NewEncoder(&bytes.Buffer{})
	if err := enc.StartTag(0xFF, 0x05, false, true); err == nil {
		t.Fatal("expected unknown page error")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_WriteHeader(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty header")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_StrIWriteError(t *testing.T) {
	enc := NewEncoder(failingWriter{})
	if err := enc.StrI("x"); err == nil {
		t.Fatal("expected write error")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_OpaqueWriteError(t *testing.T) {
	enc := NewEncoder(failingWriter{})
	if err := enc.Opaque([]byte{1}); err == nil {
		t.Fatal("expected write error")
	}
}

// SPEC: MS-ASWBXML/encoder.string
func TestEncoder_StrIBodyWriteError(t *testing.T) {
	// firstWriteOK with allowed=1 lets the leading STR_I byte through and then
	// fails on the body write, exercising the io.WriteString error branch.
	enc := NewEncoder(&firstWriteOK{allowed: 1})
	if err := enc.StrI("payload"); err == nil {
		t.Fatal("expected body write error")
	}
}

// SPEC: MS-ASWBXML/encoder.string
func TestEncoder_StrITrailingNULError(t *testing.T) {
	// allowed=2 lets the STR_I byte and the body through; the trailing NUL
	// write then fails.
	enc := NewEncoder(&firstWriteOK{allowed: 2})
	if err := enc.StrI("ok"); err == nil {
		t.Fatal("expected trailing-NUL write error")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestEncoder_StartTagSwitchError(t *testing.T) {
	// firstWriteOK with allowed=0 fails on the very first Write, exposing the
	// SWITCH_PAGE branch's error propagation.
	enc := NewEncoder(&firstWriteOK{})
	if err := enc.StartTag(2 /* Email */, 0x05, false, false); err == nil {
		t.Fatal("expected SWITCH_PAGE write failure")
	}
}

// failingWriter rejects every Write call, exposing error paths in encoder
// helpers that bypass the normal byte-buffer Writer.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, errForcedWrite }

var errForcedWrite = errors.New("forced write error")

// firstWriteOK rejects writes once its allowed budget is exhausted; with the
// default zero budget it fails on the very first Write.
type firstWriteOK struct {
	allowed int
}

func (f *firstWriteOK) Write(p []byte) (int, error) {
	if f.allowed <= 0 {
		return 0, errors.New("blocked")
	}
	f.allowed--
	return len(p), nil
}
