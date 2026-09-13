package wbxml

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// MaxOpaqueSize bounds the in-memory size of a single OPAQUE payload that the
// decoder is willing to allocate. EAS payloads in practice are well under one
// megabyte; the cap guards against pathological inputs (notably from fuzzing)
// that would otherwise request multi-gigabyte allocations. It is exposed as a
// var rather than a const so callers (and tests) may tune it.
var MaxOpaqueSize uint32 = 64 << 20 // 64 MiB

// MaxInlineStringSize bounds the in-memory size of a single inline (STR_I)
// string. The same rationale as MaxOpaqueSize applies.
var MaxInlineStringSize = 16 << 20 // 16 MiB

// MaxRawElementSize bounds the cumulative size of bytes captured by a single
// Decoder.CaptureRaw call. Per-token caps (MaxOpaqueSize, MaxInlineStringSize)
// already neutralise individual giant payloads; this cap additionally limits
// the total buffer growth from a flood of small tokens or tags within one
// element body. Exposed as a var so callers (and tests) may tune it.
var MaxRawElementSize = 64 << 20 // 64 MiB

// TokenKind classifies a logical WBXML token after SWITCH_PAGE handling.
type TokenKind int

const (
	KindTag TokenKind = iota + 1
	KindEnd
	KindString
	KindOpaque
)

func (k TokenKind) String() string {
	switch k {
	case KindTag:
		return "Tag"
	case KindEnd:
		return "End"
	case KindString:
		return "String"
	case KindOpaque:
		return "Opaque"
	default:
		return fmt.Sprintf("TokenKind(%d)", int(k))
	}
}

// Token is a decoded WBXML logical event.
type Token struct {
	Kind       TokenKind
	Page       byte   // valid for KindTag
	Tag        byte   // 6-bit identity, valid for KindTag
	HasAttrs   bool   // valid for KindTag
	HasContent bool   // valid for KindTag
	String     string // valid for KindString
	Bytes      []byte // valid for KindOpaque
}

// Decoder is a streaming WBXML 1.3 decoder. It transparently consumes
// SWITCH_PAGE tokens and reflects the active page on subsequent tag tokens.
type Decoder struct {
	r           *bufio.Reader
	page        byte
	pageInit    bool
	stringTable []byte
}

// NewDecoder returns a new decoder reading from r. The active code page is
// initialised to 0 per OMA-WBXML 1.3 §5.5.
func NewDecoder(r io.Reader) *Decoder {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Decoder{r: br, page: 0, pageInit: true}
}

// ReadHeader parses the document header.
func (d *Decoder) ReadHeader() (Header, error) {
	var h Header
	if err := h.Read(d.r); err != nil {
		return h, err
	}
	d.stringTable = h.StringTable
	return h, nil
}

// NextToken reads and returns the next logical token. SWITCH_PAGE is consumed
// internally and is not surfaced as a token.
func (d *Decoder) NextToken() (Token, error) {
	for {
		b, err := d.r.ReadByte()
		if err != nil {
			return Token{}, err
		}
		switch b {
		case SwitchPage:
			page, err := d.r.ReadByte()
			if err != nil {
				return Token{}, fmt.Errorf("wbxml: SWITCH_PAGE: %w", err)
			}
			if _, ok := PageByID(page); !ok {
				return Token{}, fmt.Errorf("wbxml: SWITCH_PAGE to unknown code page %d", page)
			}
			d.page = page
			d.pageInit = true
		case End:
			return Token{Kind: KindEnd}, nil
		case StrI:
			s, err := readNulString(d.r)
			if err != nil {
				return Token{}, err
			}
			return Token{Kind: KindString, String: s}, nil
		case StrT:
			off, _, err := ReadMbUint32(d.r)
			if err != nil {
				return Token{}, err
			}
			s, err := stringFromTable(d.stringTable, off)
			if err != nil {
				return Token{}, err
			}
			return Token{Kind: KindString, String: s}, nil
		case Opaque:
			n, _, err := ReadMbUint32(d.r)
			if err != nil {
				return Token{}, err
			}
			if n > MaxOpaqueSize {
				return Token{}, fmt.Errorf("wbxml: OPAQUE length %d exceeds %d-byte limit", n, MaxOpaqueSize)
			}
			payload := make([]byte, n)
			if _, err := io.ReadFull(d.r, payload); err != nil {
				return Token{}, err
			}
			return Token{Kind: KindOpaque, Bytes: payload}, nil
		default:
			// All remaining bytes are tag bytes; their identity must fall in
			// the 6-bit range and the active page must already be known.
			if !d.pageInit {
				return Token{}, errors.New("wbxml: tag byte before any SWITCH_PAGE")
			}
			return Token{
				Kind:       KindTag,
				Page:       d.page,
				Tag:        TagIdentity(b),
				HasAttrs:   TagHasAttributes(b),
				HasContent: TagHasContent(b),
			}, nil
		}
	}
}

// Page returns the currently active code page.
func (d *Decoder) Page() byte { return d.page }

// CaptureRaw reads bytes from the underlying stream until it consumes the
// END token that closes the element whose open tag was just returned. The
// returned slice contains everything between (but excluding) that open tag
// and the matching END token, including any inner SWITCH_PAGE markers, so it
// can be replayed verbatim by Encoder.WriteRaw. If hasContent is false the
// call is a no-op and returns a nil slice.
//
// CaptureRaw also tracks the active code page across inner SWITCH_PAGE
// transitions, so Decoder.Page reflects the page that is active immediately
// after the closing END.
//
// SPEC: MS-ASWBXML/marshal.raw
func (d *Decoder) CaptureRaw(hasContent bool) ([]byte, error) {
	if !hasContent {
		return nil, nil
	}
	var buf []byte
	checkBudget := func() error {
		if len(buf) > MaxRawElementSize {
			return fmt.Errorf("wbxml: raw element exceeds %d-byte limit", MaxRawElementSize)
		}
		return nil
	}
	depth := 1
	for depth > 0 {
		b, err := d.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		switch b {
		case SwitchPage:
			page, err := d.r.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("wbxml: SWITCH_PAGE: %w", err)
			}
			if _, ok := PageByID(page); !ok {
				return nil, fmt.Errorf("wbxml: SWITCH_PAGE to unknown code page %d", page)
			}
			d.page = page
			d.pageInit = true
			buf = append(buf, SwitchPage, page)
		case End:
			depth--
			if depth == 0 {
				return buf, nil
			}
			buf = append(buf, End)
		case StrI:
			buf = append(buf, StrI)
			strLen := 0
			for {
				c, err := d.r.ReadByte()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return nil, io.ErrUnexpectedEOF
					}
					return nil, err
				}
				if c == 0x00 {
					buf = append(buf, c)
					break
				}
				if strLen >= MaxInlineStringSize {
					return nil, fmt.Errorf("wbxml: STR_I exceeds %d-byte limit", MaxInlineStringSize)
				}
				buf = append(buf, c)
				strLen++
			}
		case StrT:
			buf = append(buf, StrT)
			if buf, err = appendMbUint32Bytes(d.r, buf); err != nil {
				return nil, err
			}
		case Opaque:
			buf = append(buf, Opaque)
			before := len(buf)
			if buf, err = appendMbUint32Bytes(d.r, buf); err != nil {
				return nil, err
			}
			n, _, perr := ReadMbUint32(bytes.NewReader(buf[before:]))
			if perr != nil {
				return nil, perr
			}
			if n > MaxOpaqueSize {
				return nil, fmt.Errorf("wbxml: OPAQUE length %d exceeds %d-byte limit", n, MaxOpaqueSize)
			}
			payload := make([]byte, n)
			if _, err := io.ReadFull(d.r, payload); err != nil {
				return nil, err
			}
			buf = append(buf, payload...)
		default:
			if !d.pageInit {
				return nil, errors.New("wbxml: tag byte before any SWITCH_PAGE")
			}
			buf = append(buf, b)
			if TagHasContent(b) {
				depth++
			}
		}
		if err := checkBudget(); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// appendMbUint32Bytes copies an mb_u_int32 from r into dst, returning the
// extended slice. It is the byte-preserving counterpart to ReadMbUint32 and is
// used by CaptureRaw to keep STR_T offsets and OPAQUE lengths verbatim.
func appendMbUint32Bytes(r io.ByteReader, dst []byte) ([]byte, error) {
	for n := 1; n <= 5; n++ {
		c, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && n > 1 {
				return dst, io.ErrUnexpectedEOF
			}
			return dst, err
		}
		dst = append(dst, c)
		if c&0x80 == 0 {
			return dst, nil
		}
	}
	return dst, errors.New("wbxml: mb_u_int32 longer than 5 bytes")
}

func readNulString(r io.ByteReader) (string, error) {
	var buf []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		if b == 0x00 {
			return string(buf), nil
		}
		if len(buf) >= MaxInlineStringSize {
			return "", fmt.Errorf("wbxml: STR_I exceeds %d-byte limit", MaxInlineStringSize)
		}
		buf = append(buf, b)
	}
}

func stringFromTable(tbl []byte, off uint32) (string, error) {
	if int(off) >= len(tbl) {
		return "", fmt.Errorf("wbxml: STR_T offset %d out of range (table=%d)", off, len(tbl))
	}
	end := int(off)
	for end < len(tbl) && tbl[end] != 0x00 {
		end++
	}
	return string(tbl[off:end]), nil
}
