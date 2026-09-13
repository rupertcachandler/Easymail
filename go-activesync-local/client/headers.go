package client

import "net/http"

// ContentTypeWBXML is the MIME type EAS uses for WBXML payloads.
const ContentTypeWBXML = "application/vnd.ms-sync.wbxml"

// HeaderOptions describes the values used to populate the mandatory EAS
// headers on a request.
type HeaderOptions struct {
	ProtocolVersion string // e.g. "14.1"
	UserAgent       string // identifies the client
	PolicyKey       string // current policy key, "0" before initial Provision
	AcceptLanguage  string // optional, e.g. "en-US"
}

// DeviceHeaderOptions describes optional first-class device profile headers.
type DeviceHeaderOptions struct {
	DeviceModel      string
	DeviceOS         string
	DeviceOSLanguage string
	Carrier          string
	PhoneNumber      string
	DeviceUserAgent  string
}

// ApplyMandatoryHeaders sets the mandatory MS-ASHTTP headers on h. Any
// existing values for managed headers are overwritten so the contract is
// deterministic for upstream code.
func ApplyMandatoryHeaders(h http.Header, opts HeaderOptions) {
	h.Set("MS-ASProtocolVersion", opts.ProtocolVersion)
	h.Set("Content-Type", ContentTypeWBXML)
	h.Set("Accept", ContentTypeWBXML)
	h.Set("User-Agent", opts.UserAgent)
	if opts.PolicyKey != "" {
		h.Set("X-MS-PolicyKey", opts.PolicyKey)
	}
	if opts.AcceptLanguage != "" {
		h.Set("Accept-Language", opts.AcceptLanguage)
	}
}

// ApplyDeviceProfileHeaders sets optional device profile headers when present.
func ApplyDeviceProfileHeaders(h http.Header, opts DeviceHeaderOptions) {
	if opts.DeviceModel != "" {
		h.Set("X-MS-DeviceModel", opts.DeviceModel)
	}
	if opts.DeviceOS != "" {
		h.Set("X-MS-DeviceOS", opts.DeviceOS)
	}
	if opts.DeviceOSLanguage != "" {
		h.Set("X-MS-DeviceOSLanguage", opts.DeviceOSLanguage)
	}
	if opts.Carrier != "" {
		h.Set("X-MS-DeviceCarrier", opts.Carrier)
	}
	if opts.PhoneNumber != "" {
		h.Set("X-MS-DevicePhoneNumber", opts.PhoneNumber)
	}
	if opts.DeviceUserAgent != "" {
		h.Set("X-MS-DeviceUserAgent", opts.DeviceUserAgent)
	}
}

// mergeExtraHeaders merges src into dst for integrator-specific headers. Each
// header name is normalized with http.CanonicalHeaderKey. If dst already
// contains any value for that name, the entire key is skipped so mandatory
// client headers cannot be overwritten. Otherwise every value from src for
// that key is added with Add (preserving duplicates from src).
func mergeExtraHeaders(dst, src http.Header) {
	if len(src) == 0 {
		return
	}
	grouped := make(map[string][]string)
	for k, vals := range src {
		if k == "" {
			continue
		}
		ck := http.CanonicalHeaderKey(k)
		grouped[ck] = append(grouped[ck], vals...)
	}
	for ck, vals := range grouped {
		if len(dst.Values(ck)) > 0 {
			continue
		}
		for _, v := range vals {
			dst.Add(ck, v)
		}
	}
}
