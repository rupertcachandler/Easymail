package eas

// PolicyTypeWBXML is the EAS 14.1 policy type identifier
// (MS-ASPROV §2.2.2.39).
const PolicyTypeWBXML = "MS-EAS-Provisioning-WBXML"

// ProvisionRequest is the client-to-server Provision payload. The same WBXML
// element shape is used both for the initial download request and for the
// acknowledgement step (MS-ASPROV §3.2.5.1).
type ProvisionRequest struct {
	XMLName           struct{}           `wbxml:"Provision.Provision"`
	DeviceInformation *DeviceInformation `wbxml:"Settings.DeviceInformation,omitempty"`
	Policies          PoliciesRequest    `wbxml:"Provision.Policies"`
}

// DeviceInformation carries the Settings:DeviceInformation payload some
// Exchange policies require during Provision (status 165).
type DeviceInformation struct {
	Set DeviceInformationSet `wbxml:"Settings.Set"`
}

// DeviceInformationSet is the Settings:Set content nested inside
// DeviceInformation.
type DeviceInformationSet struct {
	Model          string `wbxml:"Settings.Model,omitempty"`
	IMEI           string `wbxml:"Settings.IMEI,omitempty"`
	FriendlyName   string `wbxml:"Settings.FriendlyName,omitempty"`
	OS             string `wbxml:"Settings.OS,omitempty"`
	OSLanguage     string `wbxml:"Settings.OSLanguage,omitempty"`
	PhoneNumber    string `wbxml:"Settings.PhoneNumber,omitempty"`
	UserAgent      string `wbxml:"Settings.UserAgent,omitempty"`
	MobileOperator string `wbxml:"Settings.MobileOperator,omitempty"`
}

// PoliciesRequest wraps the Policy entries inside a ProvisionRequest.
type PoliciesRequest struct {
	Policy []PolicyRequest `wbxml:"Provision.Policy"`
}

// PolicyRequest is a single client-side Policy entry.
type PolicyRequest struct {
	PolicyType string `wbxml:"Provision.PolicyType"`
	PolicyKey  string `wbxml:"Provision.PolicyKey,omitempty"`
	Status     int32  `wbxml:"Provision.Status,omitempty"`
}

// NewInitialRequest builds the initial Provision download request asking the
// server for the EAS provisioning document.
func NewInitialRequest() ProvisionRequest {
	return ProvisionRequest{
		Policies: PoliciesRequest{
			Policy: []PolicyRequest{{PolicyType: PolicyTypeWBXML}},
		},
	}
}

// NewAcknowledgeRequest builds the Provision acknowledge request that echoes
// the temporary PolicyKey along with an AcknowledgeStatus (MS-ASPROV
// §2.2.2.53).
func NewAcknowledgeRequest(policyKey string, status int32) ProvisionRequest {
	return ProvisionRequest{
		Policies: PoliciesRequest{
			Policy: []PolicyRequest{{
				PolicyType: PolicyTypeWBXML,
				PolicyKey:  policyKey,
				Status:     status,
			}},
		},
	}
}

// ProvisionResponse is the server-to-client Provision payload.
type ProvisionResponse struct {
	XMLName  struct{}         `wbxml:"Provision.Provision"`
	Status   int32            `wbxml:"Provision.Status,omitempty"`
	Policies PoliciesResponse `wbxml:"Provision.Policies"`
}

// PoliciesResponse wraps the Policy entries inside a ProvisionResponse.
type PoliciesResponse struct {
	Policy []PolicyResponse `wbxml:"Provision.Policy"`
}

// PolicyResponse is a single server-side Policy entry.
type PolicyResponse struct {
	PolicyType string           `wbxml:"Provision.PolicyType"`
	PolicyKey  string           `wbxml:"Provision.PolicyKey,omitempty"`
	Status     int32            `wbxml:"Provision.Status,omitempty"`
	Data       *EASProvisionDoc `wbxml:"Provision.Data,omitempty"`
}

// EASProvisionDoc carries the EAS 14.1 device-policy document.
type EASProvisionDoc struct {
	DevicePasswordEnabled                    int32  `wbxml:"Provision.DevicePasswordEnabled"`
	AlphanumericDevicePasswordRequired       int32  `wbxml:"Provision.AlphanumericDevicePasswordRequired"`
	PasswordRecoveryEnabled                  int32  `wbxml:"Provision.PasswordRecoveryEnabled,omitempty"`
	AttachmentsEnabled                       int32  `wbxml:"Provision.AttachmentsEnabled,omitempty"`
	MinDevicePasswordLength                  int32  `wbxml:"Provision.MinDevicePasswordLength"`
	MaxInactivityTimeDeviceLock              int32  `wbxml:"Provision.MaxInactivityTimeDeviceLock"`
	MaxDevicePasswordFailedAttempts          int32  `wbxml:"Provision.MaxDevicePasswordFailedAttempts"`
	MaxAttachmentSize                        int64  `wbxml:"Provision.MaxAttachmentSize,omitempty"`
	AllowSimpleDevicePassword                int32  `wbxml:"Provision.AllowSimpleDevicePassword"`
	DevicePasswordExpiration                 int32  `wbxml:"Provision.DevicePasswordExpiration,omitempty"`
	DevicePasswordHistory                    int32  `wbxml:"Provision.DevicePasswordHistory,omitempty"`
	AllowStorageCard                         int32  `wbxml:"Provision.AllowStorageCard"`
	AllowCamera                              int32  `wbxml:"Provision.AllowCamera"`
	RequireDeviceEncryption                  int32  `wbxml:"Provision.RequireDeviceEncryption"`
	AllowUnsignedApplications                int32  `wbxml:"Provision.AllowUnsignedApplications,omitempty"`
	AllowUnsignedInstallationPackages        int32  `wbxml:"Provision.AllowUnsignedInstallationPackages,omitempty"`
	MinDevicePasswordComplexCharacters       int32  `wbxml:"Provision.MinDevicePasswordComplexCharacters,omitempty"`
	AllowWiFi                                int32  `wbxml:"Provision.AllowWiFi,omitempty"`
	AllowTextMessaging                       int32  `wbxml:"Provision.AllowTextMessaging,omitempty"`
	AllowPOPIMAPEmail                        int32  `wbxml:"Provision.AllowPOPIMAPEmail,omitempty"`
	AllowBluetooth                           int32  `wbxml:"Provision.AllowBluetooth,omitempty"`
	AllowIrDA                                int32  `wbxml:"Provision.AllowIrDA,omitempty"`
	RequireManualSyncWhenRoaming             int32  `wbxml:"Provision.RequireManualSyncWhenRoaming,omitempty"`
	AllowDesktopSync                         int32  `wbxml:"Provision.AllowDesktopSync,omitempty"`
	MaxCalendarAgeFilter                     int32  `wbxml:"Provision.MaxCalendarAgeFilter,omitempty"`
	AllowHTMLEmail                           int32  `wbxml:"Provision.AllowHTMLEmail,omitempty"`
	MaxEmailAgeFilter                        int32  `wbxml:"Provision.MaxEmailAgeFilter,omitempty"`
	MaxEmailBodyTruncationSize               int32  `wbxml:"Provision.MaxEmailBodyTruncationSize,omitempty"`
	MaxEmailHTMLBodyTruncationSize           int32  `wbxml:"Provision.MaxEmailHTMLBodyTruncationSize,omitempty"`
	RequireSignedSMIMEMessages               int32  `wbxml:"Provision.RequireSignedSMIMEMessages,omitempty"`
	RequireEncryptedSMIMEMessages            int32  `wbxml:"Provision.RequireEncryptedSMIMEMessages,omitempty"`
	RequireSignedSMIMEAlgorithm              int32  `wbxml:"Provision.RequireSignedSMIMEAlgorithm,omitempty"`
	RequireEncryptionSMIMEAlgorithm          int32  `wbxml:"Provision.RequireEncryptionSMIMEAlgorithm,omitempty"`
	AllowSMIMEEncryptionAlgorithmNegotiation int32  `wbxml:"Provision.AllowSMIMEEncryptionAlgorithmNegotiation,omitempty"`
	AllowSMIMESoftCerts                      int32  `wbxml:"Provision.AllowSMIMESoftCerts,omitempty"`
	AllowBrowser                             int32  `wbxml:"Provision.AllowBrowser,omitempty"`
	AllowConsumerEmail                       int32  `wbxml:"Provision.AllowConsumerEmail,omitempty"`
	AllowRemoteDesktop                       int32  `wbxml:"Provision.AllowRemoteDesktop,omitempty"`
	AllowInternetSharing                     int32  `wbxml:"Provision.AllowInternetSharing,omitempty"`
	UnapprovedInROMApplicationList           string `wbxml:"Provision.UnapprovedInROMApplicationList,omitempty"`
	ApprovedApplicationList                  string `wbxml:"Provision.ApprovedApplicationList,omitempty"`
}
