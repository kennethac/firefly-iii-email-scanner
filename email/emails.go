package email

import (
	"firefly-iii-email-scanner/common"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

type TextPart interface {
	GetText() string
}

type PlainTextPart struct {
	MessageId string
	Uid       uint32
	PlainText string
}

type HtmlTextPart struct {
	MessageId string
	Uid       uint32
	HtmlText  string
}

func (ptp *PlainTextPart) GetText() string {
	return ptp.PlainText
}

func (ptp *HtmlTextPart) GetText() string {
	return ptp.HtmlText
}

func ParseText(plainText string) common.TransactionInfo {
	lines := strings.Split(plainText, "\n")

	retVal := common.TransactionInfo{}

	amountAndAccountRegex, _ := regexp.Compile(`\$([\d,]+)\.(\d\d) came out of your account ending in (\d+)`)
	amountRegex, _ := regexp.Compile(`^Amount:\s*\$([\d,]+)\.(\d\d)`)
	detailsRegex, _ := regexp.Compile(`^Details:\s*(.*)$`)
	toRegex, _ := regexp.Compile("^To:(.+)$")
	dateRegex, _ := regexp.Compile("^Date:(.+)$")

	matches := 0

	for _, line := range lines {
		amountAndAccountResult := amountAndAccountRegex.FindStringSubmatch(line)
		if amountAndAccountResult != nil {
			matches = matches + 1
			dollars, _ := strconv.Atoi(strings.ReplaceAll(amountAndAccountResult[1], ",", ""))
			cents, _ := strconv.Atoi(amountAndAccountResult[2])
			account := amountAndAccountResult[3]

			retVal.SourceAccountId, _ = strconv.Atoi(account) // this is clearly wrong, just a temp thing
			retVal.Amount = common.DollarAmount{
				Dollars: dollars,
				Cents:   cents,
			}
			continue
		} else {
			amountRegex := amountRegex.FindStringSubmatch(line)
			detailsRegex := detailsRegex.FindStringSubmatch(line)

			if amountRegex != nil && detailsRegex != nil {
				matches = matches + 1
				dollars, _ := strconv.Atoi(strings.ReplaceAll(amountRegex[1], ",", ""))
				cents, _ := strconv.Atoi(amountRegex[2])
				retVal.Amount = common.DollarAmount{
					Dollars: dollars,
					Cents:   cents,
				}
				retVal.DestinationName = strings.TrimSpace(detailsRegex[1])
				continue
			}
		}

		toResult := toRegex.FindStringSubmatch(line)
		if toResult != nil {
			matches = matches + 1
			target := strings.TrimSpace(toResult[1])
			retVal.DestinationName = target
			continue
		}

		dateResult := dateRegex.FindStringSubmatch(line)
		if dateResult != nil {
			matches = matches + 1
			date, timeParseError := time.Parse("01/02/06", strings.TrimSpace(dateResult[1]))
			if timeParseError == nil {
				retVal.TransactionDate = date
			} else {
				log.Fatal(timeParseError)
			}
			continue
		}
	}
	if matches != 3 {
		log.Fatalf("Failed to extract all info from email\n%s", plainText)
	}
	return retVal
}

var c *client.Client

func GetTransactions(configs []common.EmailProcessingConfig) []common.EmailTransactionInfo {
	var server = os.Getenv("IMAP_SERVER")
	var email = os.Getenv("IMAP_EMAIL")
	var password = os.Getenv("IMAP_PASSWORD")

	// Connect to Gmail
	log.Printf("Connecting to server \"%s\"...\n", server)
	var err error
	c, err = client.DialTLS(server, nil)
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}
	log.Println("Connected to Gmail IMAP server.")

	// Login
	if err := c.Login(email, password); err != nil {
		log.Fatal("Error logging in:", err)
	}
	log.Println("Logged in as", email)

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal("Error selecting INBOX:", err)
	}
	log.Printf("INBOX has %d messages\n", mbox.Messages)

	if mbox.Messages == 0 {
		log.Println("No messages to fetch")
		return []common.EmailTransactionInfo{}
	}

	var result []common.EmailTransactionInfo

	for _, config := range configs {
		log.Printf("Checking for emails from %s", config.FromEmail)

		criteria := imap.NewSearchCriteria()
		criteria.Header.Set("From", config.FromEmail)
		criteria.WithoutFlags = []string{"\\Seen"}

		searchRes, searchErr := c.Search(criteria)
		if searchErr != nil {
			log.Fatal("Error searching for email:", searchErr)
		}

		if len(searchRes) == 0 {
			log.Println("No emails matching filters were found")
			continue
		}

		log.Printf("Got %d search results to process\n", len(searchRes))

		seqSet := new(imap.SeqSet)
		seqSet.AddNum(searchRes...)

		messages := make(chan *imap.Message, mbox.Messages)
		done := make(chan error, 1)

		section := &imap.BodySectionName{}
		section.Peek = true

		go func() {
			done <- c.Fetch(seqSet, []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope, imap.FetchUid}, messages)
		}()

		if err := <-done; err != nil {
			log.Fatal("Error fetching message:", err)
		}

		for msg := range messages {
			messageId := msg.Envelope.MessageId
			uid := msg.Uid

			m, err := mail.CreateReader(msg.GetBody(section))
			if err != nil {
				log.Fatal(err)
			}

			var textPart *PlainTextPart
			var htmlPart *HtmlTextPart

			for {
				part, err := m.NextPart()
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Printf("(skip mail): parse part: %v", err)
					break
				}

				switch part.Header.(type) {
				case *mail.InlineHeader:
					contentType := part.Header.Get("Content-Type")
					if strings.Contains(contentType, "text/plain") {
						if textPart != nil {
							log.Printf("Skipping a second inline text section")
							continue
						}
						body, err := io.ReadAll(part.Body)
						if err != nil {
							log.Printf("(skip) read plain body: %v", err)
						} else {
							textPart = &PlainTextPart{
								MessageId: messageId,
								Uid:       uid,
								PlainText: string(body),
							}
						}
					} else if strings.Contains(contentType, "text/html") {
						if htmlPart != nil {
							log.Printf("Skipping a second inline HTML section")
							continue
						}
						body, err := io.ReadAll(part.Body)
						if err != nil {
							log.Printf("Failed to read HTML: %v", err)
						} else {
							htmlPart = &HtmlTextPart{
								MessageId: messageId,
								HtmlText:  string(body),
							}
						}
					}
				case *mail.AttachmentHeader:
					log.Printf("Skipping attachment.")
				default:
					log.Printf("Not sure what I've seen here")
				}
			}

			var transaction *common.TransactionInfo
			if textPart != nil {
				transaction = processEmail(textPart.GetText(), config)
			} else if htmlPart != nil {
				transaction = processEmail(htmlPart.GetText(), config)
			} else {
				log.Println("No valid parts found for email")
			}

			result = append(result, common.EmailTransactionInfo{
				Uid:    uid,
				MailId: messageId,
				Info:   transaction,
			})
		}
	}

	log.Printf("Returning %d transactions", len(result))
	return result
}

func processEmail(body string, config common.EmailProcessingConfig) *common.TransactionInfo {
	for _, step := range config.ProcessingSteps {
		if step.Discriminator.Type == "plainTextBodyRegex" {
			matched, _ := regexp.MatchString(step.Discriminator.Regex, body)
			if matched {
				transaction := common.TransactionInfo{
					SourceAccountId: step.SourceAccountId,
				}
				for _, extractionStep := range step.ExtractionSteps {
					re := regexp.MustCompile("(?m)" + extractionStep.Regex)
					matches := re.FindStringSubmatch(body)
					if matches == nil {
						log.Fatalf("Failed to extract all info from email because regex `%s` was not found\n%s", extractionStep.Regex, body)
					}
					for _, targetField := range extractionStep.TargetFields {
						value := matches[targetField.GroupNumber]
						switch targetField.TargetField {
						case "dollars":
							dollars, _ := strconv.Atoi(strings.ReplaceAll(value, ",", ""))
							transaction.Amount.Dollars = dollars
						case "cents":
							cents, _ := strconv.Atoi(value)
							transaction.Amount.Cents = cents
						case "transactionDate":
							format := "01/02/06"
							if targetField.Format != nil {
								format = *targetField.Format
							}
							date, err := time.Parse(format, strings.TrimSpace(value))
							if err != nil {
								log.Fatal(err)
							}
							transaction.TransactionDate = date
						case "destinationAccount":
							transaction.DestinationName = strings.TrimSpace(value)
						}
					}
				}
				return &transaction
			}
		}
	}

	log.Printf("No processing step matched for email\n%s", body)
	return nil
}

// Marks the email with the given uid as "read".
func MarkRead(uid uint32) {
	// if _, selectErr := c.Select("INBOX", false); selectErr != nil {
	// 	log.Fatal("Unable to select INBOX:", selectErr)
	// }

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.SeenFlag}

	if err := c.Store(seqSet, item, flags, nil); err != nil {
		log.Fatal("Unable to mark message as read:", err)
	} else {
		log.Printf("Should have marked as read")
	}
}
