package wbxml

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"
)

// SPEC: OMA-WBXML-1.3/global.SWITCH_PAGE
// SPEC: OMA-WBXML-1.3/global.END
// SPEC: OMA-WBXML-1.3/global.STR_I
// SPEC: OMA-WBXML-1.3/global.STR_T
// SPEC: OMA-WBXML-1.3/global.OPAQUE
func TestGlobalTokens_Constants(t *testing.T) {
	cases := []struct {
		name string
		got  byte
		want byte
	}{
		{"SwitchPage", SwitchPage, 0x00},
		{"End", End, 0x01},
		{"Entity", Entity, 0x02},
		{"StrI", StrI, 0x03},
		{"Literal", Literal, 0x04},
		{"ExtI0", ExtI0, 0x40},
		{"ExtI1", ExtI1, 0x41},
		{"ExtI2", ExtI2, 0x42},
		{"PI", PI, 0x43},
		{"LiteralC", LiteralC, 0x44},
		{"ExtT0", ExtT0, 0x80},
		{"ExtT1", ExtT1, 0x81},
		{"ExtT2", ExtT2, 0x82},
		{"StrT", StrT, 0x83},
		{"LiteralA", LiteralA, 0x84},
		{"Ext0", Ext0, 0xC0},
		{"Ext1", Ext1, 0xC1},
		{"Ext2", Ext2, 0xC2},
		{"Opaque", Opaque, 0xC3},
		{"LiteralAC", LiteralAC, 0xC4},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = 0x%02X, want 0x%02X", c.name, c.got, c.want)
		}
	}
}

// SPEC: OMA-WBXML-1.3/tag.bits
func TestTagBits(t *testing.T) {
	cases := []struct {
		name       string
		b          byte
		hasAttrs   bool
		hasContent bool
		identity   byte
	}{
		{"plain", 0x05, false, false, 0x05},
		{"content", 0x45, false, true, 0x05},
		{"attrs", 0x85, true, false, 0x05},
		{"both", 0xC5, true, true, 0x05},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TagHasAttributes(c.b); got != c.hasAttrs {
				t.Errorf("TagHasAttributes(0x%02X) = %v, want %v", c.b, got, c.hasAttrs)
			}
			if got := TagHasContent(c.b); got != c.hasContent {
				t.Errorf("TagHasContent(0x%02X) = %v, want %v", c.b, got, c.hasContent)
			}
			if got := TagIdentity(c.b); got != c.identity {
				t.Errorf("TagIdentity(0x%02X) = 0x%02X, want 0x%02X", c.b, got, c.identity)
			}
			if got := EncodeTag(c.identity, c.hasAttrs, c.hasContent); got != c.b {
				t.Errorf("EncodeTag(0x%02X, %v, %v) = 0x%02X, want 0x%02X", c.identity, c.hasAttrs, c.hasContent, got, c.b)
			}
		})
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
// SPEC: OMA-WBXML-1.3/mb_u_int32.boundaries
func TestMbUint32_RoundTrip(t *testing.T) {
	cases := []struct {
		v       uint32
		encoded []byte
	}{
		{0x00000000, []byte{0x00}},
		{0x0000007F, []byte{0x7F}},
		{0x00000080, []byte{0x81, 0x00}},
		{0x000000A4, []byte{0x81, 0x24}},
		{0x00003FFF, []byte{0xFF, 0x7F}},
		{0x00004000, []byte{0x81, 0x80, 0x00}},
		{0x001FFFFF, []byte{0xFF, 0xFF, 0x7F}},
		{0x00200000, []byte{0x81, 0x80, 0x80, 0x00}},
		{0x0FFFFFFF, []byte{0xFF, 0xFF, 0xFF, 0x7F}},
		{0x10000000, []byte{0x81, 0x80, 0x80, 0x80, 0x00}},
		{0xFFFFFFFF, []byte{0x8F, 0xFF, 0xFF, 0xFF, 0x7F}},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := WriteMbUint32(&buf, c.v); err != nil {
			t.Fatalf("WriteMbUint32(%#x): %v", c.v, err)
		}
		if !bytes.Equal(buf.Bytes(), c.encoded) {
			t.Errorf("WriteMbUint32(%#x) = % X, want % X", c.v, buf.Bytes(), c.encoded)
		}
		got, n, err := ReadMbUint32(bytes.NewReader(c.encoded))
		if err != nil {
			t.Fatalf("ReadMbUint32(% X): %v", c.encoded, err)
		}
		if got != c.v {
			t.Errorf("ReadMbUint32(% X) = %#x, want %#x", c.encoded, got, c.v)
		}
		if n != len(c.encoded) {
			t.Errorf("ReadMbUint32(% X) consumed %d bytes, want %d", c.encoded, n, len(c.encoded))
		}
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestMbUint32_Truncated(t *testing.T) {
	if _, _, err := ReadMbUint32(bytes.NewReader([]byte{0x81})); !errors.Is(err, io.ErrUnexpectedEOF) && err == nil {
		t.Fatalf("expected error on truncated mb_u_int32")
	}
}

// SPEC: OMA-WBXML-1.3/header.version
// SPEC: OMA-WBXML-1.3/header.publicid
// SPEC: OMA-WBXML-1.3/header.charset
// SPEC: OMA-WBXML-1.3/header.stringtable
func TestHeader_RoundTripDefault(t *testing.T) {
	h := Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}
	var buf bytes.Buffer
	if err := h.Write(&buf); err != nil {
		t.Fatalf("Header.Write: %v", err)
	}
	want := []byte{0x03, 0x01, 0x6A, 0x00}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("Header bytes = % X, want % X", buf.Bytes(), want)
	}
	var got Header
	if err := got.Read(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Header.Read: %v", err)
	}
	if got.Version != 0x03 || got.PublicID != 0x01 || got.Charset != 0x6A || len(got.StringTable) != 0 {
		t.Fatalf("Header.Read got %+v, want default", got)
	}
}

// SPEC: OMA-WBXML-1.3/header.stringtable
func TestHeader_StringTable(t *testing.T) {
	h := Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A, StringTable: []byte("hello\x00world\x00")}
	var buf bytes.Buffer
	if err := h.Write(&buf); err != nil {
		t.Fatalf("Header.Write: %v", err)
	}
	want := append([]byte{0x03, 0x01, 0x6A, byte(len(h.StringTable))}, h.StringTable...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("Header bytes = % X, want % X", buf.Bytes(), want)
	}
	var got Header
	if err := got.Read(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Header.Read: %v", err)
	}
	if !bytes.Equal(got.StringTable, h.StringTable) {
		t.Fatalf("Header.Read StringTable = % X, want % X", got.StringTable, h.StringTable)
	}
}

// SPEC: OMA-WBXML-1.3/header.version
func TestHeader_StringTable_RoundTripShort(t *testing.T) {
	src := Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A, StringTable: []byte("a\x00b\x00")}
	var buf bytes.Buffer
	if err := src.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var got Header
	if err := got.Read(&buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got.StringTable, src.StringTable) {
		t.Fatalf("StringTable=%v want %v", got.StringTable, src.StringTable)
	}
}

// SPEC: OMA-WBXML-1.3/header.version
func TestHeader_PublicIDFromTable(t *testing.T) {
	// Encoded form: version, 0x00, mbu(7), charset, table-len 0.
	raw := []byte{0x03, 0x00, 0x07, 0x6A, 0x00}
	var h Header
	if err := h.Read(bytes.NewReader(raw)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if h.PublicID != 7 {
		t.Fatalf("PublicID = %d", h.PublicID)
	}
}

// SPEC: OMA-WBXML-1.3/header.version
func TestHeader_ReadTruncated(t *testing.T) {
	cases := [][]byte{
		{},                                 // no version
		{0x03},                             // missing public id
		{0x03, 0x00},                       // missing public-id offset after 0
		{0x03, 0x01},                       // missing charset
		{0x03, 0x01, 0x6A},                 // missing string-table length
		{0x03, 0x01, 0x6A, 0x05, 'a', 'b'}, // truncated string table
	}
	for i, raw := range cases {
		var h Header
		if err := h.Read(bytes.NewReader(raw)); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

// SPEC: OMA-WBXML-1.3/header.version
func TestHeader_StringTableLargerStrings(t *testing.T) {
	src := Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A, StringTable: bytes.Repeat([]byte{'a'}, 200)}
	src.StringTable = append(src.StringTable, 0)
	var buf bytes.Buffer
	if err := src.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var got Header
	if err := got.Read(&buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got.StringTable, src.StringTable) {
		t.Fatal("StringTable mismatch")
	}
}

// SPEC: OMA-WBXML-1.3/header.stringtable
func TestHeader_StringTableExceedsLimit(t *testing.T) {
	old := MaxStringTableSize
	MaxStringTableSize = 4
	defer func() { MaxStringTableSize = old }()
	// version=1.3, publicid=1, charset=UTF-8, stringtable len=10 (no payload bytes
	// needed because the cap check fires before io.ReadFull).
	raw := []byte{0x03, 0x01, 0x6A, 0x0A}
	if err := (&Header{}).Read(bytes.NewReader(raw)); err == nil {
		t.Fatal("expected string-table limit error")
	}
}

// SPEC: OMA-WBXML-1.3/header.version
func TestHeader_StringTableReadError(t *testing.T) {
	src := bytes.NewReader([]byte{0x03, 0x01, 0x6A, 0x05, 'a', 'b'})
	if err := (&Header{}).Read(&stallReader{r: src}); err == nil {
		t.Fatal("expected string-table read error")
	}
}

// SPEC: MS-ASWBXML/encoder.switch-page
func TestHeader_WriteError(t *testing.T) {
	if err := (&Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}).Write(&firstWriteOK{}); err == nil {
		t.Fatal("expected write failure")
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestReadMbUint32_Errors(t *testing.T) {
	// 6 bytes with high bit set → too long.
	br := bufio.NewReader(bytes.NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x01}))
	if _, _, err := ReadMbUint32(br); err == nil {
		t.Fatal("expected too-long error")
	}
	// EOF before any byte.
	br2 := bufio.NewReader(bytes.NewReader(nil))
	if _, _, err := ReadMbUint32(br2); err == nil {
		t.Fatal("expected EOF error")
	}
	// EOF mid-sequence.
	br3 := bufio.NewReader(bytes.NewReader([]byte{0x80}))
	if _, _, err := ReadMbUint32(br3); err == nil {
		t.Fatal("expected unexpected EOF error")
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestWriteMbUint32_RoundTripSingle(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMbUint32(&buf, 300); err != nil {
		t.Fatalf("WriteMbUint32: %v", err)
	}
	got, n, err := ReadMbUint32(bufio.NewReader(&buf))
	if err != nil || got != 300 {
		t.Fatalf("round-trip got %d (n=%d, err=%v)", got, n, err)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestEncodeTag_PanicsOnLargeIdentity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = EncodeTag(0x40, false, false)
}

// stallReader returns a non-EOF error on the second Read so that io.ReadFull
// surfaces it during string-table reads.
type stallReader struct {
	r       io.Reader
	stalled bool
}

func (s *stallReader) Read(b []byte) (int, error) {
	if !s.stalled {
		s.stalled = true
		return s.r.Read(b)
	}
	return 0, errors.New("read stall")
}
