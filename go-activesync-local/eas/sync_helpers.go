package eas

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/remdev/go-activesync/wbxml"
)

// applicationDataIdentity is the AirSync.ApplicationData tag identity (0x1D).
// The wrapper used by UnmarshalApplicationData re-emits this element so the
// captured raw body can be parsed by domain types whose root XMLName is
// AirSync.ApplicationData (Email, Appointment, Contact, Task, …).
const applicationDataIdentity byte = 0x1D

// ErrEmptyApplicationData is returned when a typed Sync helper is asked to
// decode a SyncAdd / SyncChange whose ApplicationData element was either
// missing or carried an empty body.
var ErrEmptyApplicationData = errors.New("eas: ApplicationData is empty")

// CaptureApplicationDataBody marshals the given value (whose XMLName must be
// AirSync.ApplicationData) and returns just the body bytes that appear between
// the open ApplicationData tag and the matching END token. This is the inverse
// of UnmarshalApplicationData: it lets a client build a SyncAdd / SyncChange
// ApplicationData payload (*wbxml.RawElement{Page: wbxml.PageAirSync, Bytes:
// body}) for sending a new or modified item to the server.
func CaptureApplicationDataBody(v any) ([]byte, error) {
	data, err := wbxml.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := wbxml.NewDecoder(bytes.NewReader(data))
	if _, err := dec.ReadHeader(); err != nil {
		return nil, err
	}
	tok, err := dec.NextToken()
	if err != nil {
		return nil, err
	}
	body, err := dec.CaptureRaw(tok.HasContent)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// UnmarshalApplicationData decodes the body bytes carried by a *wbxml.RawElement
// captured from a SyncAdd / SyncChange into a typed value of T. The caller is
// responsible for picking the right T for the collection's Class (e.g.
// *Email for "Email", *Appointment for "Calendar"); a wrong choice surfaces
// as a wbxml decoding error, never as a panic.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func UnmarshalApplicationData[T any](raw *wbxml.RawElement) (*T, error) {
	if raw == nil || len(raw.Bytes) == 0 {
		return nil, ErrEmptyApplicationData
	}
	wrapper, err := buildApplicationDataWrapper(raw)
	if err != nil {
		return nil, err
	}
	out := new(T)
	if err := wbxml.Unmarshal(wrapper, out); err != nil {
		return nil, fmt.Errorf("eas: decode ApplicationData: %w", err)
	}
	return out, nil
}

// buildApplicationDataWrapper produces a complete WBXML 1.3 document of the
// shape:
//
//	<AirSync.ApplicationData>raw.Bytes</AirSync.ApplicationData>
//
// with the encoder's active page aligned to raw.Page just before the body so
// the captured bytes can be replayed verbatim regardless of which page they
// originally referenced. Writes target a bytes.Buffer, which never returns
// an error; only ForceSwitchPage can fail, when raw.Page is unknown.
func buildApplicationDataWrapper(raw *wbxml.RawElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := wbxml.NewEncoder(&buf)
	var firstErr error
	push := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	push(enc.WriteHeader(wbxml.Header{Version: 0x03, PublicID: 0x01, Charset: 0x6A}))
	push(enc.StartTag(wbxml.PageAirSync, applicationDataIdentity, false, true))
	push(enc.ForceSwitchPage(raw.Page))
	push(enc.WriteRaw(raw.Bytes))
	push(enc.EndTag())
	if firstErr != nil {
		return nil, firstErr
	}
	return buf.Bytes(), nil
}

// Email decodes the SyncAdd's ApplicationData as an *Email.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (a SyncAdd) Email() (*Email, error) {
	return UnmarshalApplicationData[Email](a.ApplicationData)
}

// Appointment decodes the SyncAdd's ApplicationData as an *Appointment.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (a SyncAdd) Appointment() (*Appointment, error) {
	return UnmarshalApplicationData[Appointment](a.ApplicationData)
}

// Contact decodes the SyncAdd's ApplicationData as a *Contact.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (a SyncAdd) Contact() (*Contact, error) {
	return UnmarshalApplicationData[Contact](a.ApplicationData)
}

// Task decodes the SyncAdd's ApplicationData as a *Task.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (a SyncAdd) Task() (*Task, error) {
	return UnmarshalApplicationData[Task](a.ApplicationData)
}

// Email decodes the SyncChange's ApplicationData as an *Email.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (c SyncChange) Email() (*Email, error) {
	return UnmarshalApplicationData[Email](c.ApplicationData)
}

// Appointment decodes the SyncChange's ApplicationData as an *Appointment.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (c SyncChange) Appointment() (*Appointment, error) {
	return UnmarshalApplicationData[Appointment](c.ApplicationData)
}

// Contact decodes the SyncChange's ApplicationData as a *Contact.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (c SyncChange) Contact() (*Contact, error) {
	return UnmarshalApplicationData[Contact](c.ApplicationData)
}

// Task decodes the SyncChange's ApplicationData as a *Task.
//
// SPEC: MS-ASCMD/sync.applicationdata.typed
func (c SyncChange) Task() (*Task, error) {
	return UnmarshalApplicationData[Task](c.ApplicationData)
}
