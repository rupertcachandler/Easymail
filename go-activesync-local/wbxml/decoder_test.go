package wbxml

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// SPEC: MS-ASWBXML/decoder.tag
// SPEC: MS-ASWBXML/decoder.switch-page
func TestDecoder_TagAndSwitchPage(t *testing.T) {
	// Header + <Sync> <Body in page 17> </Body> </Sync>
	in := []byte{0x03, 0x01, 0x6A, 0x00, 0x45, 0x00, 0x11, 0x4A, 0x01, 0x01}
	dec := NewDecoder(bytes.NewReader(in))
	h, err := dec.ReadHeader()
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	if h.Version != 0x03 || h.PublicID != 0x01 || h.Charset != 0x6A {
		t.Fatalf("Header = %+v", h)
	}

	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken 1: %v", err)
	}
	if tok.Kind != KindTag || tok.Page != PageAirSync || tok.Tag != 0x05 || !tok.HasContent || tok.HasAttrs {
		t.Fatalf("token 1 = %+v", tok)
	}
	tok, err = dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken 2: %v", err)
	}
	if tok.Kind != KindTag || tok.Page != PageAirSyncBase || tok.Tag != 0x0A {
		t.Fatalf("token 2 = %+v (want page=AirSyncBase tag=0x0A)", tok)
	}

	tok, err = dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken 3: %v", err)
	}
	if tok.Kind != KindEnd {
		t.Fatalf("token 3 = %+v (want End)", tok)
	}
	tok, err = dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken 4: %v", err)
	}
	if tok.Kind != KindEnd {
		t.Fatalf("token 4 = %+v (want End)", tok)
	}
	if _, err := dec.NextToken(); !errors.Is(err, io.EOF) {
		t.Fatalf("NextToken 5: want io.EOF, got %v", err)
	}
}

// SPEC: MS-ASWBXML/decoder.string
func TestDecoder_StrI(t *testing.T) {
	in := []byte{0x03, 'o', 'k', 0x00}
	dec := NewDecoder(bytes.NewReader(in))
	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	if tok.Kind != KindString || tok.String != "ok" {
		t.Fatalf("token = %+v", tok)
	}
}

// SPEC: MS-ASWBXML/decoder.opaque
func TestDecoder_Opaque(t *testing.T) {
	in := []byte{0xC3, 0x04, 0xDE, 0xAD, 0xBE, 0xEF}
	dec := NewDecoder(bytes.NewReader(in))
	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	if tok.Kind != KindOpaque {
		t.Fatalf("token kind = %v want Opaque", tok.Kind)
	}
	if !bytes.Equal(tok.Bytes, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Fatalf("bytes = % X", tok.Bytes)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_TagBeforeSwitchPage_OK(t *testing.T) {
	// Default page is 0 (AirSync) and is initialised, so a tag byte at the
	// start of a token stream is valid.
	d := NewDecoder(bytes.NewReader([]byte{0x05}))
	tok, err := d.NextToken()
	if err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	if tok.Kind != KindTag || tok.Page != 0 || tok.Tag != 0x05 {
		t.Fatalf("Token = %+v", tok)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_SwitchPageUnknown(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{SwitchPage, 0xFE, 0x05}))
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected unknown code page error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_SwitchPageTruncated(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{SwitchPage}))
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected SWITCH_PAGE truncation error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_StrTUnknown(t *testing.T) {
	// Header with empty string table, then STR_T offset 5 → out of range.
	var buf bytes.Buffer
	(&Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}).Write(&buf)
	buf.WriteByte(StrT)
	buf.Write(AppendMbUint32(nil, 5))

	d := NewDecoder(&buf)
	if _, err := d.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected STR_T offset error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_StrTTruncatedOffset(t *testing.T) {
	var buf bytes.Buffer
	(&Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}).Write(&buf)
	buf.WriteByte(StrT) // missing offset bytes

	d := NewDecoder(&buf)
	if _, err := d.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected truncated STR_T error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_OpaqueTruncatedHeader(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{Opaque}))
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected OPAQUE length error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_OpaqueTruncatedPayload(t *testing.T) {
	// Length = 5 but only 2 bytes available.
	d := NewDecoder(bytes.NewReader([]byte{Opaque, 0x05, 0xAA, 0xBB}))
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected OPAQUE payload truncation")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_StrIUnterminated(t *testing.T) {
	// STR_I with no trailing NUL.
	d := NewDecoder(bytes.NewReader([]byte{StrI, 'a', 'b'}))
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected unterminated STR_I error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_OpaqueExceedsLimit(t *testing.T) {
	// The cap check fires before allocation, so we don't need to actually
	// supply that many bytes.
	defer swapMaxOpaqueSize(8)()
	buf := append([]byte{Opaque}, AppendMbUint32(nil, 9)...)
	d := NewDecoder(bytes.NewReader(buf))
	_, err := d.NextToken()
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected OPAQUE limit error, got %v", err)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_StrIExceedsLimit(t *testing.T) {
	defer swapMaxInlineStringSize(4)()
	// Five non-NUL bytes ⇒ buffer would grow beyond the cap of 4.
	d := NewDecoder(bytes.NewReader([]byte{StrI, 'a', 'b', 'c', 'd', 'e', 0}))
	_, err := d.NextToken()
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected STR_I limit error, got %v", err)
	}
}

func swapMaxOpaqueSize(v uint32) func() {
	old := MaxOpaqueSize
	MaxOpaqueSize = v
	return func() { MaxOpaqueSize = old }
}

func swapMaxInlineStringSize(v int) func() {
	old := MaxInlineStringSize
	MaxInlineStringSize = v
	return func() { MaxInlineStringSize = old }
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecoder_StringTableLookup(t *testing.T) {
	// Header with table "hi\0bye\0".
	var buf bytes.Buffer
	(&Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A, StringTable: []byte("hi\x00bye\x00")}).Write(&buf)
	buf.WriteByte(StrT)
	buf.Write(AppendMbUint32(nil, 3)) // offset 3 = "bye"

	d := NewDecoder(&buf)
	if _, err := d.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	tok, err := d.NextToken()
	if err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	if tok.Kind != KindString || tok.String != "bye" {
		t.Fatalf("Token = %+v", tok)
	}
}

// SPEC: MS-ASWBXML/decoder.string
func TestDecoder_StringTable(t *testing.T) {
	// Header with string-table populated, body referring offset 0 via STR_T.
	doc := []byte{
		0x03, 0x01, 0x6A, // version=1.3, publicid=1, charset=UTF-8 (mb_u_int32 0x6A)
		0x05, // string-table length = 5
		'h', 'i', 0,
		'!', 0,
		// Tag: AirSync.Sync (page 0, tag 0x05) with content
		0x45,
		// STR_T at offset 0
		0x83, 0x00,
		// END
		0x01,
	}
	dec := NewDecoder(bytes.NewReader(doc))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("first NextToken: %v", err)
	}
	if tok.Kind != KindTag {
		t.Fatalf("first kind = %s", tok.Kind)
	}
	str, err := dec.NextToken()
	if err != nil {
		t.Fatalf("string token: %v", err)
	}
	if str.Kind != KindString || str.String != "hi" {
		t.Fatalf("string token = %+v", str)
	}
	end, err := dec.NextToken()
	if err != nil {
		t.Fatalf("end token: %v", err)
	}
	if end.Kind != KindEnd {
		t.Fatalf("end token = %+v", end)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestNextToken_ReadByteError(t *testing.T) {
	d := NewDecoder(&errReader{})
	if _, err := d.NextToken(); err == nil {
		t.Fatal("expected ReadByte error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestByteReader_PassThrough(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("hi"))
	if got := byteReader(br); got != br {
		t.Fatal("byteReader did not pass through existing ByteReader")
	}
	got := byteReader(strings.NewReader("hi"))
	if _, ok := any(got).(io.ByteReader); !ok {
		t.Fatal("byteReader did not adapt plain Reader")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestByteReader_BufferedReader(t *testing.T) {
	br := bufio.NewReader(strings.NewReader("ab"))
	if got := byteReader(br); got != br {
		t.Fatal("expected pass-through")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestByteReader_FallbackToBufio(t *testing.T) {
	pr := &plainReader{r: bytes.NewReader([]byte{0x03, 0x01, 0x6A, 0x00})}
	var h Header
	if err := h.Read(pr); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if h.Version != 0x03 {
		t.Fatalf("Version = %x", h.Version)
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestTokenKind_String(t *testing.T) {
	cases := []struct {
		k    TokenKind
		want string
	}{
		{KindTag, "Tag"}, {KindEnd, "End"}, {KindString, "String"},
		{KindOpaque, "Opaque"}, {TokenKind(99), "TokenKind(99)"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}

// errReader fails the very first ReadByte after a header is parsed, exposing
// EOF propagation paths in NextToken.
type errReader struct{ remaining []byte }

func (r *errReader) Read(p []byte) (int, error) {
	if len(r.remaining) == 0 {
		return 0, errors.New("forced read error")
	}
	n := copy(p, r.remaining)
	r.remaining = r.remaining[n:]
	return n, nil
}

// plainReader exposes only io.Reader, never io.ByteReader, forcing the byteReader
// adapter to wrap the reader in a bufio.Reader.
type plainReader struct{ r io.Reader }

func (p *plainReader) Read(b []byte) (int, error) { return p.r.Read(b) }
