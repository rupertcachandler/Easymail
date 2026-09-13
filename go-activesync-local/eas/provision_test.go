package eas

import (
	"reflect"
	"testing"

	"github.com/remdev/go-activesync/wbxml"
)

// SPEC: MS-ASPROV/policy-type
func TestPolicyType_Constant(t *testing.T) {
	if PolicyTypeWBXML != "MS-EAS-Provisioning-WBXML" {
		t.Fatalf("PolicyTypeWBXML = %q", PolicyTypeWBXML)
	}
}

// SPEC: MS-ASPROV/two-phase-flow
func TestProvision_AcknowledgeRequest(t *testing.T) {
	req := NewAcknowledgeRequest("123456789", 1)
	if len(req.Policies.Policy) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(req.Policies.Policy))
	}
	p := req.Policies.Policy[0]
	if p.PolicyType != PolicyTypeWBXML {
		t.Errorf("PolicyType = %q", p.PolicyType)
	}
	if p.PolicyKey != "123456789" {
		t.Errorf("PolicyKey = %q", p.PolicyKey)
	}
	if p.Status != int32(1) {
		t.Errorf("Status = %d", p.Status)
	}
}

// SPEC: MS-ASPROV/policydoc.fields
func TestProvision_RoundTrip(t *testing.T) {
	in := ProvisionResponse{
		Status: 1,
		Policies: PoliciesResponse{
			Policy: []PolicyResponse{{
				PolicyType: PolicyTypeWBXML,
				PolicyKey:  "123",
				Status:     1,
				Data: &EASProvisionDoc{
					DevicePasswordEnabled:              1,
					AlphanumericDevicePasswordRequired: 0,
					MinDevicePasswordLength:            4,
					MaxInactivityTimeDeviceLock:        900,
					MaxDevicePasswordFailedAttempts:    8,
					AllowSimpleDevicePassword:          1,
					RequireDeviceEncryption:            0,
					AllowStorageCard:                   1,
					AllowCamera:                        1,
				},
			}},
		},
	}
	data, err := wbxml.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ProvisionResponse
	if err := wbxml.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out.XMLName = in.XMLName
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
