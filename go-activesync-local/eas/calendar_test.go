package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASCAL/busystatus.enum
func TestBusyStatus_Enum(t *testing.T) {
	if BusyStatusFree != 0 || BusyStatusTentative != 1 || BusyStatusBusy != 2 ||
		BusyStatusOOF != 3 || BusyStatusWorkingElsewhere != 4 {
		t.Fatalf("BusyStatus enum mismatch")
	}
}

// SPEC: MS-ASCAL/sensitivity.enum
func TestSensitivity_Enum(t *testing.T) {
	if SensitivityNormal != 0 || SensitivityPersonal != 1 || SensitivityPrivate != 2 || SensitivityConfidential != 3 {
		t.Fatalf("Sensitivity enum mismatch")
	}
}

// SPEC: MS-ASCAL/meetingstatus.enum
func TestMeetingStatus_Enum(t *testing.T) {
	allowed := map[int32]bool{
		MeetingStatusNonMeeting: true,
		MeetingStatusMeeting:    true,
		3:                       true,
		5:                       true,
		7:                       true,
		9:                       true,
		11:                      true,
		13:                      true,
		15:                      true,
	}
	for v := range allowed {
		if !ValidMeetingStatus(v) {
			t.Errorf("ValidMeetingStatus(%d) = false, want true", v)
		}
	}
	for _, v := range []int32{2, 4, 6, 8, 10, 12, 14, 16, -1} {
		if ValidMeetingStatus(v) {
			t.Errorf("ValidMeetingStatus(%d) = true, want false", v)
		}
	}
}

// SPEC: MS-ASCAL/recurrence.type.enum
func TestRecurrenceType_Enum(t *testing.T) {
	if RecurrenceDaily != 0 || RecurrenceWeekly != 1 || RecurrenceMonthly != 2 ||
		RecurrenceMonthlyByDay != 3 || RecurrenceYearly != 5 || RecurrenceYearlyByDay != 6 {
		t.Fatalf("Recurrence type enum mismatch")
	}
}

// SPEC: MS-ASCAL/fields.14.1
func TestAppointment_Fields_RoundTrip(t *testing.T) {
	in := Appointment{
		UID:            "uid-123",
		Subject:        "Standup",
		Location:       "HQ",
		StartTime:      "20250101T090000Z",
		EndTime:        "20250101T093000Z",
		AllDayEvent:    0,
		OrganizerEmail: "alice@example.com",
		OrganizerName:  "Alice",
		BusyStatus:     int32(BusyStatusBusy),
		Sensitivity:    int32(SensitivityNormal),
		MeetingStatus:  int32(MeetingStatusMeeting),
		Reminder:       15,
		DtStamp:        "20250101T080000Z",
		Categories:     &Categories{Category: []string{"work", "team"}},
		Attendees: &Attendees{Attendee: []Attendee{
			{Email: "bob@example.com", Name: "Bob", AttendeeStatus: 3, AttendeeType: 1},
		}},
		Recurrence: &Recurrence{Type: int32(RecurrenceDaily), Interval: 1, Occurrences: 5},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Appointment
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
