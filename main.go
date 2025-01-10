package main

import (
	"firefly-iii-email-scanner/common"
	"firefly-iii-email-scanner/email"
	"firefly-iii-email-scanner/firefly"
	"firefly-iii-email-scanner/mattermost"
	"fmt"
	"log"
	"os"
)

func main() {
	fireflyUrl := os.Getenv("FIREFLY_URL")

	config, err := common.GetConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := firefly.Init(); err != nil {
		log.Fatalf("Failed to initialize Firefly client: %v", err)
	}
	defer firefly.Cleanup()

	transactions := email.GetTransactions(config.ProcessEmails)
	for _, t := range transactions {
		if t.Info != nil {
			info := *t.Info
			foundMatch := firefly.GetExistingTransaction(info)

			if foundMatch == nil {
				log.Printf("Found no close matches for $%d.%02d to %s on %s", info.Amount.Dollars, info.Amount.Cents, info.DestinationName, info.TransactionDate)
				newTransactionId, matchedAccountName, err := firefly.CreateTransaction(info)
				if err != nil {
					log.Fatal(err)
				}
				url := fmt.Sprintf("%s/transactions/show/%d", fireflyUrl, newTransactionId)

				message := fmt.Sprintf(`## [New Transaction Created From Email](%s)

Please confirm:

**Destination**: %s -> %s
**Amount**: $%d.%02d
**Date**: %s`,
					url,
					info.DestinationName,
					*matchedAccountName,
					info.Amount.Dollars,
					info.Amount.Cents,
					info.TransactionDate.Format("Jan 02 , 2006"))

				if err := mattermost.CreateMessage(message); err != nil {
					log.Println(err)
				}
			} else {
				log.Printf("Close match found for $%d.%02d to %s", info.Amount.Dollars, info.Amount.Cents, info.DestinationName)
				groupTitle := foundMatch.Attributes.GroupTitle

				var title string
				if groupTitle == nil {
					title = foundMatch.Attributes.Transactions[0].Description
				} else {
					title = *groupTitle
				}

				foundDate := foundMatch.Attributes.Transactions[0].Date

				foundAccount := foundMatch.Attributes.Transactions[0].DestinationName
				url := fmt.Sprintf("%s/transactions/show/%s", fireflyUrl, foundMatch.Id)

				message := fmt.Sprintf(`## New Transaction Email Matched

Found an existing Firefly transaction [%s](%s).

Please confirm:

**Destination**: %s (%s)
**Amount**: $%d.%02d,
**Date**: %s`,
					title,
					url,
					info.DestinationName,
					*foundAccount,
					info.Amount.Dollars,
					info.Amount.Cents,
					foundDate.Format("Jan 02, 2006"))

				if err := mattermost.CreateMessage(message); err != nil {
					log.Println(err)
				}
			}
		} else {
			message := fmt.Sprintf(`## Unparsable Email

An email was received that could not be parsed. This may be a bug or it may be an irrelevant email.

**UID**: %d
**Message ID**: %s`,
				t.Uid,
				t.MailId)

			if err := mattermost.CreateMessage(message); err != nil {
				log.Println(err)
			}
		}

		email.MarkRead(t.Uid)
	}
}
