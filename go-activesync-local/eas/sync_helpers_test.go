package eas

import (
	"bytes"
	"errors"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// captureApplicationDataBody marshals the given value (whose XMLName must be
// AirSync.ApplicationData) and returns just the body bytes that appear
// between the open ApplicationData tag and the matching END token, suitable
// for use as RawElement.Bytes.
func captureApplicationDataBody(t *testing.T, v any) []byte {
	t.Helper()
	data, err := wbxml.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	dec := wbxml.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	tok, err := dec.NextToken()
	if err != nil {
		t.Fatalf("NextToken: %v", err)
	}
	body, err := dec.CaptureRaw(tok.HasContent)
	if err != nil {
		t.Fatalf("CaptureRaw: %v", err)
	}
	return body
}

// SPEC: MS-ASCMD/sync.applicationdata.raw
func TestSyncAdd_ChangeApplicationDataIsRawElement(t *testing.T) {
	in := SyncResponse{
		Collections: SyncCollections{Collection: []SyncCollection{{
			SyncKey:      "S-1",
			CollectionID: "1",
			Status:       int32(SyncStatusSuccess),
			Commands: &SyncCommands{
				Add: []SyncAdd{{
					ServerID:        "1",
					ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: captureApplicationDataBody(t, &Email{Subject: "x"})},
				}},
				Change: []SyncChange{{
					ServerID:        "1",
					ApplicationData: &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: captureApplicationDataBody(t, &Email{Subject: "x"})},
				}},
			},
		}}},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out SyncResponse
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	cmd := out.Collections.Collection[0].Commands
	if cmd == nil || len(cmd.Add) != 1 || cmd.Add[0].ApplicationData == nil {
		t.Fatalf("Add ApplicationData missing: %+v", cmd)
	}
	if len(cmd.Change) != 1 || cmd.Change[0].ApplicationData == nil {
		t.Fatalf("Change ApplicationData missing: %+v", cmd)
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestSyncAdd_TypedHelpers(t *testing.T) {
	emailBody := captureApplicationDataBody(t, &Email{Subject: "hi", From: "a@b"})
	apptBody := captureApplicationDataBody(t, &Appointment{Subject: "meet"})
	contactBody := captureApplicationDataBody(t, &Contact{FirstName: "A", LastName: "B"})
	taskBody := captureApplicationDataBody(t, &Task{Subject: "todo"})

	page := wbxml.PageAirSync
	tests := []struct {
		name string
		body []byte
		fn   func(SyncAdd) (any, error)
		want any
	}{
		{
			name: "email",
			body: emailBody,
			fn:   func(a SyncAdd) (any, error) { return a.Email() },
			want: &Email{Subject: "hi", From: "a@b"},
		},
		{
			name: "appointment",
			body: apptBody,
			fn:   func(a SyncAdd) (any, error) { return a.Appointment() },
			want: &Appointment{Subject: "meet"},
		},
		{
			name: "contact",
			body: contactBody,
			fn:   func(a SyncAdd) (any, error) { return a.Contact() },
			want: &Contact{FirstName: "A", LastName: "B"},
		},
		{
			name: "task",
			body: taskBody,
			fn:   func(a SyncAdd) (any, error) { return a.Task() },
			want: &Task{Subject: "todo"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			add := SyncAdd{ApplicationData: &wbxml.RawElement{Page: page, Bytes: tc.body}}
			got, err := tc.fn(add)
			if err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}
			if !equalApplicationData(t, got, tc.want) {
				t.Fatalf("decode %s mismatch: got %+v want %+v", tc.name, got, tc.want)
			}
		})
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestSyncChange_TypedHelpers(t *testing.T) {
	page := wbxml.PageAirSync
	chg := SyncChange{
		ApplicationData: &wbxml.RawElement{
			Page:  page,
			Bytes: captureApplicationDataBody(t, &Email{Subject: "z"}),
		},
	}
	if got, err := chg.Email(); err != nil || got.Subject != "z" {
		t.Fatalf("Email: %+v err=%v", got, err)
	}
	chg.ApplicationData = &wbxml.RawElement{Page: page, Bytes: captureApplicationDataBody(t, &Appointment{Subject: "m"})}
	if got, err := chg.Appointment(); err != nil || got.Subject != "m" {
		t.Fatalf("Appointment: %+v err=%v", got, err)
	}
	chg.ApplicationData = &wbxml.RawElement{Page: page, Bytes: captureApplicationDataBody(t, &Contact{FirstName: "f"})}
	if got, err := chg.Contact(); err != nil || got.FirstName != "f" {
		t.Fatalf("Contact: %+v err=%v", got, err)
	}
	chg.ApplicationData = &wbxml.RawElement{Page: page, Bytes: captureApplicationDataBody(t, &Task{Subject: "tt"})}
	if got, err := chg.Task(); err != nil || got.Subject != "tt" {
		t.Fatalf("Task: %+v err=%v", got, err)
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestUnmarshalApplicationData_Empty(t *testing.T) {
	if _, err := UnmarshalApplicationData[Email](nil); !errors.Is(err, ErrEmptyApplicationData) {
		t.Fatalf("nil raw: want ErrEmptyApplicationData, got %v", err)
	}
	if _, err := UnmarshalApplicationData[Email](&wbxml.RawElement{}); !errors.Is(err, ErrEmptyApplicationData) {
		t.Fatalf("empty raw: want ErrEmptyApplicationData, got %v", err)
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestUnmarshalApplicationData_DecodeError(t *testing.T) {
	raw := &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: []byte{0xFF, 0xFF}}
	if _, err := UnmarshalApplicationData[Email](raw); err == nil {
		t.Fatal("expected decode error for malformed body")
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestUnmarshalApplicationData_UnknownPage(t *testing.T) {
	raw := &wbxml.RawElement{Page: 0xFF, Bytes: []byte{0x01}}
	if _, err := UnmarshalApplicationData[Email](raw); err == nil {
		t.Fatal("expected wrapper error for unknown page")
	}
}

// SPEC: MS-ASCMD/sync.applicationdata.typed
func TestUnmarshalApplicationData_WrongType(t *testing.T) {
	body := captureApplicationDataBody(t, &Email{Subject: "x"})
	raw := &wbxml.RawElement{Page: wbxml.PageAirSync, Bytes: body}
	// Decoding an Email body into a Task is permissive (unknown tags are
	// skipped), but the resulting Task carries no fields from the email.
	got, err := UnmarshalApplicationData[Task](raw)
	if err != nil {
		t.Fatalf("UnmarshalApplicationData[Task]: %v", err)
	}
	if got.Subject != "" {
		t.Fatalf("expected empty Task, got %+v", got)
	}
}

func equalApplicationData(t *testing.T, got, want any) bool {
	t.Helper()
	switch w := want.(type) {
	case *Email:
		g, ok := got.(*Email)
		return ok && g.Subject == w.Subject && g.From == w.From && g.To == w.To
	case *Appointment:
		g, ok := got.(*Appointment)
		return ok && g.Subject == w.Subject
	case *Contact:
		g, ok := got.(*Contact)
		return ok && g.FirstName == w.FirstName && g.LastName == w.LastName
	case *Task:
		g, ok := got.(*Task)
		return ok && g.Subject == w.Subject
	}
	t.Fatalf("unsupported type %T", want)
	return false
}
