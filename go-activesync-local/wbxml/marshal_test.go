package wbxml

import (
	"bytes"
	"reflect"
	"testing"
)

type syncRoot struct {
	XMLName  struct{} `wbxml:"AirSync.Sync"`
	SyncKey  string   `wbxml:"AirSync.SyncKey"`
	WindowSz int      `wbxml:"AirSync.WindowSize,omitempty"`
}

type bodyMsg struct {
	XMLName struct{} `wbxml:"AirSync.Sync"`
	Body    *body    `wbxml:"AirSyncBase.Body"`
}

type body struct {
	XMLName struct{} `wbxml:"AirSyncBase.Body"`
	Type    int      `wbxml:"AirSyncBase.Type"`
	Data    []byte   `wbxml:"AirSyncBase.Data,opaque"`
}

type pingRoot struct {
	XMLName           struct{} `wbxml:"Ping.Ping"`
	HeartbeatInterval int      `wbxml:"Ping.HeartbeatInterval"`
	Folders           folders  `wbxml:"Ping.Folders"`
}

type folders struct {
	XMLName struct{}     `wbxml:"Ping.Folders"`
	Folder  []pingFolder `wbxml:"Ping.Folder"`
}

type pingFolder struct {
	XMLName struct{} `wbxml:"Ping.Folder"`
	ID      string   `wbxml:"Ping.Id"`
	Class   string   `wbxml:"Ping.Class"`
}

// SPEC: MS-ASWBXML/marshal.tag-resolution
// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_RoundTrip_Simple(t *testing.T) {
	in := syncRoot{SyncKey: "0", WindowSz: 25}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got syncRoot
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, got) {
		t.Fatalf("round-trip differs:\n in: %+v\nout: %+v\nbytes: % X", in, got, data)
	}
}

// SPEC: MS-ASWBXML/marshal.omitempty
func TestMarshal_OmitEmpty(t *testing.T) {
	in := syncRoot{SyncKey: "X"}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// WindowSize tag identity = 0x15, content bit makes it 0x55. It must NOT
	// appear in the encoded body.
	if bytes.IndexByte(data[4:], 0x55) >= 0 {
		t.Fatalf("WindowSize unexpectedly emitted: % X", data)
	}
}

// SPEC: MS-ASWBXML/marshal.opaque
func TestMarshal_Opaque(t *testing.T) {
	raw := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	in := bodyMsg{Body: &body{Type: 4, Data: raw}}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(data, []byte{0xC3, 0x04, 0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Fatalf("OPAQUE block not present: % X", data)
	}
	var got bodyMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Body == nil || got.Body.Type != 4 || !bytes.Equal(got.Body.Data, raw) {
		t.Fatalf("round-trip body = %+v want type=4 data=% X", got.Body, raw)
	}
}

// SPEC: MS-ASWBXML/marshal.slice
// SPEC: MS-ASWBXML/marshal.roundtrip
func TestMarshal_Slice(t *testing.T) {
	in := pingRoot{
		HeartbeatInterval: 60,
		Folders: folders{
			Folder: []pingFolder{
				{ID: "1", Class: "Email"},
				{ID: "2", Class: "Calendar"},
			},
		},
	}
	data, err := Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got pingRoot
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, got) {
		t.Fatalf("round-trip differs:\n in: %+v\nout: %+v", in, got)
	}
}
