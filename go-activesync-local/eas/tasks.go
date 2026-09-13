package eas

// Task Importance enum values per MS-ASTASK §2.2.2.6.
const (
	TaskImportanceLow    int32 = 0
	TaskImportanceNormal int32 = 1
	TaskImportanceHigh   int32 = 2
)

// Task is the sync representation of a task (MS-ASTASK 14.1).
type Task struct {
	XMLName       struct{}        `wbxml:"AirSync.ApplicationData"`
	Subject       string          `wbxml:"Tasks.Subject,omitempty"`
	StartDate     string          `wbxml:"Tasks.StartDate,omitempty"`
	UtcStartDate  string          `wbxml:"Tasks.UtcStartDate,omitempty"`
	DueDate       string          `wbxml:"Tasks.DueDate,omitempty"`
	UtcDueDate    string          `wbxml:"Tasks.UtcDueDate,omitempty"`
	Importance    int32           `wbxml:"Tasks.Importance,omitempty"`
	Sensitivity   int32           `wbxml:"Tasks.Sensitivity,omitempty"`
	Complete      int32           `wbxml:"Tasks.Complete,omitempty"`
	DateCompleted string          `wbxml:"Tasks.DateCompleted,omitempty"`
	ReminderSet   int32           `wbxml:"Tasks.ReminderSet,omitempty"`
	ReminderTime  string          `wbxml:"Tasks.ReminderTime,omitempty"`
	Categories    *TaskCategories `wbxml:"Tasks.Categories,omitempty"`
	Recurrence    *TaskRecurrence `wbxml:"Tasks.Recurrence,omitempty"`
}

// TaskCategories wraps a Tasks.Categories element.
type TaskCategories struct {
	Category []string `wbxml:"Tasks.Category"`
}

// TaskRecurrence describes a task recurrence pattern.
type TaskRecurrence struct {
	Type        int32  `wbxml:"Tasks.Recurrence_Type,omitempty"`
	Start       string `wbxml:"Tasks.Recurrence_Start,omitempty"`
	Until       string `wbxml:"Tasks.Recurrence_Until,omitempty"`
	Occurrences int32  `wbxml:"Tasks.Recurrence_Occurrences,omitempty"`
	Interval    int32  `wbxml:"Tasks.Recurrence_Interval,omitempty"`
	DayOfMonth  int32  `wbxml:"Tasks.Recurrence_DayOfMonth,omitempty"`
	DayOfWeek   int32  `wbxml:"Tasks.Recurrence_DayOfWeek,omitempty"`
	WeekOfMonth int32  `wbxml:"Tasks.Recurrence_WeekOfMonth,omitempty"`
	MonthOfYear int32  `wbxml:"Tasks.Recurrence_MonthOfYear,omitempty"`
	Regenerate  int32  `wbxml:"Tasks.Recurrence_Regenerate,omitempty"`
	DeadOccur   int32  `wbxml:"Tasks.Recurrence_DeadOccur,omitempty"`
}
