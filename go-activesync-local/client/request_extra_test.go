package client

import (
	"encoding/base64"
	"strings"
	"testing"
)

// SPEC: MS-ASHTTP/query.base64.layout
func TestQueryBase64_PolicyKey(t *testing.T) {
	pk := uint32(0x11223344)
	q := Query{
		ProtocolVersion: 0x91,
		Cmd:             CmdSync,
		Locale:          0x0409,
		DeviceID:        "d",
		DeviceType:      "t",
		PolicyKey:       &pk,
	}
	enc, err := q.EncodeBase64()
	if err != nil {
		t.Fatalf("EncodeBase64: %v", err)
	}
	got, err := ParseBase64(enc)
	if err != nil {
		t.Fatalf("ParseBase64: %v", err)
	}
	if got.PolicyKey == nil || *got.PolicyKey != pk {
		t.Fatalf("PolicyKey = %v", got.PolicyKey)
	}
}

// SPEC: MS-ASHTTP/query.base64.layout
func TestQueryBase64_DeviceIDTooLong(t *testing.T) {
	q := Query{DeviceID: strings.Repeat("a", 256)}
	if _, err := q.EncodeBase64(); err == nil {
		t.Fatal("expected error")
	}
}

// SPEC: MS-ASHTTP/query.base64.layout
func TestQueryBase64_DeviceTypeTooLong(t *testing.T) {
	q := Query{DeviceID: "a", DeviceType: strings.Repeat("b", 256)}
	if _, err := q.EncodeBase64(); err == nil {
		t.Fatal("expected error")
	}
}

// SPEC: MS-ASHTTP/query.base64.layout
func TestQueryBase64_ParamValueTooLong(t *testing.T) {
	q := Query{DeviceID: "a", DeviceType: "b", Params: []QueryParam{{Tag: 1, Value: make([]byte, 256)}}}
	if _, err := q.EncodeBase64(); err == nil {
		t.Fatal("expected error")
	}
}

// SPEC: MS-ASHTTP/query.base64.example
func TestParseBase64_Errors(t *testing.T) {
	cases := map[string][]byte{
		"too short":           {0x91},
		"device id truncated": {0x91, 0, 0, 0, 5, 'a'},
		"missing pk":          {0x91, 0, 0, 0, 1, 'a'},
		"pk truncated":        {0x91, 0, 0, 0, 1, 'a', 4, 1, 2},
		"missing dt":          {0x91, 0, 0, 0, 1, 'a', 0},
		"dt truncated":        {0x91, 0, 0, 0, 1, 'a', 0, 5, 't'},
		"bad pk len":          {0x91, 0, 0, 0, 1, 'a', 3, 0, 0, 0, 0, 1, 't'},
		"param hdr trunc":     {0x91, 0, 0, 0, 1, 'a', 0, 1, 't', 8},
		"param val trunc":     {0x91, 0, 0, 0, 1, 'a', 0, 1, 't', 8, 5, 'a'},
	}
	for name, raw := range cases {
		enc := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(raw)
		if _, err := ParseBase64(enc); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := ParseBase64("!!!"); err == nil {
		t.Errorf("bad base64 should error")
	}
}

// SPEC: MS-ASHTTP/query.command-codes
func TestCommandName_Coverage(t *testing.T) {
	codes := []byte{
		CmdSync, CmdSendMail, CmdSmartForward, CmdSmartReply, CmdGetAttachment,
		CmdFolderSync, CmdFolderCreate, CmdFolderDelete, CmdFolderUpdate,
		CmdMoveItems, CmdGetItemEstimate, CmdMeetingResponse, CmdSearch,
		CmdSettings, CmdPing, CmdItemOperations, CmdProvision,
		CmdResolveRecipients, CmdValidateCert, 0xFE,
	}
	for _, c := range codes {
		if commandName(c) == "" {
			t.Errorf("commandName(%d) empty", c)
		}
	}
}

// SPEC: MS-ASHTTP/query.plain
func TestQueryPlain_AllParamTags(t *testing.T) {
	q := Query{
		ProtocolVersion: 0x91,
		Cmd:             CmdSync,
		DeviceID:        "id",
		DeviceType:      "PC",
		Params: []QueryParam{
			{Tag: ParamUser, Value: []byte("u")},
			{Tag: ParamCollectionID, Value: []byte("c")},
			{Tag: ParamCollectionName, Value: []byte("cn")},
			{Tag: ParamItemID, Value: []byte("i")},
			{Tag: ParamLongID, Value: []byte("l")},
			{Tag: ParamParentID, Value: []byte("p")},
			{Tag: ParamOccurrence, Value: []byte("o")},
			{Tag: ParamOptions, Value: []byte("opt")},
			{Tag: ParamSaveInSent, Value: []byte("s")},
			{Tag: ParamAttachmentName, Value: []byte("a")},
			{Tag: ParamAcceptMultipart, Value: []byte("am")},
			{Tag: 0xFE, Value: []byte("ignored")},
		},
	}
	got := q.EncodePlain()
	if wantPrefix := "Cmd=Sync&User=u&DeviceId=id&DeviceType=PC"; !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("plain query prefix = %q, want prefix %q", got, wantPrefix)
	}
	for _, want := range []string{"User=u", "CollectionId=c", "CollectionName=cn", "ItemId=i",
		"LongId=l", "ParentId=p", "Occurrence=o", "Options=opt", "SaveInSent=s",
		"AttachmentName=a", "AcceptMultiPart=am"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// SPEC: MS-ASHTTP/request.path
func TestBuildURL(t *testing.T) {
	cases := []struct {
		base    string
		query   string
		plain   bool
		want    string
		wantErr bool
	}{
		{"https://eas/example", "abc", false, "https://eas/example?abc", false},
		{"https://eas", "x=1", true, "https://eas/Microsoft-Server-ActiveSync?x=1", false},
		{"https://eas/", "abc", false, "https://eas/Microsoft-Server-ActiveSync?abc", false},
		{"://bad", "", false, "", true},
	}
	for _, c := range cases {
		got, err := BuildURL(c.base, c.query, c.plain)
		if c.wantErr {
			if err == nil {
				t.Errorf("BuildURL(%q): expected err", c.base)
			}
			continue
		}
		if err != nil {
			t.Errorf("BuildURL(%q): %v", c.base, err)
			continue
		}
		if got != c.want {
			t.Errorf("BuildURL(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}
