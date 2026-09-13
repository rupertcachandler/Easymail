package eas

// BusyStatus enum values per MS-ASCAL §2.2.2.8.
const (
	BusyStatusFree             int32 = 0
	BusyStatusTentative        int32 = 1
	BusyStatusBusy             int32 = 2
	BusyStatusOOF              int32 = 3
	BusyStatusWorkingElsewhere int32 = 4
)

// Sensitivity enum values per MS-ASCAL §2.2.2.46.
const (
	SensitivityNormal       int32 = 0
	SensitivityPersonal     int32 = 1
	SensitivityPrivate      int32 = 2
	SensitivityConfidential int32 = 3
)

// MeetingStatus enum values per MS-ASCAL §2.2.2.32.
const (
	MeetingStatusNonMeeting int32 = 0
	MeetingStatusMeeting    int32 = 1
)

// ValidMeetingStatus reports whether v is one of the values defined for
// calendar:MeetingStatus in MS-ASCAL §2.2.2.32 (0,1,3,5,7,9,11,13,15).
func ValidMeetingStatus(v int32) bool {
	switch v {
	case 0, 1, 3, 5, 7, 9, 11, 13, 15:
		return true
	}
	return false
}

// Recurrence Type enum values per MS-ASCAL §2.2.2.43.4.
const (
	RecurrenceDaily        int32 = 0
	RecurrenceWeekly       int32 = 1
	RecurrenceMonthly      int32 = 2
	RecurrenceMonthlyByDay int32 = 3
	RecurrenceYearly       int32 = 5
	RecurrenceYearlyByDay  int32 = 6
)

// Appointment is the sync representation of a calendar item (MS-ASCAL 14.1).
type Appointment struct {
	XMLName        struct{}    `wbxml:"AirSync.ApplicationData"`
	UID            string      `wbxml:"Calendar.UID,omitempty"`
	Subject        string      `wbxml:"Calendar.Subject,omitempty"`
	Location       string      `wbxml:"Calendar.Location,omitempty"`
	StartTime      string      `wbxml:"Calendar.StartTime,omitempty"`
	EndTime        string      `wbxml:"Calendar.EndTime,omitempty"`
	AllDayEvent    int32       `wbxml:"Calendar.AllDayEvent,omitempty"`
	OrganizerEmail string      `wbxml:"Calendar.OrganizerEmail,omitempty"`
	OrganizerName  string      `wbxml:"Calendar.OrganizerName,omitempty"`
	BusyStatus     int32       `wbxml:"Calendar.BusyStatus,omitempty"`
	Sensitivity    int32       `wbxml:"Calendar.Sensitivity,omitempty"`
	MeetingStatus  int32       `wbxml:"Calendar.MeetingStatus,omitempty"`
	Reminder       int32       `wbxml:"Calendar.Reminder,omitempty"`
	DtStamp        string      `wbxml:"Calendar.DtStamp,omitempty"`
	Categories     *Categories `wbxml:"Calendar.Categories,omitempty"`
	Attendees      *Attendees  `wbxml:"Calendar.Attendees,omitempty"`
	Recurrence     *Recurrence `wbxml:"Calendar.Recurrence,omitempty"`
}

// Categories wraps a Calendar.Categories element with its child Category
// strings.
type Categories struct {
	Category []string `wbxml:"Calendar.Category"`
}

// Attendees wraps a Calendar.Attendees element with its child Attendee
// records.
type Attendees struct {
	Attendee []Attendee `wbxml:"Calendar.Attendee"`
}

// Attendee describes a single Calendar.Attendee.
type Attendee struct {
	Email          string `wbxml:"Calendar.Email,omitempty"`
	Name           string `wbxml:"Calendar.Name,omitempty"`
	AttendeeStatus int32  `wbxml:"Calendar.AttendeeStatus,omitempty"`
	AttendeeType   int32  `wbxml:"Calendar.AttendeeType,omitempty"`
}

// Recurrence describes a calendar recurrence pattern.
type Recurrence struct {
	Type        int32  `wbxml:"Calendar.Type,omitempty"`
	Until       string `wbxml:"Calendar.Until,omitempty"`
	Occurrences int32  `wbxml:"Calendar.Occurrences,omitempty"`
	Interval    int32  `wbxml:"Calendar.Interval,omitempty"`
	DayOfWeek   int32  `wbxml:"Calendar.DayOfWeek,omitempty"`
	DayOfMonth  int32  `wbxml:"Calendar.DayOfMonth,omitempty"`
	WeekOfMonth int32  `wbxml:"Calendar.WeekOfMonth,omitempty"`
	MonthOfYear int32  `wbxml:"Calendar.MonthOfYear,omitempty"`
}
