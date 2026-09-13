package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASEMAIL/importance.enum
func TestImportance_Enum(t *testing.T) {
	if ImportanceLow != 0 || ImportanceNormal != 1 || ImportanceHigh != 2 {
		t.Fatalf("importance enum = %d/%d/%d, want 0/1/2",
			ImportanceLow, ImportanceNormal, ImportanceHigh)
	}
}

// SPEC: MS-ASEMAIL/fields.14.1
func TestEmail_Fields_RoundTrip(t *testing.T) {
	in := Email{
		DateReceived: "20250101T120000Z",
		Subject:      "Hello",
		From:         "alice@example.com",
		To:           "bob@example.com",
		Cc:           "cc@example.com",
		ReplyTo:      "alice@example.com",
		DisplayTo:    "Bob",
		ThreadTopic:  "Hello",
		Importance:   int32(ImportanceHigh),
		Read:         true,
		MessageClass: "IPM.Note",
		ContentClass: "urn:content-classes:message",
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Email
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
