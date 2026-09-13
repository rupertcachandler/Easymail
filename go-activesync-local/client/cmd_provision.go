package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/remdev/go-activesync/eas"
)

// Provision performs the two-pass MS-ASPROV exchange: an initial download
// returning a temporary PolicyKey + EAS provisioning document, followed by
// an Acknowledge call that promotes the temporary PolicyKey to the active
// one. The active PolicyKey is persisted in the configured PolicyStore.
func (c *Client) Provision(ctx context.Context, user string) (*eas.EASProvisionDoc, error) {
	initial := eas.NewInitialRequest()
	c.applyDeviceInformation(&initial)
	var first eas.ProvisionResponse
	if err := c.do(ctx, CmdProvision, user, &initial, &first); err != nil {
		return nil, err
	}
	if first.Status != int32(eas.StatusSuccess) {
		return nil, &StatusError{Command: "Provision", Status: first.Status}
	}
	if len(first.Policies.Policy) != 1 {
		return nil, errors.New("provision: response missing Policy")
	}
	pol := first.Policies.Policy[0]
	if pol.PolicyType != eas.PolicyTypeWBXML {
		return nil, fmt.Errorf("provision: unexpected PolicyType %q", pol.PolicyType)
	}
	if pol.Status != int32(eas.StatusSuccess) {
		return nil, &StatusError{Command: "Provision", Status: pol.Status}
	}
	if pol.PolicyKey == "" {
		return nil, errors.New("provision: response missing temporary PolicyKey")
	}

	if err := c.PolicyStore.Set(ctx, pol.PolicyKey); err != nil {
		return nil, fmt.Errorf("provision: persist temp policy key: %w", err)
	}

	ack := eas.NewAcknowledgeRequest(pol.PolicyKey, int32(eas.StatusSuccess))
	c.applyDeviceInformation(&ack)
	var second eas.ProvisionResponse
	if err := c.do(ctx, CmdProvision, user, &ack, &second); err != nil {
		return nil, err
	}
	if second.Status != int32(eas.StatusSuccess) {
		return nil, &StatusError{Command: "Provision", Status: second.Status}
	}
	if len(second.Policies.Policy) != 1 {
		return nil, errors.New("provision: ack response missing Policy")
	}
	final := second.Policies.Policy[0]
	if final.Status != int32(eas.StatusSuccess) {
		return nil, &StatusError{Command: "Provision", Status: final.Status}
	}
	if final.PolicyKey == "" {
		return nil, errors.New("provision: ack missing PolicyKey")
	}
	if err := c.PolicyStore.Set(ctx, final.PolicyKey); err != nil {
		return nil, fmt.Errorf("provision: persist final policy key: %w", err)
	}
	return pol.Data, nil
}

func (c *Client) applyDeviceInformation(req *eas.ProvisionRequest) {
	set := eas.DeviceInformationSet{
		Model:          c.DeviceModel,
		IMEI:           c.DeviceIMEI,
		FriendlyName:   c.DeviceFriendlyName,
		OS:             c.DeviceOS,
		OSLanguage:     c.DeviceOSLanguage,
		PhoneNumber:    c.PhoneNumber,
		UserAgent:      c.DeviceUserAgent,
		MobileOperator: c.Carrier,
	}
	if set.Model == "" &&
		set.IMEI == "" &&
		set.FriendlyName == "" &&
		set.OS == "" &&
		set.OSLanguage == "" &&
		set.PhoneNumber == "" &&
		set.UserAgent == "" &&
		set.MobileOperator == "" {
		return
	}
	req.DeviceInformation = &eas.DeviceInformation{Set: set}
}
