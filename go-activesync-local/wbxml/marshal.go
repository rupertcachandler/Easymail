package wbxml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// Marshal serialises v as a complete WBXML 1.3 document with a default EAS
// header (version 0x03, publicid 0x01, UTF-8 charset, empty string-table).
//
// v must be a pointer to a struct whose XMLName field carries the root
// element's wbxml tag. Field tags use the form `wbxml:"Page.Tag"` with
// optional comma-separated options omitempty and opaque.
func Marshal(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("wbxml: Marshal expects pointer to struct, got %T", v)
	}
	info, err := infoFor(rv.Type())
	if err != nil {
		return nil, err
	}
	if info.self == nil {
		return nil, fmt.Errorf("wbxml: Marshal: %s has no XMLName tag", rv.Type().Name())
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHeader(Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}); err != nil {
		return nil, err
	}
	if err := encodeStruct(enc, rv, info, info.self); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal reads a WBXML document from data into v, which must be a pointer
// to a struct of the same shape as the document.
func Unmarshal(data []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("wbxml: Unmarshal expects pointer to struct, got %T", v)
	}
	info, err := infoFor(rv.Elem().Type())
	if err != nil {
		return err
	}
	if info.self == nil {
		return fmt.Errorf("wbxml: Unmarshal: %s has no XMLName tag", rv.Elem().Type().Name())
	}

	dec := NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		return err
	}
	tok, err := dec.NextToken()
	if err != nil {
		return err
	}
	if tok.Kind != KindTag {
		return fmt.Errorf("wbxml: Unmarshal: expected root tag, got %s", tok.Kind)
	}
	if tok.Page != info.self.page || tok.Tag != info.self.identity {
		return fmt.Errorf("wbxml: root tag mismatch: got page=%d id=0x%02X want page=%d id=0x%02X",
			tok.Page, tok.Tag, info.self.page, info.self.identity)
	}
	return decodeStruct(dec, rv.Elem(), info, tok.HasContent)
}

// ------- internal: type info cache -------

type tagSpec struct {
	page      byte
	identity  byte
	pageName  string
	tagName   string
	omitempty bool
	opaque    bool
	raw       bool
}

type fieldSpec struct {
	tagSpec
	index []int
	field reflect.StructField
}

type structInfo struct {
	self   *tagSpec       // element identity (from XMLName), or nil
	fields []fieldSpec    // non-XMLName fields
	byTag  map[uint32]int // page<<8|identity → index into fields
}

var infoCache sync.Map // map[reflect.Type]*structInfo

func infoFor(t reflect.Type) (*structInfo, error) {
	if cached, ok := infoCache.Load(t); ok {
		return cached.(*structInfo), nil
	}
	si := &structInfo{byTag: map[uint32]int{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		raw := f.Tag.Get("wbxml")
		if raw == "" {
			continue
		}
		spec, err := parseTag(raw)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", t.Name(), f.Name, err)
		}
		if f.Name == "XMLName" {
			s := spec
			si.self = &s
			continue
		}
		fs := fieldSpec{tagSpec: spec, index: f.Index, field: f}
		si.fields = append(si.fields, fs)
		key := uint32(spec.page)<<8 | uint32(spec.identity)
		si.byTag[key] = len(si.fields) - 1
	}
	infoCache.Store(t, si)
	return si, nil
}

func parseTag(raw string) (tagSpec, error) {
	parts := strings.Split(raw, ",")
	head := parts[0]
	dot := strings.IndexByte(head, '.')
	if dot < 0 {
		return tagSpec{}, fmt.Errorf("wbxml tag %q must be Page.Tag", raw)
	}
	pageName := head[:dot]
	tagName := head[dot+1:]
	page, ok := PageByName(pageName)
	if !ok {
		return tagSpec{}, fmt.Errorf("unknown page %q", pageName)
	}
	tok, ok := TokenByTag(page.ID, tagName)
	if !ok {
		return tagSpec{}, fmt.Errorf("unknown tag %s.%s", pageName, tagName)
	}
	out := tagSpec{
		page:     page.ID,
		identity: tok,
		pageName: pageName,
		tagName:  tagName,
	}
	for _, opt := range parts[1:] {
		switch strings.TrimSpace(opt) {
		case "omitempty":
			out.omitempty = true
		case "opaque":
			out.opaque = true
		case "raw":
			out.raw = true
		case "":
		default:
			return tagSpec{}, fmt.Errorf("unknown wbxml option %q", opt)
		}
	}
	if out.raw && out.opaque {
		return tagSpec{}, fmt.Errorf("wbxml options ,raw and ,opaque are mutually exclusive in %q", raw)
	}
	return out, nil
}

var rawElementType = reflect.TypeOf(RawElement{})

// ------- encoding -------

func encodeStruct(enc *Encoder, v reflect.Value, info *structInfo, self *tagSpec) error {
	hasContent := structHasContent(v, info)
	if err := enc.StartTag(self.page, self.identity, false, hasContent); err != nil {
		return err
	}
	if hasContent {
		for i := range info.fields {
			fs := &info.fields[i]
			fv := v.FieldByIndex(fs.index)
			if fs.omitempty && isZero(fv) {
				continue
			}
			if err := encodeField(enc, fv, fs); err != nil {
				return err
			}
		}
		if err := enc.EndTag(); err != nil {
			return err
		}
	}
	return nil
}

func encodeField(enc *Encoder, v reflect.Value, fs *fieldSpec) error {
	if fs.raw {
		return encodeRawField(enc, v, fs)
	}
	// Slices of struct or scalar produce one element per entry.
	if v.Kind() == reflect.Slice && v.Type().Elem().Kind() != reflect.Uint8 {
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			sub := elem
			if sub.Kind() == reflect.Pointer {
				if sub.IsNil() {
					continue
				}
				sub = sub.Elem()
			}
			switch sub.Kind() {
			case reflect.Struct:
				subInfo, err := infoFor(sub.Type())
				if err != nil {
					return err
				}
				if err := encodeStruct(enc, sub, subInfo, &fs.tagSpec); err != nil {
					return err
				}
			case reflect.String, reflect.Bool,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if err := encodeScalarElement(enc, sub, &fs.tagSpec); err != nil {
					return err
				}
			default:
				return fmt.Errorf("wbxml: slice of unsupported kind %s for %s.%s", sub.Kind(), fs.pageName, fs.tagName)
			}
		}
		return nil
	}
	// Pointer to struct.
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			if fs.omitempty {
				return nil
			}
			return enc.StartTag(fs.page, fs.identity, false, false)
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		subInfo, err := infoFor(v.Type())
		if err != nil {
			return err
		}
		return encodeStruct(enc, v, subInfo, &fs.tagSpec)
	case reflect.Slice:
		// []byte path (opaque or inline).
		if fs.opaque {
			if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
				return err
			}
			if err := enc.Opaque(v.Bytes()); err != nil {
				return err
			}
			return enc.EndTag()
		}
		// Without opaque, []byte is treated as a string.
		if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
			return err
		}
		if err := enc.StrI(string(v.Bytes())); err != nil {
			return err
		}
		return enc.EndTag()
	case reflect.String:
		if v.String() == "" && fs.omitempty {
			return nil
		}
		if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
			return err
		}
		if err := enc.StrI(v.String()); err != nil {
			return err
		}
		return enc.EndTag()
	case reflect.Bool:
		if !v.Bool() && fs.omitempty {
			return nil
		}
		if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
			return err
		}
		s := "0"
		if v.Bool() {
			s = "1"
		}
		if err := enc.StrI(s); err != nil {
			return err
		}
		return enc.EndTag()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() == 0 && fs.omitempty {
			return nil
		}
		if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
			return err
		}
		if err := enc.StrI(strconv.FormatInt(v.Int(), 10)); err != nil {
			return err
		}
		return enc.EndTag()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() == 0 && fs.omitempty {
			return nil
		}
		if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
			return err
		}
		if err := enc.StrI(strconv.FormatUint(v.Uint(), 10)); err != nil {
			return err
		}
		return enc.EndTag()
	default:
		return fmt.Errorf("wbxml: unsupported field kind %s for %s.%s", v.Kind(), fs.pageName, fs.tagName)
	}
}

func encodeScalarElement(enc *Encoder, v reflect.Value, ts *tagSpec) error {
	if err := enc.StartTag(ts.page, ts.identity, false, true); err != nil {
		return err
	}
	var s string
	switch v.Kind() {
	case reflect.String:
		s = v.String()
	case reflect.Bool:
		if v.Bool() {
			s = "1"
		} else {
			s = "0"
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s = strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s = strconv.FormatUint(v.Uint(), 10)
	default:
		return fmt.Errorf("wbxml: unsupported scalar kind %s for %s.%s", v.Kind(), ts.pageName, ts.tagName)
	}
	if err := enc.StrI(s); err != nil {
		return err
	}
	return enc.EndTag()
}

func structHasContent(v reflect.Value, info *structInfo) bool {
	for i := range info.fields {
		fv := v.FieldByIndex(info.fields[i].index)
		if !info.fields[i].omitempty || !isZero(fv) {
			return true
		}
	}
	return false
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		return v.IsNil() || (v.Kind() == reflect.Slice && v.Len() == 0) || (v.Kind() == reflect.Map && v.Len() == 0)
	}
	return v.IsZero()
}

// ------- decoding -------

func decodeStruct(dec *Decoder, v reflect.Value, info *structInfo, hasContent bool) error {
	if !hasContent {
		return nil
	}
	for {
		tok, err := dec.NextToken()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		switch tok.Kind {
		case KindEnd:
			return nil
		case KindTag:
			key := uint32(tok.Page)<<8 | uint32(tok.Tag)
			idx, ok := info.byTag[key]
			if !ok {
				if err := skipElement(dec, tok.HasContent); err != nil {
					return err
				}
				continue
			}
			fs := &info.fields[idx]
			fv := v.FieldByIndex(fs.index)
			if err := decodeField(dec, fv, fs, tok); err != nil {
				return err
			}
		default:
			return fmt.Errorf("wbxml: unexpected token %s inside struct", tok.Kind)
		}
	}
}

func decodeField(dec *Decoder, fv reflect.Value, fs *fieldSpec, openTok Token) error {
	if fs.raw {
		return decodeRawField(dec, fv, fs, openTok)
	}
	if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() != reflect.Uint8 {
		elemType := fv.Type().Elem()
		isPtr := elemType.Kind() == reflect.Pointer
		if isPtr {
			elemType = elemType.Elem()
		}
		switch elemType.Kind() {
		case reflect.Struct:
			newPtr := reflect.New(elemType)
			subInfo, err := infoFor(elemType)
			if err != nil {
				return err
			}
			if err := decodeStruct(dec, newPtr.Elem(), subInfo, openTok.HasContent); err != nil {
				return err
			}
			if isPtr {
				fv.Set(reflect.Append(fv, newPtr))
			} else {
				fv.Set(reflect.Append(fv, newPtr.Elem()))
			}
			return nil
		case reflect.String, reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			newPtr := reflect.New(elemType)
			if openTok.HasContent {
				s, opaque, err := readElementValue(dec)
				if err != nil {
					return err
				}
				if opaque != nil {
					s = string(opaque)
				}
				if err := assignScalar(newPtr.Elem(), s); err != nil {
					return err
				}
			}
			if isPtr {
				fv.Set(reflect.Append(fv, newPtr))
			} else {
				fv.Set(reflect.Append(fv, newPtr.Elem()))
			}
			return nil
		default:
			return fmt.Errorf("wbxml: unsupported slice elem kind %s for %s.%s", elemType.Kind(), fs.pageName, fs.tagName)
		}
	}
	if fv.Kind() == reflect.Pointer {
		elemType := fv.Type().Elem()
		newPtr := reflect.New(elemType)
		fv.Set(newPtr)
		fv = newPtr.Elem()
	}
	switch fv.Kind() {
	case reflect.Struct:
		subInfo, err := infoFor(fv.Type())
		if err != nil {
			return err
		}
		return decodeStruct(dec, fv, subInfo, openTok.HasContent)
	case reflect.Slice:
		if !openTok.HasContent {
			fv.SetBytes(nil)
			return nil
		}
		s, opaque, err := readElementValue(dec)
		if err != nil {
			return err
		}
		if opaque != nil {
			fv.SetBytes(opaque)
		} else {
			fv.SetBytes([]byte(s))
		}
		return nil
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !openTok.HasContent {
			fv.SetZero()
			return nil
		}
		s, opaque, err := readElementValue(dec)
		if err != nil {
			return err
		}
		if opaque != nil {
			s = string(opaque)
		}
		return assignScalar(fv, s)
	default:
		return fmt.Errorf("wbxml: unsupported field kind %s for %s.%s", fv.Kind(), fs.pageName, fs.tagName)
	}
}

func readElementValue(dec *Decoder) (string, []byte, error) {
	var sb strings.Builder
	var op []byte
	for {
		tok, err := dec.NextToken()
		if err != nil {
			return "", nil, err
		}
		switch tok.Kind {
		case KindEnd:
			if op != nil {
				return "", op, nil
			}
			return sb.String(), nil, nil
		case KindString:
			sb.WriteString(tok.String)
		case KindOpaque:
			op = append(op, tok.Bytes...)
		case KindTag:
			return "", nil, fmt.Errorf("wbxml: unexpected nested tag inside scalar element")
		}
	}
}

func assignScalar(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		switch s {
		case "1", "true", "True", "TRUE":
			v.SetBool(true)
		case "0", "false", "False", "FALSE", "":
			v.SetBool(false)
		default:
			return fmt.Errorf("wbxml: cannot parse %q as bool", s)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(n)
	default:
		return fmt.Errorf("wbxml: cannot assign string to %s", v.Kind())
	}
	return nil
}

// encodeRawField emits a *RawElement (or RawElement) field. The element body
// is re-aligned to RawElement.Page via Encoder.ForceSwitchPage so that the
// raw bytes can be written verbatim regardless of which page was active
// before. After the body the encoder's page is marked as unknown by
// Encoder.WriteRaw, so the next sibling is preceded by a SWITCH_PAGE.
func encodeRawField(enc *Encoder, v reflect.Value, fs *fieldSpec) error {
	var raw *RawElement
	switch v.Kind() {
	case reflect.Pointer:
		if v.Type().Elem() != rawElementType {
			return fmt.Errorf("wbxml: ,raw field %s.%s must be *RawElement, got %s", fs.pageName, fs.tagName, v.Type())
		}
		if v.IsNil() {
			if fs.omitempty {
				return nil
			}
			return enc.StartTag(fs.page, fs.identity, false, false)
		}
		raw = v.Interface().(*RawElement)
	case reflect.Struct:
		if v.Type() != rawElementType {
			return fmt.Errorf("wbxml: ,raw field %s.%s must be RawElement, got %s", fs.pageName, fs.tagName, v.Type())
		}
		// v may be unaddressable (e.g. when the caller passed a value, not a
		// pointer, to Marshal); copy the RawElement out by value so we never
		// rely on v.Addr() succeeding.
		rawValue := v.Interface().(RawElement)
		raw = &rawValue
	default:
		return fmt.Errorf("wbxml: ,raw field %s.%s must be RawElement or *RawElement, got %s", fs.pageName, fs.tagName, v.Type())
	}
	if len(raw.Bytes) == 0 {
		if fs.omitempty {
			return nil
		}
		return enc.StartTag(fs.page, fs.identity, false, false)
	}
	if err := enc.StartTag(fs.page, fs.identity, false, true); err != nil {
		return err
	}
	// Align the encoder's active page with raw.Page only when needed: emitting
	// an unconditional SWITCH_PAGE would prevent byte-identity round trips for
	// RawElement values where the active page was already correct.
	if !enc.pageInit || enc.page != raw.Page {
		if err := enc.ForceSwitchPage(raw.Page); err != nil {
			return err
		}
	}
	if err := enc.WriteRaw(raw.Bytes); err != nil {
		return err
	}
	return enc.EndTag()
}

// decodeRawField captures the verbatim wire content of an open element into
// a *RawElement (or RawElement) field. The active code page at the moment of
// the open tag is recorded in RawElement.Page.
func decodeRawField(dec *Decoder, fv reflect.Value, fs *fieldSpec, openTok Token) error {
	page := openTok.Page
	body, err := dec.CaptureRaw(openTok.HasContent)
	if err != nil {
		return err
	}
	switch fv.Kind() {
	case reflect.Pointer:
		if fv.Type().Elem() != rawElementType {
			return fmt.Errorf("wbxml: ,raw field %s.%s must be *RawElement, got %s", fs.pageName, fs.tagName, fv.Type())
		}
		fv.Set(reflect.ValueOf(&RawElement{Page: page, Bytes: body}))
		return nil
	case reflect.Struct:
		if fv.Type() != rawElementType {
			return fmt.Errorf("wbxml: ,raw field %s.%s must be RawElement, got %s", fs.pageName, fs.tagName, fv.Type())
		}
		fv.Set(reflect.ValueOf(RawElement{Page: page, Bytes: body}))
		return nil
	default:
		return fmt.Errorf("wbxml: ,raw field %s.%s must be RawElement or *RawElement, got %s", fs.pageName, fs.tagName, fv.Type())
	}
}

func skipElement(dec *Decoder, hasContent bool) error {
	if !hasContent {
		return nil
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.NextToken()
		if err != nil {
			return err
		}
		switch tok.Kind {
		case KindTag:
			if tok.HasContent {
				depth++
			}
		case KindEnd:
			depth--
		}
	}
	return nil
}
