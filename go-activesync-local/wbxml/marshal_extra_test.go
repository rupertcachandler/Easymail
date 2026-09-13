package wbxml

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

type scalarSliceMsg struct {
	XMLName  struct{} `wbxml:"AirSync.Sync"`
	Children []string `wbxml:"AirSync.SyncKey"`
	Counts   []int32  `wbxml:"AirSync.WindowSize"`
	Flags    []bool   `wbxml:"AirSync.GetChanges"`
}

// SPEC: MS-ASWBXML/marshal.slice
func TestMarshal_SliceOfScalar(t *testing.T) {
	in := scalarSliceMsg{
		Children: []string{"a", "b", "c"},
		Counts:   []int32{1, 2},
		Flags:    []bool{true, false},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out scalarSliceMsg
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

type uintMsg struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Window  uint32   `wbxml:"AirSync.WindowSize"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_UintScalar(t *testing.T) {
	in := uintMsg{Window: 42}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out uintMsg
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

type ptrChildMsg struct {
	XMLName struct{}  `wbxml:"AirSync.Sync"`
	Inner   *innerMsg `wbxml:"AirSync.Collection,omitempty"`
}

type innerMsg struct {
	Name string `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_PointerStruct(t *testing.T) {
	in := ptrChildMsg{Inner: &innerMsg{Name: "abc"}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ptrChildMsg
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Inner == nil || out.Inner.Name != "abc" {
		t.Fatalf("Inner = %+v", out.Inner)
	}
}

type simpleSync struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	SyncKey string   `wbxml:"AirSync.SyncKey,omitempty"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestUnmarshal_IgnoresUnknownChild(t *testing.T) {
	in := struct {
		XMLName struct{} `wbxml:"AirSync.Sync"`
		Key     string   `wbxml:"AirSync.SyncKey"`
		Extra   string   `wbxml:"AirSync.Class"`
	}{Key: "k", Extra: "e"}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out simpleSync
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.SyncKey != "k" {
		t.Fatalf("SyncKey = %q", out.SyncKey)
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type slicePtrNil struct {
	XMLName struct{}       `wbxml:"AirSync.Sync"`
	Items   []*innerScalar `wbxml:"AirSync.Collection"`
}

type innerScalar struct {
	K string `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.slice
func TestMarshal_SlicePtrWithNilEntries(t *testing.T) {
	in := slicePtrNil{Items: []*innerScalar{nil, {K: "x"}}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out slicePtrNil
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].K != "x" {
		t.Fatalf("Items = %+v", out.Items)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type allOmit struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Bytes   []byte   `wbxml:"AirSync.Class,omitempty"`
}

func TestMarshal_OmitEmptyBytes(t *testing.T) {
	in := allOmit{}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got allOmit
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, got) {
		t.Fatalf("round-trip mismatch")
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceStructInner struct {
	K string `wbxml:"AirSync.SyncKey"`
}
type sliceStructDirect struct {
	XMLName struct{}           `wbxml:"AirSync.Sync"`
	Items   []sliceStructInner `wbxml:"AirSync.Collection"`
}

func TestMarshal_SliceOfStructDirect(t *testing.T) {
	in := sliceStructDirect{Items: []sliceStructInner{{K: "a"}, {K: "b"}}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out sliceStructDirect
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Items) != 2 || out.Items[0].K != "a" || out.Items[1].K != "b" {
		t.Fatalf("Items = %+v", out.Items)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type ptrStructHolder struct {
	XMLName struct{}          `wbxml:"AirSync.Sync"`
	Inner   *sliceStructInner `wbxml:"AirSync.Collection"`
}

func TestMarshal_PointerStructFilled(t *testing.T) {
	in := ptrStructHolder{Inner: &sliceStructInner{K: "p"}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ptrStructHolder
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Inner == nil || out.Inner.K != "p" {
		t.Fatalf("Inner = %+v", out.Inner)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type byteSlicePlain struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Class   []byte   `wbxml:"AirSync.Class"`
}

func TestMarshal_ByteSliceNonOpaque(t *testing.T) {
	in := byteSlicePlain{Class: []byte("Email")}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out byteSlicePlain
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(out.Class) != "Email" {
		t.Fatalf("Class = %q", out.Class)
	}
}

// SPEC: MS-ASWBXML/marshal.omitempty
type ptrOmit struct {
	XMLName struct{}     `wbxml:"AirSync.Sync"`
	Inner   *innerScalar `wbxml:"AirSync.Collection,omitempty"`
}

func TestMarshal_NilPointerWithOmitempty(t *testing.T) {
	in := ptrOmit{}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ptrOmit
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Inner != nil {
		t.Fatalf("Inner should be nil")
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceBoolHolder struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Bs      []bool   `wbxml:"AirSync.GetChanges"`
}

func TestMarshal_SliceOfBoolTrue(t *testing.T) {
	in := sliceBoolHolder{Bs: []bool{true, false, true}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out sliceBoolHolder
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in.Bs, out.Bs) {
		t.Fatalf("Bs = %v, want %v", out.Bs, in.Bs)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type byteSliceHolder struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Class   []byte   `wbxml:"AirSync.Class,opaque"`
}

func TestUnmarshal_OpaqueByteSliceWithoutContent(t *testing.T) {
	// Build doc: Sync (content) → Class (no content) → End.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x10 /* Class */, false, false)
	enc.EndTag()
	var out byteSliceHolder
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Class != nil {
		t.Fatalf("Class = %v", out.Class)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type sliceIntHolder struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Vals    []int32  `wbxml:"AirSync.WindowSize"`
}

func TestUnmarshal_SliceOfScalarWithoutContent(t *testing.T) {
	// Build doc: Sync (content) → WindowSize (no content) → end.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x15 /* WindowSize */, false, false)
	enc.EndTag()

	var out sliceIntHolder
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Vals) != 1 {
		t.Fatalf("Vals = %v", out.Vals)
	}
}

// SPEC: MS-ASWBXML/marshal.slice
type sliceUintHolder struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Vals    []uint32 `wbxml:"AirSync.MIMETruncation"`
}

func TestMarshal_SliceOfUint(t *testing.T) {
	in := sliceUintHolder{Vals: []uint32{1, 2, 3}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out sliceUintHolder
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in.Vals, out.Vals) {
		t.Fatalf("Vals = %v", out.Vals)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_OmitEmptyZeroScalars(t *testing.T) {
	type allZero struct {
		XMLName struct{} `wbxml:"AirSync.Sync"`
		S       string   `wbxml:"AirSync.SyncKey,omitempty"`
		B       bool     `wbxml:"AirSync.GetChanges,omitempty"`
		I       int32    `wbxml:"AirSync.WindowSize,omitempty"`
		U       uint32   `wbxml:"AirSync.MIMETruncation,omitempty"`
	}
	in := allZero{}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out allZero
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
type wrappedTag struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Inner   string   `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestUnmarshal_ScalarWithoutContent(t *testing.T) {
	// Sync (with content) → SyncKey (no content) → end.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x0B /* SyncKey */, false, false)
	enc.EndTag()

	var out wrappedTag
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Inner != "" {
		t.Fatalf("Inner = %q", out.Inner)
	}
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestSkipElement_Nested(t *testing.T) {
	// Build a doc with a tag we know but inside it a deeply nested unknown
	// child tree, ensuring decodeStruct calls skipElement which recurses.
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A})
	enc.StartTag(0, 0x05 /* Sync */, false, true)
	enc.StartTag(0, 0x0B /* SyncKey */, false, true)
	enc.StrI("k")
	enc.EndTag()
	// Unknown nested subtree (Class) with content and grand-children.
	enc.StartTag(0, 0x10 /* Class */, false, true)
	enc.StartTag(0, 0x15 /* WindowSize */, false, true)
	enc.StrI("1")
	enc.EndTag()
	enc.EndTag()
	enc.EndTag()

	var out wrappedTag
	if err := Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Inner != "k" {
		t.Fatalf("Inner = %q", out.Inner)
	}
}

type allKinds struct {
	XMLName  struct{} `wbxml:"AirSync.Sync"`
	Str      string   `wbxml:"AirSync.SyncKey"`
	Bln      bool     `wbxml:"AirSync.GetChanges"`
	Int32V   int32    `wbxml:"AirSync.WindowSize"`
	Uint32V  uint32   `wbxml:"AirSync.MIMETruncation"`
	BytesRaw []byte   `wbxml:"AirSync.Class"`
	BytesOp  []byte   `wbxml:"AirSync.ApplicationData,opaque"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_AllScalarKinds(t *testing.T) {
	in := allKinds{
		Str:      "key",
		Bln:      true,
		Int32V:   100,
		Uint32V:  200,
		BytesRaw: []byte("text"),
		BytesOp:  []byte{0x00, 0xFF, 0x10},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out allKinds
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

type omitMe struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Str     string   `wbxml:"AirSync.SyncKey,omitempty"`
	Bln     bool     `wbxml:"AirSync.GetChanges,omitempty"`
	Int32V  int32    `wbxml:"AirSync.WindowSize,omitempty"`
	Uint32V uint32   `wbxml:"AirSync.MIMETruncation,omitempty"`
}

// SPEC: MS-ASWBXML/marshal.omitempty
func TestMarshal_OmitemptyAllZero(t *testing.T) {
	in := omitMe{}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// header(4) + opening empty tag (no content) for AirSync.Sync.
	if !bytes.Contains(data, []byte{0x05}) {
		t.Fatalf("expected Sync tag in output, got %x", data)
	}
}

type nilPtr struct {
	XMLName struct{}    `wbxml:"AirSync.Sync"`
	Inner   *innerEmpty `wbxml:"AirSync.Collection"`
}

type innerEmpty struct {
	Key string `wbxml:"AirSync.SyncKey,omitempty"`
}

// SPEC: MS-ASWBXML/marshal.omitempty
func TestMarshal_NilPointerWithoutOmitempty(t *testing.T) {
	in := nilPtr{}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out nilPtr
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
}

type sliceStructPtr struct {
	XMLName struct{}     `wbxml:"AirSync.Sync"`
	Items   []*innerItem `wbxml:"AirSync.Collection"`
}

type innerItem struct {
	Key string `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.slice
func TestMarshal_SliceOfPointerStruct(t *testing.T) {
	in := sliceStructPtr{
		Items: []*innerItem{{Key: "a"}, {Key: "b"}},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out sliceStructPtr
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Items) != 2 || out.Items[0].Key != "a" || out.Items[1].Key != "b" {
		t.Fatalf("Items = %+v", out.Items)
	}
}

type withSkippable struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Key     string   `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.roundtrip
func TestUnmarshal_SkipNestedUnknown(t *testing.T) {
	// Build a document with a nested unknown element to exercise skipElement.
	src := struct {
		XMLName struct{}   `wbxml:"AirSync.Sync"`
		Key     string     `wbxml:"AirSync.SyncKey"`
		Extra   *withInner `wbxml:"AirSync.Class"`
	}{Key: "k", Extra: &withInner{Inner: &withInnerInner{Name: "x"}}}
	data, err := Marshal(&src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out withSkippable
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Key != "k" {
		t.Fatalf("Key = %q", out.Key)
	}
}

type withInner struct {
	Inner *withInnerInner `wbxml:"AirSync.Collection"`
}

type withInnerInner struct {
	Name string `wbxml:"AirSync.SyncKey"`
}

// SPEC: MS-ASWBXML/marshal.omitempty
func TestIsZero_MapAndPointer(t *testing.T) {
	m := map[string]int(nil)
	if !isZero(reflect.ValueOf(m)) {
		t.Fatal("nil map not zero")
	}
	mp := map[string]int{"a": 1}
	if isZero(reflect.ValueOf(mp)) {
		t.Fatal("populated map zero")
	}
	var ptr *int
	if !isZero(reflect.ValueOf(ptr)) {
		t.Fatal("nil pointer not zero")
	}
}

type rawOuter struct {
	XMLName struct{}    `wbxml:"AirSync.Sync"`
	SyncKey string      `wbxml:"AirSync.SyncKey"`
	AppData *RawElement `wbxml:"AirSync.ApplicationData,omitempty,raw"`
}

type rawAppData struct {
	XMLName struct{} `wbxml:"AirSync.ApplicationData"`
	Subject string   `wbxml:"Email.Subject"`
}

// emailBodyBytes returns the WBXML body of an Email-class ApplicationData
// element containing a single Subject child. The bytes are produced as if the
// encoder were already in PageAirSync (ApplicationData's own page); the
// leading SWITCH_PAGE Email transition required to interpret Subject is part
// of the returned slice.
func emailBodyBytes(t *testing.T, subject string) []byte {
	t.Helper()
	var w bytes.Buffer
	e := NewEncoder(&w)
	if err := e.StartTag(PageEmail, 0x14, false, true); err != nil { // Email.Subject
		t.Fatalf("StartTag: %v", err)
	}
	if err := e.StrI(subject); err != nil {
		t.Fatalf("StrI: %v", err)
	}
	if err := e.EndTag(); err != nil {
		t.Fatalf("EndTag: %v", err)
	}
	return w.Bytes()
}

// SPEC: MS-ASWBXML/marshal.raw
func TestMarshal_RawElement_RoundTrip(t *testing.T) {
	body := emailBodyBytes(t, "hello")
	in := rawOuter{
		SyncKey: "1",
		AppData: &RawElement{Page: PageAirSync, Bytes: body},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out rawOuter
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.AppData == nil {
		t.Fatal("RawElement is nil after round-trip")
	}
	if out.AppData.Page != PageAirSync {
		t.Fatalf("RawElement.Page mismatch: got %d want %d", out.AppData.Page, PageAirSync)
	}
	if !bytes.Equal(out.AppData.Bytes, body) {
		t.Fatalf("RawElement.Bytes mismatch:\n got %x\nwant %x", out.AppData.Bytes, body)
	}

	data2, err := Marshal(&out)
	if err != nil {
		t.Fatalf("Marshal(out): %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("re-marshal not byte-identical:\n got %x\nwant %x", data2, data)
	}

	var rebuilt struct {
		XMLName struct{}    `wbxml:"AirSync.Sync"`
		SyncKey string      `wbxml:"AirSync.SyncKey"`
		AppData *rawAppData `wbxml:"AirSync.ApplicationData,omitempty"`
	}
	if err := Unmarshal(data, &rebuilt); err != nil {
		t.Fatalf("Unmarshal rebuilt: %v", err)
	}
	if rebuilt.AppData == nil || rebuilt.AppData.Subject != "hello" {
		t.Fatalf("typed re-decode mismatch: %+v", rebuilt.AppData)
	}
}

type rawWithSibling struct {
	XMLName struct{}    `wbxml:"AirSync.Sync"`
	SyncKey string      `wbxml:"AirSync.SyncKey"`
	AppData *RawElement `wbxml:"AirSync.ApplicationData,raw"`
	Status  int32       `wbxml:"AirSync.Status"`
}

// SPEC: MS-ASWBXML/marshal.raw-page
func TestMarshal_RawElement_PreservesInnerSwitchPage(t *testing.T) {
	body := emailBodyBytes(t, "x")
	in := rawWithSibling{
		SyncKey: "1",
		AppData: &RawElement{Page: PageAirSync, Bytes: body},
		Status:  1,
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	statusTag := EncodeTag(0x0E, false, true) // AirSync.Status with content bit
	if !bytes.Contains(data, []byte{SwitchPage, PageAirSync, statusTag}) {
		t.Fatalf("expected SWITCH_PAGE AirSync before Status open tag in %x", data)
	}

	var out rawWithSibling
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Status != 1 {
		t.Fatalf("Status not preserved: %d", out.Status)
	}
	if !bytes.Equal(out.AppData.Bytes, body) {
		t.Fatalf("body mismatch")
	}
}

type rawSimple struct {
	XMLName struct{}    `wbxml:"AirSync.Sync"`
	AppData *RawElement `wbxml:"AirSync.ApplicationData,raw"`
}

type rawByValue struct {
	XMLName struct{}   `wbxml:"AirSync.Sync"`
	AppData RawElement `wbxml:"AirSync.ApplicationData,raw"`
}

// SPEC: MS-ASWBXML/marshal.raw
func TestMarshal_RawElement_OmitEmpty(t *testing.T) {
	in := rawOuter{SyncKey: "1"}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(data, []byte{0x5D}) { // ApplicationData with content bit
		t.Fatalf("ApplicationData unexpectedly present in %x", data)
	}

	in2 := rawOuter{SyncKey: "1", AppData: &RawElement{Page: PageAirSync}}
	data2, err := Marshal(&in2)
	if err != nil {
		t.Fatalf("Marshal omitempty empty: %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("non-nil empty RawElement should be omitted with omitempty: got %x want %x", data2, data)
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestMarshal_RawElement_NilWithoutOmitEmpty(t *testing.T) {
	data, err := Marshal(&rawSimple{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	apd := byte(0x1D) // ApplicationData identity, no content bit
	if !bytes.Contains(data, []byte{apd}) {
		t.Fatalf("expected empty ApplicationData tag, got %x", data)
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestMarshal_RawElement_ByValue(t *testing.T) {
	body := emailBodyBytes(t, "v")
	in := rawByValue{AppData: RawElement{Page: PageAirSync, Bytes: body}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out rawByValue
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(out.AppData.Bytes, body) {
		t.Fatalf("body mismatch")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
// TestMarshal_RawElement_ByValue_NonAddressable verifies that a RawElement
// value field marshals without panicking when the parent struct itself is
// passed by value (i.e. unaddressable inside reflect).
func TestMarshal_RawElement_ByValue_NonAddressable(t *testing.T) {
	body := emailBodyBytes(t, "v")
	in := rawByValue{AppData: RawElement{Page: PageAirSync, Bytes: body}}
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal(value): %v", err)
	}
	var out rawByValue
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(out.AppData.Bytes, body) {
		t.Fatalf("body mismatch")
	}
}

type rawWrongPtrType struct {
	XMLName struct{}    `wbxml:"AirSync.Sync"`
	AppData *rawByValue `wbxml:"AirSync.ApplicationData,raw"`
}

type rawWrongValueType struct {
	XMLName struct{}   `wbxml:"AirSync.Sync"`
	AppData rawByValue `wbxml:"AirSync.ApplicationData,raw"`
}

type rawWrongScalar struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	AppData int      `wbxml:"AirSync.ApplicationData,raw"`
}

// SPEC: MS-ASWBXML/marshal.raw
func TestMarshal_RawElement_WrongTypes(t *testing.T) {
	if _, err := Marshal(&rawWrongPtrType{AppData: &rawByValue{}}); err == nil {
		t.Fatal("expected error for *non-RawElement field tagged raw")
	}
	if _, err := Marshal(&rawWrongValueType{}); err == nil {
		t.Fatal("expected error for struct-non-RawElement field tagged raw")
	}
	if _, err := Marshal(&rawWrongScalar{AppData: 1}); err == nil {
		t.Fatal("expected error for scalar field tagged raw")
	}
}

// SPEC: MS-ASWBXML/marshal.raw-page
func TestMarshal_RawElement_NonZeroPageEmitsForceSwitch(t *testing.T) {
	body := []byte{EncodeTag(0x14, false, true), StrI, 'h', 'i', 0x00, End} // Email.Subject = hi
	in := rawOuter{
		SyncKey: "1",
		AppData: &RawElement{Page: PageEmail, Bytes: body},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(data, []byte{SwitchPage, PageEmail}) {
		t.Fatalf("expected SWITCH_PAGE Email in %x", data)
	}
	var out rawOuter
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.AppData == nil || out.AppData.Page != PageAirSync {
		t.Fatalf("Page mismatch: %+v", out.AppData)
	}
	expectedBody := append([]byte{SwitchPage, PageEmail}, body...)
	if !bytes.Equal(out.AppData.Bytes, expectedBody) {
		t.Fatalf("body mismatch:\n got %x\nwant %x", out.AppData.Bytes, expectedBody)
	}
}

// SPEC: OMA-WBXML-1.3/global.SWITCH_PAGE
func TestEncoder_ForceSwitchPage_UnknownPage(t *testing.T) {
	e := NewEncoder(&bytes.Buffer{})
	if err := e.ForceSwitchPage(0xFF); err == nil {
		t.Fatal("expected unknown-page error")
	}
}

type errWriter struct{ err error }

func (w *errWriter) Write(p []byte) (int, error) { return 0, w.err }

// SPEC: OMA-WBXML-1.3/global.SWITCH_PAGE
func TestEncoder_ForceSwitchPage_WriteError(t *testing.T) {
	e := NewEncoder(&errWriter{err: io.ErrShortWrite})
	if err := e.ForceSwitchPage(PageEmail); err == nil {
		t.Fatal("expected write error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestEncoder_WriteRaw_WriteError(t *testing.T) {
	e := NewEncoder(&errWriter{err: io.ErrShortWrite})
	if err := e.WriteRaw([]byte{0x01}); err == nil {
		t.Fatal("expected write error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_Page(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{SwitchPage, PageEmail, 0x40}))
	if _, err := d.NextToken(); err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	if d.Page() != PageEmail {
		t.Fatalf("Page=%d want %d", d.Page(), PageEmail)
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_NoContent(t *testing.T) {
	d := NewDecoder(bytes.NewReader(nil))
	body, err := d.CaptureRaw(false)
	if err != nil || body != nil {
		t.Fatalf("CaptureRaw(false)=%x,%v; want nil,nil", body, err)
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_TruncatedTag(t *testing.T) {
	d := NewDecoder(bytes.NewReader(nil))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected EOF error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_TruncatedSwitchPage(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{SwitchPage}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected SWITCH_PAGE error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_UnknownPage(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{SwitchPage, 0xFF}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected unknown page error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_TruncatedStrI(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{StrI, 'a'}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected STR_I truncation error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_RawElementExceedsLimit(t *testing.T) {
	prev := MaxRawElementSize
	MaxRawElementSize = 4
	defer func() { MaxRawElementSize = prev }()
	// A flood of small tag bytes (each one statement-sized in buf) under the
	// individual STR_I/OPAQUE caps but past the overall budget.
	stream := []byte{0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, End}
	d := NewDecoder(bytes.NewReader(stream))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected raw-element limit error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_StrIExceedsLimit(t *testing.T) {
	prev := MaxInlineStringSize
	MaxInlineStringSize = 4
	defer func() { MaxInlineStringSize = prev }()
	stream := append([]byte{StrI}, []byte("aaaaaaaa")...)
	stream = append(stream, 0x00, End)
	d := NewDecoder(bytes.NewReader(stream))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected STR_I limit error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
// TestDecoder_CaptureRaw_StrIBoundary verifies the STR_I cap is exact:
// MaxInlineStringSize bytes are accepted, MaxInlineStringSize+1 bytes are
// rejected before the extra byte is appended (no off-by-one).
func TestDecoder_CaptureRaw_StrIBoundary(t *testing.T) {
	prev := MaxInlineStringSize
	MaxInlineStringSize = 4
	defer func() { MaxInlineStringSize = prev }()

	atLimit := append([]byte{StrI}, []byte("aaaa")...)
	atLimit = append(atLimit, 0x00, End)
	d := NewDecoder(bytes.NewReader(atLimit))
	if _, err := d.CaptureRaw(true); err != nil {
		t.Fatalf("at-limit STR_I should pass, got %v", err)
	}

	overLimit := append([]byte{StrI}, []byte("aaaaa")...)
	overLimit = append(overLimit, 0x00, End)
	d2 := NewDecoder(bytes.NewReader(overLimit))
	if _, err := d2.CaptureRaw(true); err == nil {
		t.Fatal("over-limit STR_I should fail")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_StrTAndOpaque(t *testing.T) {
	// StrT 0x83 + offset 5 (mb_u_int32 single byte), then END for outer.
	d := NewDecoder(bytes.NewReader([]byte{StrT, 0x05, End}))
	body, err := d.CaptureRaw(true)
	if err != nil {
		t.Fatalf("CaptureRaw: %v", err)
	}
	if !bytes.Equal(body, []byte{StrT, 0x05}) {
		t.Fatalf("body=%x want StrT 05", body)
	}
	d2 := NewDecoder(bytes.NewReader([]byte{Opaque, 0x02, 0xAA, 0xBB, End}))
	body2, err := d2.CaptureRaw(true)
	if err != nil {
		t.Fatalf("CaptureRaw opaque: %v", err)
	}
	if !bytes.Equal(body2, []byte{Opaque, 0x02, 0xAA, 0xBB}) {
		t.Fatalf("body=%x", body2)
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_TruncatedOpaqueLength(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{Opaque, 0x80}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected OPAQUE length error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_OpaqueExceedsLimit(t *testing.T) {
	defer swapMaxOpaqueSize(2)()
	d := NewDecoder(bytes.NewReader([]byte{Opaque, 0x05, 1, 2, 3, 4, 5, End}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected OPAQUE limit error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_TruncatedOpaquePayload(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{Opaque, 0x05, 1, 2}))
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected truncation error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecoder_CaptureRaw_BadTagBeforeSwitchPage(t *testing.T) {
	d := NewDecoder(bytes.NewReader([]byte{0x40}))
	d.pageInit = false
	if _, err := d.CaptureRaw(true); err == nil {
		t.Fatal("expected error for tag before SWITCH_PAGE")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestDecodeRawField_TruncatedBody(t *testing.T) {
	// Build a Marshal-like wire: header, Sync open with content, ApplicationData
	// open with content, then EOF (truncation inside CaptureRaw).
	var w bytes.Buffer
	enc := NewEncoder(&w)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		t.Fatal(err)
	}
	if err := enc.StartTag(PageAirSync, 0x05, false, true); err != nil { // AirSync.Sync
		t.Fatal(err)
	}
	if err := enc.StartTag(PageAirSync, 0x1D, false, true); err != nil { // ApplicationData
		t.Fatal(err)
	}
	if err := Unmarshal(w.Bytes(), &rawSimple{}); err == nil {
		t.Fatal("expected truncation error")
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestAppendMbUint32Bytes_Overlong(t *testing.T) {
	r := bytes.NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80})
	if _, err := appendMbUint32Bytes(r, nil); err == nil {
		t.Fatal("expected overlong error")
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestAppendMbUint32Bytes_EOFMidStream(t *testing.T) {
	r := bytes.NewReader([]byte{0x80})
	if _, err := appendMbUint32Bytes(r, nil); err == nil {
		t.Fatal("expected EOF error")
	}
}

// SPEC: OMA-WBXML-1.3/mb_u_int32.encoding
func TestAppendMbUint32Bytes_FirstByteEOF(t *testing.T) {
	r := bytes.NewReader(nil)
	if _, err := appendMbUint32Bytes(r, nil); err == nil {
		t.Fatal("expected EOF error")
	}
}

// SPEC: MS-ASWBXML/marshal.raw
func TestParseTag_RawOption(t *testing.T) {
	spec, err := parseTag("AirSync.ApplicationData,raw")
	if err != nil {
		t.Fatalf("parseTag(raw): %v", err)
	}
	if !spec.raw {
		t.Fatal("raw option not set")
	}
	if _, err := parseTag("AirSync.ApplicationData,raw,opaque"); err == nil {
		t.Fatal("expected error for raw+opaque combination")
	}
}
