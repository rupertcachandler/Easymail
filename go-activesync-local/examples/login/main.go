// Command login demonstrates the discovery + Provision exchange against an
// EAS 14.1 server. It resolves the EAS endpoint via Autodiscover, runs the
// two-pass Provision flow, and prints the negotiated PolicyKey.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/remdev/go-activesync/autodiscover"
	"github.com/remdev/go-activesync/client"
)

func main() {
	var (
		email      = flag.String("email", "", "user@example.com")
		password   = flag.String("password", "", "password")
		deviceID   = flag.String("device-id", "go-activesync-example", "stable device id")
		deviceType = flag.String("device-type", "SmartPhone", "device type token")
	)
	flag.Parse()
	if *email == "" || *password == "" {
		log.Fatalf("usage: login -email user@example.com -password ****")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d := autodiscover.New(http.DefaultClient)
	ad, err := d.Discover(ctx, *email, &autodiscover.Credentials{Username: *email, Password: *password})
	if err != nil {
		log.Fatalf("autodiscover: %v", err)
	}
	fmt.Printf("EAS endpoint: %s\n", ad.URL)

	c, err := client.New(client.Config{
		BaseURL:    ad.URL,
		Auth:       &client.BasicAuth{Username: *email, Password: *password},
		DeviceID:   *deviceID,
		DeviceType: *deviceType,
		UserAgent:  "go-activesync-login/0.1",
	})
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	doc, err := c.Provision(ctx, *email)
	if err != nil {
		log.Fatalf("provision: %v", err)
	}
	pk, _ := c.PolicyStore.Get(ctx)
	fmt.Printf("active PolicyKey: %s\n", pk)
	if doc != nil {
		fmt.Printf("MinDevicePasswordLength=%d MaxInactivityTimeDeviceLock=%d\n",
			doc.MinDevicePasswordLength, doc.MaxInactivityTimeDeviceLock)
	}
}
