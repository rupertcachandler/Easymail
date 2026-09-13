package client

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"reflect"
	"testing"
)

// SPEC: MS-ASHTTP/query.base64.example
func TestQueryBase64_CanonicalExample(t *testing.T) {
	q := Query{
		ProtocolVersion: 0x8C, // 14.0
		Cmd:             CmdSync,
		Locale:          0x0409, // en-US
		DeviceID:        "v140Device",
		DeviceType:      "SmartPhone",
	}
	got, err := q.EncodeBase64()
	if err != nil {
		t.Fatalf("EncodeBase64: %v", err)
	}
	want := "jAAJBAp2MTQwRGV2aWNlAApTbWFydFBob25l"
	if got != want {
		t.Fatalf("EncodeBase64 = %q, want %q", got, want)
	}

	// Round-trip parse.
	parsed, err := ParseBase64(got)
	if err != nil {
		t.Fatalf("ParseBase64: %v", err)
	}
	if !reflect.DeepEqual(parsed, q) {
		t.Fatalf("ParseBase64 = %+v, want %+v", parsed, q)
	}
}

// SPEC: MS-ASHTTP/query.base64.layout
func TestQueryBase64_LayoutBytes(t *testing.T) {
	q := Query{
		ProtocolVersion: 0x91, // 14.1
		Cmd:             CmdProvision,
		Locale:          0x0409,
		DeviceID:        "abc",
		DeviceType:      "WP",
	}
	enc, err := q.EncodeBase64()
	if err != nil {
		t.Fatalf("EncodeBase64: %v", err)
	}
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(enc)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	want := []byte{
		0x91,         // ProtocolVersion 14.1
		CmdProvision, // CommandCode 20
		0x09, 0x04,   // Locale 0x0409 little-endian
		0x03, 'a', 'b', 'c', // DeviceID
		0x00,           // PolicyKey length 0
		0x02, 'W', 'P', // DeviceType
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("bytes = % X, want % X", raw, want)
	}
}

// SPEC: MS-ASHTTP/query.base64.params
func TestQueryBase64_Params(t *testing.T) {
	q := Query{
		ProtocolVersion: 0x91,
		Cmd:             CmdSync,
		Locale:          0x0409,
		DeviceID:        "id",
		DeviceType:      "X",
		Params: []QueryParam{
			{Tag: ParamUser, Value: []byte("user@example.com")},
		},
	}
	enc, err := q.EncodeBase64()
	if err != nil {
		t.Fatalf("EncodeBase64: %v", err)
	}
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(enc)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	// last 18 bytes should be 0x08 (User tag), 0x10 (length 16), then 16 bytes payload.
	tail := raw[len(raw)-18:]
	wantTail := append([]byte{0x08, 0x10}, []byte("user@example.com")...)
	if !bytes.Equal(tail, wantTail) {
		t.Fatalf("tail = % X, want % X", tail, wantTail)
	}
}

// SPEC: MS-ASHTTP/query.command-codes
func TestCommandCodeConstants(t *testing.T) {
	cases := []struct {
		name string
		got  byte
		want byte
	}{
		{"Sync", CmdSync, 0},
		{"SendMail", CmdSendMail, 1},
		{"GetAttachment", CmdGetAttachment, 4},
		{"FolderSync", CmdFolderSync, 9},
		{"Ping", CmdPing, 18},
		{"Provision", CmdProvision, 20},
		{"ItemOperations", CmdItemOperations, 19},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// SPEC: MS-ASHTTP/query.plain
func TestQueryPlain(t *testing.T) {
	q := Query{
		ProtocolVersion: 0x91,
		Cmd:             CmdFolderSync,
		Locale:          0x0409,
		DeviceID:        "id1",
		DeviceType:      "PC",
		Params: []QueryParam{
			{Tag: ParamUser, Value: []byte("user@example.com")},
		},
	}
	got := q.EncodePlain()
	parsed, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if parsed.Get("Cmd") != "FolderSync" {
		t.Errorf("Cmd = %q", parsed.Get("Cmd"))
	}
	if parsed.Get("DeviceId") != "id1" {
		t.Errorf("DeviceId = %q", parsed.Get("DeviceId"))
	}
	if parsed.Get("DeviceType") != "PC" {
		t.Errorf("DeviceType = %q", parsed.Get("DeviceType"))
	}
	if parsed.Get("User") != "user@example.com" {
		t.Errorf("User = %q", parsed.Get("User"))
	}
}

// SPEC: MS-ASHTTP/request.path
func TestEndpointPath(t *testing.T) {
	if EndpointPath != "/Microsoft-Server-ActiveSync" {
		t.Fatalf("EndpointPath = %q", EndpointPath)
	}
}
