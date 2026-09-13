package wbxml

import (
	"bufio"
	"io"
)

// byteReader adapts an io.Reader to the io.ByteReader interface.
func byteReader(r io.Reader) io.ByteReader {
	if br, ok := r.(io.ByteReader); ok {
		return br
	}
	return bufio.NewReader(r)
}
