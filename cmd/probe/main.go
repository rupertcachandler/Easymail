package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"easymail/internal/activesync"
	"easymail/internal/models"
)

func main() {
	filter := int32(0)
	if len(os.Args) >= 2 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			filter = int32(v)
		}
	}
	db, err := sql.Open("sqlite3", "/home/rupert/.config/boi/boi.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	acc := &models.Account{}
	err = db.QueryRow(`SELECT id, name, email, password, server_url, device_id, device_type FROM accounts LIMIT 1`).
		Scan(&acc.ID, &acc.Name, &acc.Email, &acc.Password, &acc.ServerURL, &acc.DeviceID, &acc.DeviceType)
	if err != nil {
		log.Fatal("account: ", err)
	}
	fmt.Printf("account: %s server: %s device: %s\n", acc.Email, acc.ServerURL, acc.DeviceID)

	client, err := activesync.NewClient(acc)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		log.Fatal("connect: ", err)
	}

	for _, fid := range os.Args[2:] {
		start := time.Now()
		emails, err := client.SyncEmailsFilter(ctx, fid, filter)
		el := time.Since(start).Round(time.Millisecond)
		if err != nil {
			fmt.Printf("folder %s: ERROR %v (after %v)\n", fid, err, el)
			continue
		}
		fmt.Printf("folder %s: %d emails in %v\n", fid, len(emails), el)
		if len(emails) > 0 {
			newest := emails[0].DateReceived
			for _, e := range emails {
				if e.DateReceived.After(newest) {
					newest = e.DateReceived
				}
			}
			// list the top-5 by date
			for i := 0; i < len(emails) && i < 5; i++ {
				fmt.Printf("   sample[%d] id=%s date=%s subj=%q\n", i, emails[i].ServerID, emails[i].DateReceived.Format(time.RFC3339), emails[i].Subject)
			}
			fmt.Printf("   NEWEST: %s\n", newest.Format(time.RFC3339))
			// count per day
			counts := map[string]int{}
			for _, e := range emails {
				counts[e.DateReceived.Format("2006-01-02")]++
			}
			days := make([]string, 0, len(counts))
			for d := range counts {
				days = append(days, d)
			}
			// sort desc (simplest: 5 most recent keys)
			for i := 0; i < len(days); i++ {
				for j := i + 1; j < len(days); j++ {
					if days[j] > days[i] {
						days[i], days[j] = days[j], days[i]
					}
				}
			}
			if len(days) > 8 {
				days = days[:8]
			}
			for _, d := range days {
				fmt.Printf("   %s: %d\n", d, counts[d])
			}
		}
	}
}