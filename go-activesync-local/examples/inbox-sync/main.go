// Command inbox-sync demonstrates an initial Sync against the inbox folder.
// It resolves the endpoint, provisions the device, runs FolderSync to find
// the inbox CollectionId, then issues an initial Sync (SyncKey=0) followed
// by a content Sync (SyncKey=<received>) and prints any new e-mails.
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
		deviceID   = flag.String("device-id", "go-activesync-example", "stable device id")
		deviceType = flag.String("device-type", "SmartPhone", "device type token")
	)
	flag.Parse()
	if *email == "" || *password == "" {
		log.Fatalf("usage: inbox-sync -email user@example.com -password ****")
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
		UserAgent:  "go-activesync-inbox-sync/0.1",
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
		log.Fatalf("inbox folder not found in FolderSync result")
	}

	initial, err := c.Sync(ctx, *email, &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{
				SyncKey:      "0",
				CollectionID: inboxID,
			}},
		},
	})
	if err != nil {
		log.Fatalf("sync (initial): %v", err)
	}
	syncKey := initial.Collections.Collection[0].SyncKey

	resp, err := c.Sync(ctx, *email, &eas.SyncRequest{
		Collections: eas.SyncCollections{
			Collection: []eas.SyncCollection{{
				SyncKey:      syncKey,
				CollectionID: inboxID,
				GetChanges:   1,
				WindowSize:   25,
			}},
		},
	})
	if err != nil {
		log.Fatalf("sync (content): %v", err)
	}

	for _, col := range resp.Collections.Collection {
		if col.Commands == nil {
			fmt.Printf("collection %s: no new items\n", col.CollectionID)
			continue
		}
		fmt.Printf("collection %s: %d adds\n", col.CollectionID, len(col.Commands.Add))
	}
}
