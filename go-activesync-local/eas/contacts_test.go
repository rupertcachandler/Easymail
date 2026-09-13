package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCNTC/fields.14.1
func TestContact_Fields_RoundTrip(t *testing.T) {
	in := Contact{
		FirstName:           "Alice",
		LastName:            "Smith",
		MiddleName:          "Q",
		Title:               "Ms.",
		CompanyName:         "Acme",
		JobTitle:            "Engineer",
		Email1Address:       "a@example.com",
		Email2Address:       "alice@personal.example",
		Email3Address:       "alice3@example.com",
		HomePhoneNumber:     "+1 555-0100",
		MobilePhoneNumber:   "+1 555-0101",
		BusinessPhoneNumber: "+1 555-0102",
		HomeStreet:          "1 Main St",
		HomeCity:            "Springfield",
		HomePostalCode:      "12345",
		HomeCountry:         "USA",
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Contact
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
