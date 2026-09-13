// Command calendar-sync runs an initial calendar Sync.
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

const calendarFolderType = 8

func main() {
	var (
		email      = flag.String("email", "", "user@example.com")
		password   = flag.String("password", "", "password")
		deviceID   = flag.String("device-id", "go-activesync-example", "stable device id")
		deviceType = flag.String("device-type", "SmartPhone", "device type token")
	)
	flag.Parse()
	if *email == "" || *password == "" {
		log.Fatalf("usage: calendar-sync -email user@example.com -password ****")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
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
		UserAgent:  "go-activesync-calendar-sync/0.1",
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
	var calID string
	for _, f := range folderResp.Changes.Add {
		if f.Type == calendarFolderType {
			calID = f.ServerID
			break
		}
	}
	if calID == "" {
		log.Fatalf("calendar folder not found")
	}

	initial, err := c.Sync(ctx, *email, &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{SyncKey: "0", CollectionID: calID}},
		},
	})
	if err != nil {
		log.Fatalf("sync initial: %v", err)
	}
	resp, err := c.Sync(ctx, *email, &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{
				SyncKey:      initial.Collections.Collection[0].SyncKey,
				CollectionID: calID,
				GetChanges:   1,
				WindowSize:   25,
			}},
		},
	})
	if err != nil {
		log.Fatalf("sync content: %v", err)
	}
	for _, col := range resp.Collections.Collection {
		fmt.Printf("calendar %s status=%d\n", col.CollectionID, col.Status)
		if col.Commands != nil {
			fmt.Printf("  %d adds\n", len(col.Commands.Add))
		}
	}
}
