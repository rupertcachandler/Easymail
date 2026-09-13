package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASTASK/importance.enum
func TestTaskImportance_Enum(t *testing.T) {
	if TaskImportanceLow != 0 || TaskImportanceNormal != 1 || TaskImportanceHigh != 2 {
		t.Fatalf("task importance enum mismatch")
	}
}

// SPEC: MS-ASTASK/fields.14.1
func TestTask_Fields_RoundTrip(t *testing.T) {
	in := Task{
		Subject:      "Write report",
		StartDate:    "20250101T090000Z",
		DueDate:      "20250105T170000Z",
		UtcStartDate: "20250101T090000Z",
		UtcDueDate:   "20250105T170000Z",
		Importance:   int32(TaskImportanceHigh),
		Sensitivity:  int32(SensitivityPersonal),
		Complete:     0,
		ReminderSet:  1,
		ReminderTime: "20250104T170000Z",
		Categories:   &TaskCategories{Category: []string{"work"}},
		Recurrence: &TaskRecurrence{
			Type:     int32(RecurrenceWeekly),
			Interval: 1,
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Task
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
