// Command ping demonstrates a long-poll Ping that waits for changes on the
// inbox folder and exits as soon as the server signals Status=2 or the
// context deadline expires.
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
	"github.com/remdev/go-activesync/eas"
)

const inboxFolderType = 2

func main() {
	var (
		email      = flag.String("email", "", "user@example.com")
		password   = flag.String("password", "", "password")
		heartbeat  = flag.Int("heartbeat", 480, "heartbeat interval in seconds (60..3540)")
		timeout    = flag.Duration("timeout", 10*time.Minute, "overall context timeout")
		deviceID   = flag.String("device-id", "go-activesync-example", "stable device id")
		deviceType = flag.String("device-type", "SmartPhone", "device type token")
	)
	flag.Parse()
	if *email == "" || *password == "" {
		log.Fatalf("usage: ping -email user@example.com -password ****")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	d := autodiscover.New(http.DefaultClient)
	ad, err := d.Discover(ctx, *email, &autodiscover.Credentials{Username: *email, Password: *password})
	if err != nil {
		log.Fatalf("autodiscover: %v", err)
	}
	c, err := client.New(client.Config{
		BaseURL:    ad.URL,
		Auth:       &client.BasicAuth{Username: *email, Password: *password},
		DeviceID:   *deviceID,
		DeviceType: *deviceType,
		UserAgent:  "go-activesync-ping/0.1",
	})
	if err != nil {
		log.Fatalf("client: %v", err)
	}
	if _, err := c.Provision(ctx, *email); err != nil {
		log.Fatalf("provision: %v", err)
	}
	folderResp, err := c.FolderSync(ctx, *email, "0")
	if err != nil {
		log.Fatalf("foldersync: %v", err)
	}
	var inboxID string
	for _, f := range folderResp.Changes.Add {
		if f.Type == inboxFolderType {
			inboxID = f.ServerID
			break
		}
	}
	if inboxID == "" {
		log.Fatalf("inbox folder not found")
	}

	resp, err := c.Ping(ctx, *email, &eas.PingRequest{
		HeartbeatInterval: int32(*heartbeat),
		Folders: eas.PingFolders{
			Folder: []eas.PingFolder{{ID: inboxID, Class: "Email"}},
		},
	})
	if err != nil {
		log.Fatalf("ping: %v", err)
	}
	if eas.PingHasChanges(resp.Status) {
		fmt.Printf("changes available in %v\n", resp.Folders.Folder)
	} else {
		fmt.Printf("no changes (status=%d)\n", resp.Status)
	}
}
