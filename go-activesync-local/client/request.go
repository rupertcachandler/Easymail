// Package client provides the high-level Exchange ActiveSync client used to
// talk to a 14.1 server. It wires the WBXML codec, MS-ASHTTP transport,
// authentication, policy and sync-state stores into typed command methods.
package client

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// EndpointPath is the request-URI path mandated by MS-ASHTTP §2.2.1.
const EndpointPath = "/Microsoft-Server-ActiveSync"

// Command codes (MS-ASHTTP §2.2.1.1.1.1.2).
const (
	CmdSync              byte = 0
	CmdSendMail          byte = 1
	CmdSmartForward      byte = 2
	CmdSmartReply        byte = 3
	CmdGetAttachment     byte = 4
	CmdFolderSync        byte = 9
	CmdFolderCreate      byte = 10
	CmdFolderDelete      byte = 11
	CmdFolderUpdate      byte = 12
	CmdMoveItems         byte = 13
	CmdGetItemEstimate   byte = 14
	CmdMeetingResponse   byte = 15
	CmdSearch            byte = 16
	CmdSettings          byte = 17
	CmdPing              byte = 18
	CmdItemOperations    byte = 19
	CmdProvision         byte = 20
	CmdResolveRecipients byte = 21
	CmdValidateCert      byte = 22
)

// Command-specific parameter tags (MS-ASHTTP §2.2.1.1.1.1.3).
const (
	ParamAttachmentName  byte = 0
	ParamCollectionID    byte = 1
	ParamCollectionName  byte = 2
	ParamItemID          byte = 3
	ParamLongID          byte = 4
	ParamParentID        byte = 5
	ParamOccurrence      byte = 6
	ParamOptions         byte = 7
	ParamUser            byte = 8
	ParamSaveInSent      byte = 9
	ParamAcceptMultipart byte = 10
)

// QueryParam is one entry in the optional command-specific parameter list.
type QueryParam struct {
	Tag   byte
	Value []byte
}

// QueryEncoding selects the MS-ASHTTP request query representation.
type QueryEncoding string

const (
	// QueryEncodingBase64 uses the compact binary query encoded as URL-safe base64.
	QueryEncodingBase64 QueryEncoding = "base64"
	// QueryEncodingPlain uses Cmd=...&User=...&DeviceId=...&DeviceType=... URI parameters.
	QueryEncodingPlain QueryEncoding = "plain"
)

// Query is the abstract representation of the MS-ASHTTP request query, used
// in both base64 (binary) and plain (URL key=value) encodings.
type Query struct {
	ProtocolVersion byte   // 0x91 for 14.1
	Cmd             byte   // command code
	Locale          uint16 // e.g. 0x0409 for en-US
	DeviceID        string
	DeviceType      string
	PolicyKey       *uint32 // nil means absent (length 0 byte in the wire form)
	Params          []QueryParam
}

// EncodeBase64 produces the URL-safe base64 form of the binary query. The
// transport layer prepends the produced string to the request URI.
func (q Query) EncodeBase64() (string, error) {
	if len(q.DeviceID) > 0xFF {
		return "", errors.New("ashttp: DeviceID longer than 255 bytes")
	}
	if len(q.DeviceType) > 0xFF {
		return "", errors.New("ashttp: DeviceType longer than 255 bytes")
	}
	buf := make([]byte, 0, 32+len(q.DeviceID)+len(q.DeviceType))
	buf = append(buf, q.ProtocolVersion, q.Cmd, byte(q.Locale&0xFF), byte(q.Locale>>8))
	buf = append(buf, byte(len(q.DeviceID)))
	buf = append(buf, q.DeviceID...)
	if q.PolicyKey == nil {
		buf = append(buf, 0)
	} else {
		k := *q.PolicyKey
		buf = append(buf, 4, byte(k), byte(k>>8), byte(k>>16), byte(k>>24))
	}
	buf = append(buf, byte(len(q.DeviceType)))
	buf = append(buf, q.DeviceType...)
	for _, p := range q.Params {
		if len(p.Value) > 0xFF {
			return "", fmt.Errorf("ashttp: parameter tag 0x%02X value longer than 255 bytes", p.Tag)
		}
		buf = append(buf, p.Tag, byte(len(p.Value)))
		buf = append(buf, p.Value...)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(buf), nil
}

// ParseBase64 decodes the URL-safe base64 form of the binary query.
func ParseBase64(s string) (Query, error) {
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		// Try standard padding form too, since some servers tolerate it.
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return Query{}, fmt.Errorf("ashttp: base64 decode: %w", err)
		}
	}
	if len(raw) < 5 {
		return Query{}, errors.New("ashttp: query shorter than fixed prefix")
	}
	q := Query{
		ProtocolVersion: raw[0],
		Cmd:             raw[1],
		Locale:          uint16(raw[2]) | uint16(raw[3])<<8,
	}
	i := 4
	devLen := int(raw[i])
	i++
	if i+devLen > len(raw) {
		return Query{}, errors.New("ashttp: DeviceID length out of range")
	}
	q.DeviceID = string(raw[i : i+devLen])
	i += devLen
	if i >= len(raw) {
		return Query{}, errors.New("ashttp: missing PolicyKey length")
	}
	pkLen := int(raw[i])
	i++
	switch pkLen {
	case 0:
	case 4:
		if i+4 > len(raw) {
			return Query{}, errors.New("ashttp: PolicyKey truncated")
		}
		k := uint32(raw[i]) | uint32(raw[i+1])<<8 | uint32(raw[i+2])<<16 | uint32(raw[i+3])<<24
		q.PolicyKey = &k
		i += 4
	default:
		return Query{}, fmt.Errorf("ashttp: PolicyKey length %d invalid", pkLen)
	}
	if i >= len(raw) {
		return Query{}, errors.New("ashttp: missing DeviceType length")
	}
	dtLen := int(raw[i])
	i++
	if i+dtLen > len(raw) {
		return Query{}, errors.New("ashttp: DeviceType truncated")
	}
	q.DeviceType = string(raw[i : i+dtLen])
	i += dtLen

	for i < len(raw) {
		if i+1 >= len(raw) {
			return Query{}, errors.New("ashttp: parameter header truncated")
		}
		tag := raw[i]
		ln := int(raw[i+1])
		i += 2
		if i+ln > len(raw) {
			return Query{}, errors.New("ashttp: parameter value truncated")
		}
		val := make([]byte, ln)
		copy(val, raw[i:i+ln])
		i += ln
		q.Params = append(q.Params, QueryParam{Tag: tag, Value: val})
	}
	return q, nil
}

// EncodePlain returns the URL-encoded plain query (Cmd=Foo&User=...&...).
func (q Query) EncodePlain() string {
	type pair struct {
		name  string
		value string
	}
	user := ""
	params := make([]pair, 0, len(q.Params))
	for _, p := range q.Params {
		switch p.Tag {
		case ParamUser:
			user = string(p.Value)
		case ParamCollectionID:
			params = append(params, pair{"CollectionId", string(p.Value)})
		case ParamCollectionName:
			params = append(params, pair{"CollectionName", string(p.Value)})
		case ParamItemID:
			params = append(params, pair{"ItemId", string(p.Value)})
		case ParamLongID:
			params = append(params, pair{"LongId", string(p.Value)})
		case ParamParentID:
			params = append(params, pair{"ParentId", string(p.Value)})
		case ParamOccurrence:
			params = append(params, pair{"Occurrence", string(p.Value)})
		case ParamOptions:
			params = append(params, pair{"Options", string(p.Value)})
		case ParamSaveInSent:
			params = append(params, pair{"SaveInSent", string(p.Value)})
		case ParamAttachmentName:
			params = append(params, pair{"AttachmentName", string(p.Value)})
		case ParamAcceptMultipart:
			params = append(params, pair{"AcceptMultiPart", string(p.Value)})
		}
	}

	parts := make([]string, 0, 4+len(params))
	add := func(name, value string) {
		parts = append(parts, url.QueryEscape(name)+"="+url.QueryEscape(value))
	}
	add("Cmd", commandName(q.Cmd))
	if user != "" {
		add("User", user)
	}
	add("DeviceId", q.DeviceID)
	add("DeviceType", q.DeviceType)
	for _, p := range params {
		add(p.name, p.value)
	}
	return strings.Join(parts, "&")
}

func commandName(code byte) string {
	switch code {
	case CmdSync:
		return "Sync"
	case CmdSendMail:
		return "SendMail"
	case CmdSmartForward:
		return "SmartForward"
	case CmdSmartReply:
		return "SmartReply"
	case CmdGetAttachment:
		return "GetAttachment"
	case CmdFolderSync:
		return "FolderSync"
	case CmdFolderCreate:
		return "FolderCreate"
	case CmdFolderDelete:
		return "FolderDelete"
	case CmdFolderUpdate:
		return "FolderUpdate"
	case CmdMoveItems:
		return "MoveItems"
	case CmdGetItemEstimate:
		return "GetItemEstimate"
	case CmdMeetingResponse:
		return "MeetingResponse"
	case CmdSearch:
		return "Search"
	case CmdSettings:
		return "Settings"
	case CmdPing:
		return "Ping"
	case CmdItemOperations:
		return "ItemOperations"
	case CmdProvision:
		return "Provision"
	case CmdResolveRecipients:
		return "ResolveRecipients"
	case CmdValidateCert:
		return "ValidateCert"
	default:
		return fmt.Sprintf("Cmd%d", code)
	}
}

// BuildURL composes the full request URL by appending the encoded query
// string to base. The base argument must already point at the EAS endpoint
// host and path; trailing slashes on path are tolerated.
func BuildURL(base string, encodedQuery string, plain bool) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = EndpointPath
	}
	// Both transports place the encoded query (plain key=value pairs or the
	// base64 binary blob) into RawQuery verbatim. The plain flag is part of
	// the public signature for forward compatibility.
	_ = plain
	u.RawQuery = encodedQuery
	out := strings.TrimSuffix(u.String(), "?")
	return out, nil
}
