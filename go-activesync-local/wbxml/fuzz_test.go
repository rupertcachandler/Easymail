package wbxml

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// SPEC: MS-ASWBXML/decoder.fuzz
func FuzzDecode(f *testing.F) {
	// Seed corpus: a small but representative set of well-formed and edge
	// inputs. The fuzzer mutates these byte sequences and must never cause
	// the decoder to panic.
	seeds := [][]byte{
		// Empty input.
		nil,
		// Header only.
		{0x03, 0x01, 0x6A, 0x00},
		// Header + AirSync.Sync open/close.
		{0x03, 0x01, 0x6A, 0x00, 0x45, 0x01},
		// Header + cross-page Sync containing AirSyncBase.Body.
		{0x03, 0x01, 0x6A, 0x00, 0x45, 0x00, 0x11, 0x4A, 0x01, 0x01},
		// STR_I "ok".
		{0x03, 'o', 'k', 0x00},
		// OPAQUE 4 bytes.
		{0xC3, 0x04, 0xDE, 0xAD, 0xBE, 0xEF},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := NewDecoder(bytes.NewReader(data))
		_, _ = dec.ReadHeader()
		for i := 0; i < 1024; i++ {
			_, err := dec.NextToken()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return
				}
				return
			}
		}
	})
}
