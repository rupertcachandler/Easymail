package wbxml

import (
	"bytes"
	"testing"
)

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_NonStruct(t *testing.T) {
	x := 42
	if _, err := Marshal(&x); err == nil {
		t.Fatal("expected error for non-struct pointer target")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_NilTopLevel(t *testing.T) {
	if _, err := Marshal(nil); err == nil {
		t.Fatal("expected error for nil")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_RejectsUnknownPage(t *testing.T) {
	type bad struct {
		XMLName struct{} `wbxml:"NotAPage.Sync"`
	}
	if _, err := Marshal(&bad{}); err == nil {
		t.Fatalf("Marshal: expected error for unknown page")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_RejectsBadTagOption(t *testing.T) {
	type bad struct {
		XMLName struct{} `wbxml:"AirSync.Sync,wrongoption"`
	}
	if _, err := Marshal(&bad{}); err == nil {
		t.Fatalf("Marshal: expected error for bad option")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_RejectsMissingXMLName(t *testing.T) {
	type bad struct {
		Field string `wbxml:"AirSync.SyncKey"`
	}
	if _, err := Marshal(&bad{}); err == nil {
		t.Fatalf("Marshal: expected error for missing XMLName")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
type noXMLName struct {
	K string `wbxml:"AirSync.SyncKey"`
}

func TestMarshal_NoXMLName(t *testing.T) {
	if _, err := Marshal(&noXMLName{}); err == nil {
		t.Fatal("expected missing XMLName error")
	}
}

func TestUnmarshal_NoXMLName(t *testing.T) {
	if err := Unmarshal([]byte{0x03, 0x01, 0x6A, 0x00, 0x05, 0x01}, &noXMLName{}); err == nil {
		t.Fatal("expected missing XMLName error")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
type unsupportedField struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Val     float64  `wbxml:"AirSync.SyncKey"`
}

func TestMarshal_UnsupportedKind(t *testing.T) {
	in := unsupportedField{Val: 1.5}
	if _, err := Marshal(&in); err == nil {
		t.Fatal("expected unsupported field error")
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceUnsupported struct {
	XMLName struct{}  `wbxml:"AirSync.Sync"`
	Vals    []float64 `wbxml:"AirSync.SyncKey"`
}

func TestMarshal_SliceOfUnsupportedKind(t *testing.T) {
	in := sliceUnsupported{Vals: []float64{1.0}}
	if _, err := Marshal(&in); err == nil {
		t.Fatal("expected unsupported slice elem error")
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceFloatPtr struct {
	XMLName struct{}   `wbxml:"AirSync.Sync"`
	Vals    []*float64 `wbxml:"AirSync.SyncKey"`
}

func TestMarshal_SliceUnsupportedPtrElem(t *testing.T) {
	v := 1.0
	in := sliceFloatPtr{Vals: []*float64{&v}}
	if _, err := Marshal(&in); err == nil {
		t.Fatal("expected unsupported slice ptr elem error")
	}
}

// Marshal-time error from a sub-struct with an unknown tag option.
type badInner struct {
	X string `wbxml:"AirSync.SyncKey,wat"`
}
type badOuter struct {
	XMLName struct{}  `wbxml:"AirSync.Sync"`
	Inner   *badInner `wbxml:"AirSync.Collection"`
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_NestedInfoForError(t *testing.T) {
	in := badOuter{Inner: &badInner{X: "y"}}
	if _, err := Marshal(&in); err == nil {
		t.Fatal("expected nested infoFor error")
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestUnmarshal_RootMismatch(t *testing.T) {
	// Build a doc whose root is AirSync.SyncKey rather than AirSync.Sync.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if err := enc.StartTag(0, 0x0B /* SyncKey */, false, false); err != nil {
		t.Fatalf("StartTag: %v", err)
	}
	if err := Unmarshal(buf.Bytes(), &allKinds{}); err == nil {
		t.Fatal("expected root-tag mismatch")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestUnmarshal_HeaderEOF(t *testing.T) {
	if err := Unmarshal(nil, &allKinds{}); err == nil {
		t.Fatal("expected header read error")
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestAssignScalar_Errors(t *testing.T) {
	type intHolder struct {
		XMLName struct{} `wbxml:"AirSync.Sync"`
		N       int32    `wbxml:"AirSync.WindowSize"`
	}
	type uintHolder struct {
		XMLName struct{} `wbxml:"AirSync.Sync"`
		N       uint32   `wbxml:"AirSync.WindowSize"`
	}
	type boolHolder struct {
		XMLName struct{} `wbxml:"AirSync.Sync"`
		B       bool     `wbxml:"AirSync.GetChanges"`
	}
	build := func(tagID byte, value string) []byte {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
		enc.StartTag(0, 0x05, false, true)
		enc.StartTag(0, tagID, false, true)
		enc.StrI(value)
		enc.EndTag()
		enc.EndTag()
		return buf.Bytes()
	}
	if err := Unmarshal(build(0x15 /* WindowSize */, "abc"), &intHolder{}); err == nil {
		t.Errorf("int parse error expected")
	}
	if err := Unmarshal(build(0x15, "-1"), &uintHolder{}); err == nil {
		t.Errorf("uint parse error expected")
	}
	if err := Unmarshal(build(0x13 /* GetChanges */, "maybe"), &boolHolder{}); err == nil {
		t.Errorf("bool parse error expected")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestMarshal_RejectsNonPointer(t *testing.T) {
	if _, err := Marshal(123); err == nil {
		t.Fatalf("expected error on non-struct")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestUnmarshal_RejectsNonPointer(t *testing.T) {
	if err := Unmarshal([]byte{0x03, 0x01, 0x6A, 0x00}, allKinds{}); err == nil {
		t.Fatalf("expected error on non-pointer")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestUnmarshal_RejectsTruncated(t *testing.T) {
	if err := Unmarshal([]byte{}, &allKinds{}); err == nil {
		t.Fatalf("expected error on empty input")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestParseTag_RejectsBadFormats(t *testing.T) {
	if _, err := parseTag("AirSyncSync"); err == nil {
		t.Errorf("expected error for missing dot")
	}
	if _, err := parseTag("AirSync.NotATag"); err == nil {
		t.Errorf("expected error for unknown tag")
	}
	if _, err := parseTag("Bogus.Sync"); err == nil {
		t.Errorf("expected error for unknown page")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestParseTag_AllErrors(t *testing.T) {
	cases := []string{"NoDot", "AirSync.NoSuchTag", "BadPage.Sync", "AirSync.Sync,bogus"}
	for _, c := range cases {
		if _, err := parseTag(c); err == nil {
			t.Errorf("parseTag(%q): expected error", c)
		}
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
func TestParseTag_EmptyOption(t *testing.T) {
	spec, err := parseTag("AirSync.Sync,,omitempty")
	if err != nil {
		t.Fatalf("parseTag: %v", err)
	}
	if !spec.omitempty {
		t.Fatal("omitempty not set")
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestSkipElement_TruncatedReturnsErr(t *testing.T) {
	// Missing END after the opening tag triggers skipElement's read error path.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05, false, true)
	enc.StartTag(0, 0x10 /* Class */, false, true)
	// no end markers
	if err := Unmarshal(buf.Bytes(), &wrappedTag{}); err == nil {
		t.Fatal("expected truncation error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestReadElementValue_NestedTagError(t *testing.T) {
	// Build a doc where SyncKey contains a nested tag (not allowed for scalar).
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05, false, true)
	enc.StartTag(0, 0x0B /* SyncKey */, false, true)
	enc.StartTag(0, 0x15 /* WindowSize */, false, false)
	enc.EndTag()
	enc.EndTag()

	if err := Unmarshal(buf.Bytes(), &wrappedTag{}); err == nil {
		t.Fatal("expected nested-tag-in-scalar error")
	}
}

// SPEC: MS-ASWBXML/decoder.tag
func TestDecodeField_UnexpectedToken(t *testing.T) {
	// Construct a doc whose Sync content begins with a STR_I instead of a tag.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StrI("loose")
	enc.EndTag()

	if err := Unmarshal(buf.Bytes(), &wrappedTag{}); err == nil {
		t.Fatal("expected unexpected-token error inside struct")
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceUnsupportedDecode struct {
	XMLName struct{}  `wbxml:"AirSync.Sync"`
	Vals    []float64 `wbxml:"AirSync.SyncKey"`
}

func TestUnmarshal_SliceUnsupportedKind(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x0B /* SyncKey */, false, true)
	enc.StrI("x")
	enc.EndTag()
	enc.EndTag()
	if err := Unmarshal(buf.Bytes(), &sliceUnsupportedDecode{}); err == nil {
		t.Fatal("expected unsupported slice elem kind error")
	}
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
type unsupportedDecode struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Val     float64  `wbxml:"AirSync.SyncKey"`
}

func TestUnmarshal_UnsupportedFieldKind(t *testing.T) {
	// Hand-craft a doc that has Sync→SyncKey="x"→end. Decoding into float64
	// must produce an error from decodeField's default branch.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x0B /* SyncKey */, false, true)
	enc.StrI("1.5")
	enc.EndTag()
	enc.EndTag()

	if err := Unmarshal(buf.Bytes(), &unsupportedDecode{}); err == nil {
		t.Fatal("expected unsupported field-kind error")
	}
}
