package wbxml

import (
	"sort"
	"testing"
)

// expectedPage describes the well-known shape of a single MS-ASWBXML 14.1
// code page: its numeric identifier, canonical name, and a small set of
// signature tags whose presence and token values are pinned down by the spec.
type expectedPage struct {
	id   byte
	name string
	tags []expectedTag
}

type expectedTag struct {
	name  string
	token byte
}

// Signature tags below are taken from the per-page tables in MS-ASWBXML
// §2.1.2.1.x; they are intentionally a small but representative selection
// whose token codes appear verbatim in the spec.
var allPages = []expectedPage{
	// SPEC: MS-ASWBXML/codepage.0.AirSync
	{id: 0, name: "AirSync", tags: []expectedTag{
		{"Sync", 0x05}, {"Responses", 0x06}, {"Add", 0x07}, {"Change", 0x08},
		{"Delete", 0x09}, {"Fetch", 0x0A}, {"SyncKey", 0x0B}, {"ClientId", 0x0C},
		{"ServerId", 0x0D}, {"Status", 0x0E}, {"Collection", 0x0F},
		{"Class", 0x10}, {"CollectionId", 0x12}, {"GetChanges", 0x13},
		{"MoreAvailable", 0x14}, {"WindowSize", 0x15}, {"Commands", 0x16},
		{"Options", 0x17}, {"FilterType", 0x18}, {"Conflict", 0x1B},
		{"Collections", 0x1C}, {"ApplicationData", 0x1D}, {"DeletesAsMoves", 0x1E},
		{"Supported", 0x20}, {"SoftDelete", 0x21}, {"MIMESupport", 0x22},
		{"MIMETruncation", 0x23}, {"Wait", 0x24}, {"Limit", 0x25},
		{"Partial", 0x26}, {"ConversationMode", 0x27}, {"MaxItems", 0x28},
		{"HeartbeatInterval", 0x29},
	}},
	// SPEC: MS-ASWBXML/codepage.1.Contacts
	{id: 1, name: "Contacts", tags: []expectedTag{
		{"Anniversary", 0x05}, {"AssistantName", 0x06}, {"AssistantPhoneNumber", 0x07},
		{"Birthday", 0x08}, {"Business2PhoneNumber", 0x0C}, {"BusinessCity", 0x0D},
		{"BusinessCountry", 0x0E}, {"BusinessPostalCode", 0x0F}, {"BusinessState", 0x10},
		{"BusinessStreet", 0x11}, {"BusinessFaxNumber", 0x12}, {"BusinessPhoneNumber", 0x13},
		{"CarPhoneNumber", 0x14}, {"Categories", 0x15}, {"Category", 0x16},
		{"Children", 0x17}, {"Child", 0x18}, {"CompanyName", 0x19},
		{"Department", 0x1A}, {"Email1Address", 0x1B}, {"Email2Address", 0x1C},
		{"Email3Address", 0x1D}, {"FileAs", 0x1E}, {"FirstName", 0x1F},
		{"Home2PhoneNumber", 0x20}, {"HomeCity", 0x21}, {"HomeCountry", 0x22},
		{"HomePostalCode", 0x23}, {"HomeState", 0x24}, {"HomeStreet", 0x25},
		{"HomeFaxNumber", 0x26}, {"HomePhoneNumber", 0x27}, {"JobTitle", 0x28},
		{"LastName", 0x29}, {"MiddleName", 0x2A}, {"MobilePhoneNumber", 0x2B},
		{"OfficeLocation", 0x2C}, {"OtherCity", 0x2D}, {"OtherCountry", 0x2E},
		{"OtherPostalCode", 0x2F}, {"OtherState", 0x30}, {"OtherStreet", 0x31},
		{"PagerNumber", 0x32}, {"RadioPhoneNumber", 0x33}, {"Spouse", 0x34},
		{"Suffix", 0x35}, {"Title", 0x36}, {"WebPage", 0x37},
		{"YomiCompanyName", 0x38}, {"YomiFirstName", 0x39}, {"YomiLastName", 0x3A},
		{"Picture", 0x3C}, {"Alias", 0x3D}, {"WeightedRank", 0x3E},
	}},
	// SPEC: MS-ASWBXML/codepage.2.Email
	{id: 2, name: "Email", tags: []expectedTag{
		{"DateReceived", 0x0F}, {"DisplayTo", 0x11}, {"Importance", 0x12},
		{"MessageClass", 0x13}, {"Subject", 0x14}, {"Read", 0x15}, {"To", 0x16},
		{"Cc", 0x17}, {"From", 0x18}, {"ReplyTo", 0x19}, {"AllDayEvent", 0x1A},
		{"Categories", 0x1B}, {"Category", 0x1C}, {"DtStamp", 0x1D},
		{"EndTime", 0x1E}, {"InstanceType", 0x1F}, {"BusyStatus", 0x20},
		{"Location", 0x21}, {"MeetingRequest", 0x22}, {"Organizer", 0x23},
		{"RecurrenceId", 0x24}, {"Reminder", 0x25}, {"ResponseRequested", 0x26},
		{"Recurrences", 0x27}, {"Recurrence", 0x28}, {"Recurrence_Type", 0x29},
		{"Recurrence_Until", 0x2A}, {"Recurrence_Occurrences", 0x2B},
		{"Recurrence_Interval", 0x2C}, {"Recurrence_DayOfWeek", 0x2D},
		{"Recurrence_DayOfMonth", 0x2E}, {"Recurrence_WeekOfMonth", 0x2F},
		{"Recurrence_MonthOfYear", 0x30}, {"StartTime", 0x31}, {"Sensitivity", 0x32},
		{"TimeZone", 0x33}, {"GlobalObjId", 0x34}, {"ThreadTopic", 0x35},
		{"InternetCPID", 0x39}, {"Flag", 0x3A}, {"FlagStatus", 0x3B},
		{"ContentClass", 0x3C}, {"FlagType", 0x3D}, {"CompleteTime", 0x3E},
		{"DisallowNewTimeProposal", 0x3F},
	}},
	// SPEC: MS-ASWBXML/codepage.3.AirNotify
	{id: 3, name: "AirNotify", tags: []expectedTag{}},
	// SPEC: MS-ASWBXML/codepage.4.Calendar
	{id: 4, name: "Calendar", tags: []expectedTag{
		{"TimeZone", 0x05}, {"AllDayEvent", 0x06}, {"Attendees", 0x07},
		{"Attendee", 0x08}, {"Email", 0x09}, {"Name", 0x0A},
		{"BusyStatus", 0x0D}, {"Categories", 0x0E}, {"Category", 0x0F},
		{"DtStamp", 0x11}, {"EndTime", 0x12}, {"Exception", 0x13},
		{"Exceptions", 0x14}, {"Deleted", 0x15}, {"ExceptionStartTime", 0x16},
		{"Location", 0x17}, {"MeetingStatus", 0x18}, {"OrganizerEmail", 0x19},
		{"OrganizerName", 0x1A}, {"Recurrence", 0x1B}, {"Type", 0x1C},
		{"Until", 0x1D}, {"Occurrences", 0x1E}, {"Interval", 0x1F},
		{"DayOfWeek", 0x20}, {"DayOfMonth", 0x21}, {"WeekOfMonth", 0x22},
		{"MonthOfYear", 0x23}, {"Reminder", 0x24}, {"Sensitivity", 0x25},
		{"Subject", 0x26}, {"StartTime", 0x27}, {"UID", 0x28},
		{"AttendeeStatus", 0x29}, {"AttendeeType", 0x2A}, {"DisallowNewTimeProposal", 0x33},
		{"ResponseRequested", 0x34}, {"AppointmentReplyTime", 0x35},
		{"ResponseType", 0x36}, {"CalendarType", 0x37}, {"IsLeapMonth", 0x38},
		{"FirstDayOfWeek", 0x39}, {"OnlineMeetingConfLink", 0x3A},
		{"OnlineMeetingExternalLink", 0x3B},
	}},
	// SPEC: MS-ASWBXML/codepage.5.Move
	{id: 5, name: "Move", tags: []expectedTag{
		{"MoveItems", 0x05}, {"Move", 0x06}, {"SrcMsgId", 0x07},
		{"SrcFldId", 0x08}, {"DstFldId", 0x09}, {"Response", 0x0A},
		{"Status", 0x0B}, {"DstMsgId", 0x0C},
	}},
	// SPEC: MS-ASWBXML/codepage.6.GetItemEstimate
	{id: 6, name: "GetItemEstimate", tags: []expectedTag{
		{"GetItemEstimate", 0x05}, {"Version", 0x06}, {"Collections", 0x07},
		{"Collection", 0x08}, {"Class", 0x09}, {"CollectionId", 0x0A},
		{"DateTime", 0x0B}, {"Estimate", 0x0C}, {"Response", 0x0D},
		{"Status", 0x0E},
	}},
	// SPEC: MS-ASWBXML/codepage.7.FolderHierarchy
	{id: 7, name: "FolderHierarchy", tags: []expectedTag{
		{"DisplayName", 0x07}, {"ServerId", 0x08}, {"ParentId", 0x09},
		{"Type", 0x0A}, {"Status", 0x0C}, {"Changes", 0x0E},
		{"Add", 0x0F}, {"Delete", 0x10}, {"Update", 0x11},
		{"SyncKey", 0x12}, {"FolderCreate", 0x13}, {"FolderDelete", 0x14},
		{"FolderUpdate", 0x15}, {"FolderSync", 0x16}, {"Count", 0x17},
	}},
	// SPEC: MS-ASWBXML/codepage.8.MeetingResponse
	{id: 8, name: "MeetingResponse", tags: []expectedTag{
		{"CalendarId", 0x05}, {"CollectionId", 0x06}, {"MeetingResponse", 0x07},
		{"RequestId", 0x08}, {"Request", 0x09}, {"Result", 0x0A},
		{"Status", 0x0B}, {"UserResponse", 0x0C}, {"InstanceId", 0x0E},
	}},
	// SPEC: MS-ASWBXML/codepage.9.Tasks
	{id: 9, name: "Tasks", tags: []expectedTag{
		{"Categories", 0x08}, {"Category", 0x09}, {"Complete", 0x0A},
		{"DateCompleted", 0x0B}, {"DueDate", 0x0C}, {"UtcDueDate", 0x0D},
		{"Importance", 0x0E}, {"Recurrence", 0x0F}, {"Recurrence_Type", 0x10},
		{"Recurrence_Start", 0x11}, {"Recurrence_Until", 0x12},
		{"Recurrence_Occurrences", 0x13}, {"Recurrence_Interval", 0x14},
		{"Recurrence_DayOfMonth", 0x15}, {"Recurrence_DayOfWeek", 0x16},
		{"Recurrence_WeekOfMonth", 0x17}, {"Recurrence_MonthOfYear", 0x18},
		{"Recurrence_Regenerate", 0x19}, {"Recurrence_DeadOccur", 0x1A},
		{"ReminderSet", 0x1B}, {"ReminderTime", 0x1C}, {"Sensitivity", 0x1D},
		{"StartDate", 0x1E}, {"UtcStartDate", 0x1F}, {"Subject", 0x20},
		{"OrdinalDate", 0x22}, {"SubOrdinalDate", 0x23}, {"CalendarType", 0x24},
		{"IsLeapMonth", 0x25}, {"FirstDayOfWeek", 0x26},
	}},
	// SPEC: MS-ASWBXML/codepage.10.ResolveRecipients
	{id: 10, name: "ResolveRecipients", tags: []expectedTag{
		{"ResolveRecipients", 0x05}, {"Response", 0x06}, {"Status", 0x07},
		{"Type", 0x08}, {"Recipient", 0x09}, {"DisplayName", 0x0A},
		{"EmailAddress", 0x0B}, {"Certificates", 0x0C}, {"Certificate", 0x0D},
		{"MiniCertificate", 0x0E}, {"Options", 0x0F}, {"To", 0x10},
		{"CertificateRetrieval", 0x11}, {"RecipientCount", 0x12},
		{"MaxCertificates", 0x13}, {"MaxAmbiguousRecipients", 0x14},
		{"CertificateCount", 0x15}, {"Availability", 0x16}, {"StartTime", 0x17},
		{"EndTime", 0x18}, {"MergedFreeBusy", 0x19}, {"Picture", 0x1A},
		{"MaxSize", 0x1B}, {"Data", 0x1C}, {"MaxPictures", 0x1D},
	}},
	// SPEC: MS-ASWBXML/codepage.11.ValidateCert
	{id: 11, name: "ValidateCert", tags: []expectedTag{
		{"ValidateCert", 0x05}, {"Certificates", 0x06}, {"Certificate", 0x07},
		{"CertificateChain", 0x08}, {"CheckCRL", 0x09}, {"Status", 0x0A},
	}},
	// SPEC: MS-ASWBXML/codepage.12.Contacts2
	{id: 12, name: "Contacts2", tags: []expectedTag{
		{"CustomerId", 0x05}, {"GovernmentId", 0x06}, {"IMAddress", 0x07},
		{"IMAddress2", 0x08}, {"IMAddress3", 0x09}, {"ManagerName", 0x0A},
		{"CompanyMainPhone", 0x0B}, {"AccountName", 0x0C}, {"NickName", 0x0D},
		{"MMS", 0x0E},
	}},
	// SPEC: MS-ASWBXML/codepage.13.Ping
	{id: 13, name: "Ping", tags: []expectedTag{
		{"Ping", 0x05}, {"AutdState", 0x06}, {"Status", 0x07},
		{"HeartbeatInterval", 0x08}, {"Folders", 0x09}, {"Folder", 0x0A},
		{"Id", 0x0B}, {"Class", 0x0C}, {"MaxFolders", 0x0D},
	}},
	// SPEC: MS-ASWBXML/codepage.14.Provision
	{id: 14, name: "Provision", tags: []expectedTag{
		{"Provision", 0x05}, {"Policies", 0x06}, {"Policy", 0x07},
		{"PolicyType", 0x08}, {"PolicyKey", 0x09}, {"Data", 0x0A},
		{"Status", 0x0B}, {"RemoteWipe", 0x0C}, {"EASProvisionDoc", 0x0D},
		{"DevicePasswordEnabled", 0x0E}, {"AlphanumericDevicePasswordRequired", 0x0F},
		{"PasswordRecoveryEnabled", 0x11}, {"AttachmentsEnabled", 0x13},
		{"MinDevicePasswordLength", 0x14}, {"MaxInactivityTimeDeviceLock", 0x15},
		{"MaxDevicePasswordFailedAttempts", 0x16}, {"MaxAttachmentSize", 0x17},
		{"AllowSimpleDevicePassword", 0x18}, {"DevicePasswordExpiration", 0x19},
		{"DevicePasswordHistory", 0x1A}, {"AllowStorageCard", 0x1B},
		{"AllowCamera", 0x1C}, {"RequireDeviceEncryption", 0x1D},
		{"AllowUnsignedApplications", 0x1E}, {"AllowUnsignedInstallationPackages", 0x1F},
		{"MinDevicePasswordComplexCharacters", 0x20}, {"AllowWiFi", 0x21},
		{"AllowTextMessaging", 0x22}, {"AllowPOPIMAPEmail", 0x23},
		{"AllowBluetooth", 0x24}, {"AllowIrDA", 0x25},
		{"RequireManualSyncWhenRoaming", 0x26}, {"AllowDesktopSync", 0x27},
		{"MaxCalendarAgeFilter", 0x28}, {"AllowHTMLEmail", 0x29},
		{"MaxEmailAgeFilter", 0x2A}, {"MaxEmailBodyTruncationSize", 0x2B},
		{"MaxEmailHTMLBodyTruncationSize", 0x2C}, {"RequireSignedSMIMEMessages", 0x2D},
		{"RequireEncryptedSMIMEMessages", 0x2E}, {"RequireSignedSMIMEAlgorithm", 0x2F},
		{"RequireEncryptionSMIMEAlgorithm", 0x30}, {"AllowSMIMEEncryptionAlgorithmNegotiation", 0x31},
		{"AllowSMIMESoftCerts", 0x32}, {"AllowBrowser", 0x33},
		{"AllowConsumerEmail", 0x34}, {"AllowRemoteDesktop", 0x35},
		{"AllowInternetSharing", 0x36}, {"UnapprovedInROMApplicationList", 0x37},
		{"ApplicationName", 0x38}, {"ApprovedApplicationList", 0x39},
		{"Hash", 0x3A},
	}},
	// SPEC: MS-ASWBXML/codepage.15.Search
	{id: 15, name: "Search", tags: []expectedTag{
		{"Search", 0x05}, {"Store", 0x07}, {"Name", 0x08}, {"Query", 0x09},
		{"Options", 0x0A}, {"Range", 0x0B}, {"Status", 0x0C},
		{"Response", 0x0D}, {"Result", 0x0E}, {"Properties", 0x0F},
		{"Total", 0x10}, {"EqualTo", 0x11}, {"Value", 0x12},
		{"And", 0x13}, {"Or", 0x14}, {"FreeText", 0x15},
		{"DeepTraversal", 0x17}, {"LongId", 0x18}, {"RebuildResults", 0x19},
		{"LessThan", 0x1A}, {"GreaterThan", 0x1B}, {"Schema", 0x1C},
		{"Supported", 0x1D}, {"UserName", 0x1E}, {"Password", 0x1F},
		{"ConversationId", 0x20}, {"Picture", 0x21}, {"MaxSize", 0x22},
		{"MaxPictures", 0x23},
	}},
	// SPEC: MS-ASWBXML/codepage.16.GAL
	{id: 16, name: "GAL", tags: []expectedTag{
		{"DisplayName", 0x05}, {"Phone", 0x06}, {"Office", 0x07},
		{"Title", 0x08}, {"Company", 0x09}, {"Alias", 0x0A},
		{"FirstName", 0x0B}, {"LastName", 0x0C}, {"HomePhone", 0x0D},
		{"MobilePhone", 0x0E}, {"EmailAddress", 0x0F}, {"Picture", 0x10},
		{"Status", 0x11}, {"Data", 0x12},
	}},
	// SPEC: MS-ASWBXML/codepage.17.AirSyncBase
	{id: 17, name: "AirSyncBase", tags: []expectedTag{
		{"BodyPreference", 0x05}, {"Type", 0x06}, {"TruncationSize", 0x07},
		{"AllOrNone", 0x08}, {"Body", 0x0A}, {"Data", 0x0B},
		{"EstimatedDataSize", 0x0C}, {"Truncated", 0x0D}, {"Attachments", 0x0E},
		{"Attachment", 0x0F}, {"DisplayName", 0x10}, {"FileReference", 0x11},
		{"Method", 0x12}, {"ContentId", 0x13}, {"ContentLocation", 0x14},
		{"IsInline", 0x15}, {"NativeBodyType", 0x16}, {"ContentType", 0x17},
		{"Preview", 0x18}, {"BodyPartPreference", 0x19}, {"BodyPart", 0x1A},
		{"Status", 0x1B},
	}},
	// SPEC: MS-ASWBXML/codepage.18.Settings
	{id: 18, name: "Settings", tags: []expectedTag{
		{"Settings", 0x05}, {"Status", 0x06}, {"Get", 0x07}, {"Set", 0x08},
		{"Oof", 0x09}, {"OofState", 0x0A}, {"StartTime", 0x0B}, {"EndTime", 0x0C},
		{"OofMessage", 0x0D}, {"AppliesToInternal", 0x0E}, {"AppliesToExternalKnown", 0x0F},
		{"AppliesToExternalUnknown", 0x10}, {"Enabled", 0x11}, {"ReplyMessage", 0x12},
		{"BodyType", 0x13}, {"DevicePassword", 0x14}, {"Password", 0x15},
		{"DeviceInformation", 0x16}, {"Model", 0x17}, {"IMEI", 0x18},
		{"FriendlyName", 0x19}, {"OS", 0x1A}, {"OSLanguage", 0x1B},
		{"PhoneNumber", 0x1C}, {"UserInformation", 0x1D}, {"EmailAddresses", 0x1E},
		{"SmtpAddress", 0x1F}, {"UserAgent", 0x20}, {"EnableOutboundSMS", 0x21},
		{"MobileOperator", 0x22}, {"PrimarySmtpAddress", 0x23}, {"Accounts", 0x24},
		{"Account", 0x25}, {"AccountId", 0x26}, {"AccountName", 0x27},
		{"UserDisplayName", 0x28}, {"SendDisabled", 0x29}, {"RightsManagementInformation", 0x2B},
	}},
	// SPEC: MS-ASWBXML/codepage.19.DocumentLibrary
	{id: 19, name: "DocumentLibrary", tags: []expectedTag{
		{"LinkId", 0x05}, {"DisplayName", 0x06}, {"IsFolder", 0x07},
		{"CreationDate", 0x08}, {"LastModifiedDate", 0x09}, {"IsHidden", 0x0A},
		{"ContentLength", 0x0B}, {"ContentType", 0x0C},
	}},
	// SPEC: MS-ASWBXML/codepage.20.ItemOperations
	{id: 20, name: "ItemOperations", tags: []expectedTag{
		{"ItemOperations", 0x05}, {"Fetch", 0x06}, {"Store", 0x07},
		{"Options", 0x08}, {"Range", 0x09}, {"Total", 0x0A},
		{"Properties", 0x0B}, {"Data", 0x0C}, {"Status", 0x0D},
		{"Response", 0x0E}, {"Version", 0x0F}, {"Schema", 0x10},
		{"Part", 0x11}, {"EmptyFolderContents", 0x12}, {"DeleteSubFolders", 0x13},
		{"UserName", 0x14}, {"Password", 0x15}, {"Move", 0x16},
		{"DstFldId", 0x17}, {"ConversationId", 0x18}, {"MoveAlways", 0x19},
	}},
	// SPEC: MS-ASWBXML/codepage.21.ComposeMail
	{id: 21, name: "ComposeMail", tags: []expectedTag{
		{"SendMail", 0x05}, {"SmartForward", 0x06}, {"SmartReply", 0x07},
		{"SaveInSentItems", 0x08}, {"ReplaceMime", 0x09}, {"Source", 0x0B},
		{"FolderId", 0x0C}, {"ItemId", 0x0D}, {"LongId", 0x0E},
		{"InstanceId", 0x0F}, {"MIME", 0x10}, {"ClientId", 0x11},
		{"Status", 0x12}, {"AccountId", 0x13},
	}},
	// SPEC: MS-ASWBXML/codepage.22.Email2
	{id: 22, name: "Email2", tags: []expectedTag{
		{"UmCallerID", 0x05}, {"UmUserNotes", 0x06}, {"UmAttDuration", 0x07},
		{"UmAttOrder", 0x08}, {"ConversationId", 0x09}, {"ConversationIndex", 0x0A},
		{"LastVerbExecuted", 0x0B}, {"LastVerbExecutionTime", 0x0C}, {"ReceivedAsBcc", 0x0D},
		{"Sender", 0x0E}, {"CalendarType", 0x0F}, {"IsLeapMonth", 0x10},
		{"AccountId", 0x11}, {"FirstDayOfWeek", 0x12}, {"MeetingMessageType", 0x13},
		{"IsDraft", 0x15}, {"Bcc", 0x16}, {"Send", 0x17},
	}},
	// SPEC: MS-ASWBXML/codepage.23.Notes
	{id: 23, name: "Notes", tags: []expectedTag{
		{"Subject", 0x05}, {"MessageClass", 0x06}, {"LastModifiedDate", 0x07},
		{"Categories", 0x08}, {"Category", 0x09},
	}},
	// SPEC: MS-ASWBXML/codepage.24.RightsManagement
	{id: 24, name: "RightsManagement", tags: []expectedTag{
		{"RightsManagementSupport", 0x05}, {"RightsManagementTemplates", 0x06},
		{"RightsManagementTemplate", 0x07}, {"RightsManagementLicense", 0x08},
		{"EditAllowed", 0x09}, {"ReplyAllowed", 0x0A}, {"ReplyAllAllowed", 0x0B},
		{"ForwardAllowed", 0x0C}, {"ModifyRecipientsAllowed", 0x0D},
		{"ExtractAllowed", 0x0E}, {"PrintAllowed", 0x0F}, {"ExportAllowed", 0x10},
		{"ProgrammaticAccessAllowed", 0x11}, {"Owner", 0x12},
		{"ContentExpiryDate", 0x13}, {"TemplateID", 0x14}, {"TemplateName", 0x15},
		{"TemplateDescription", 0x16}, {"ContentOwner", 0x17}, {"RemoveRightsManagementProtection", 0x18},
	}},
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodePages_AllPagesRegistered(t *testing.T) {
	for _, want := range allPages {
		t.Run(want.name, func(t *testing.T) {
			page, ok := PageByID(want.id)
			if !ok {
				t.Fatalf("PageByID(%d) missing", want.id)
			}
			if page.Name != want.name {
				t.Fatalf("PageByID(%d).Name = %q, want %q", want.id, page.Name, want.name)
			}
		})
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodePages_TagsMatchSpec(t *testing.T) {
	for _, want := range allPages {
		page, ok := PageByID(want.id)
		if !ok {
			t.Fatalf("PageByID(%d) missing", want.id)
		}
		for _, et := range want.tags {
			tok, ok := TokenByTag(want.id, et.name)
			if !ok {
				t.Errorf("page %s: tag %q missing", want.name, et.name)
				continue
			}
			if tok != et.token {
				t.Errorf("page %s tag %s: token = 0x%02X, want 0x%02X", want.name, et.name, tok, et.token)
				continue
			}
			name, ok := TagByToken(want.id, et.token)
			if !ok || name != et.name {
				t.Errorf("page %s token 0x%02X: name = %q (ok=%v), want %q", want.name, et.token, name, ok, et.name)
			}
		}
		_ = page
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodePages_NoDuplicates(t *testing.T) {
	for _, want := range allPages {
		page, ok := PageByID(want.id)
		if !ok {
			t.Fatalf("PageByID(%d) missing", want.id)
		}
		seenName := map[string]bool{}
		seenTok := map[byte]bool{}
		for _, tag := range page.Tags {
			if tag.Token < 0x05 || tag.Token > 0x3F {
				t.Errorf("page %s tag %s: token 0x%02X out of [0x05, 0x3F]", page.Name, tag.Name, tag.Token)
			}
			if seenName[tag.Name] {
				t.Errorf("page %s: duplicate tag name %q", page.Name, tag.Name)
			}
			seenName[tag.Name] = true
			if seenTok[tag.Token] {
				t.Errorf("page %s: duplicate token 0x%02X", page.Name, tag.Token)
			}
			seenTok[tag.Token] = true
		}
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodePages_LookupMissingReturnsFalse(t *testing.T) {
	if _, ok := PageByID(0xFE); ok {
		t.Errorf("PageByID(0xFE) returned ok=true")
	}
	if _, ok := TagByToken(0, 0x00); ok {
		t.Errorf("TagByToken(0, 0x00) returned ok=true")
	}
	if _, ok := TokenByTag(0, "no-such-tag"); ok {
		t.Errorf("TokenByTag(\"no-such-tag\") returned ok=true")
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodepages_TagLookup(t *testing.T) {
	if name, ok := TagByToken(0, 0x05); !ok || name != "Sync" {
		t.Fatalf("TagByToken(0,0x05) = %q,%v", name, ok)
	}
	if _, ok := TagByToken(99, 0x05); ok {
		t.Fatal("expected unknown page miss")
	}
	if tok, ok := TokenByTag(0, "Sync"); !ok || tok != 0x05 {
		t.Fatalf("TokenByTag(0,Sync) = %02X,%v", tok, ok)
	}
	if _, ok := TokenByTag(0, "BogusTag"); ok {
		t.Fatal("expected unknown tag miss")
	}
	if _, ok := TokenByTag(99, "Sync"); ok {
		t.Fatal("expected unknown page miss")
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestPageByID_Unknown(t *testing.T) {
	if _, ok := PageByID(99); ok {
		t.Fatal("expected unknown page miss")
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestPageByName_Unknown(t *testing.T) {
	if _, ok := PageByName("Bogus"); ok {
		t.Fatal("expected unknown page miss")
	}
}

// SPEC: MS-ASWBXML/codepage.invariants
func TestCodePages_AllPageIDsRegistered(t *testing.T) {
	got := AllPageIDs()
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24}
	if len(got) != len(want) {
		t.Fatalf("AllPageIDs len = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllPageIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
