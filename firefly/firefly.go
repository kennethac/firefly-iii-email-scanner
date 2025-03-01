package firefly

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yml firefly-iii-6.1.24-v1.yaml

import (
	"context"
	"firefly-iii-email-scanner/common"
	"firefly-iii-email-scanner/util"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	client             *ClientWithResponses
	recentTransactions []TransactionRead
	accounts           []AccountRead
	cleanAccountNames  map[string]string
)

// The account name for the "no name" account, which should be used
// when no other account is found that seems to match the transaction.
// I intentially do not create new accounts, though they _can_ implicitly
// be created by setting the name of the account on the transaction.
const noNameName = "(no name)"

// Prepares the Firefly client for use by initializing the client and fetching
// some initial data about recent transactions and accounts.
//
// The recent transactions are used while matching transaction candidates from emails to
// existing transactions in Firefly. The last 120 days of transactions are retrieved, so
// if an email is received for a transaction that is older than that, it will not be matched.
//
// A call to `Init` should be followed by a call to `Cleanup` when the client is no longer needed.
// and those caches should be reset.
func Init() error {
	url := os.Getenv("FIREFLY_URL")
	pat := os.Getenv("FIREFLY_PAT")
	var err error
	client, err = NewClientWithResponses(
		url+"/api",
		WithHTTPClient(http.DefaultClient),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+pat)
			return nil
		}),
	)
	if err != nil {
		return err
	}

	recentTransactions, err = getRecentTransactions()
	if err != nil {
		client = nil
		return err
	}

	accounts, err = getAllAccounts()
	if err != nil {
		client = nil
		return err
	}

	cleanAccountNames = make(map[string]string)
	for _, account := range accounts {
		cleanAccountNames[account.Id] = cleanString(account.Attributes.Name)
	}

	return nil
}

// Resets the Firefly client and the caches of recent transactions and accounts.
func Cleanup() {
	client = nil
	recentTransactions = nil
	accounts = nil
	cleanAccountNames = nil
}

// Retrieves the last 120 days of transactions from Firefly.
func getRecentTransactions() ([]TransactionRead, error) {
	var allTransactions []TransactionRead
	var page int32 = 1

	// Calculate the date 30 days ago
	startDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, -30*4)}

	for {
		// Set up the context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Call the API to get transactions
		params := ListTransactionParams{
			Start: &startDate,
			Page:  &page,
		}
		resp, err := client.ListTransactionWithResponse(ctx, &params)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to get transactions: %s", resp.Status())
		}

		allTransactions = append(allTransactions, resp.ApplicationvndApiJSON200.Data...)

		if resp.ApplicationvndApiJSON200.Meta.Pagination.CurrentPage != nil &&
			resp.ApplicationvndApiJSON200.Meta.Pagination.TotalPages != nil &&
			*resp.ApplicationvndApiJSON200.Meta.Pagination.CurrentPage >= *resp.ApplicationvndApiJSON200.Meta.Pagination.TotalPages {
			break
		}
		page++
	}

	return allTransactions, nil
}

// Retrieves all (relevant) accounts from Firefly.
//
// It excludes inactive, revenue and initial balance type accounts.
func getAllAccounts() ([]AccountRead, error) {
	var allAccounts []AccountRead
	var page int32 = 1

	for {
		// Set up the context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Call the API to get accounts
		params := ListAccountParams{
			Page: &page,
		}
		resp, err := client.ListAccountWithResponse(ctx, &params)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to get accounts: %s", resp.Status())
		}

		for _, account := range resp.ApplicationvndApiJSON200.Data {
			if (account.Attributes.Active == nil || *account.Attributes.Active) &&
				account.Attributes.Type != "initial-balance" &&
				account.Attributes.Type != "revenue" {
				allAccounts = append(allAccounts, account)
			}
		}

		if resp.ApplicationvndApiJSON200.Meta.Pagination.CurrentPage != nil &&
			resp.ApplicationvndApiJSON200.Meta.Pagination.TotalPages != nil &&
			*resp.ApplicationvndApiJSON200.Meta.Pagination.CurrentPage >= *resp.ApplicationvndApiJSON200.Meta.Pagination.TotalPages {
			break
		}
		page++
	}

	return allAccounts, nil
}

// Calculate the Levenshtein distance between two strings
func levenshtein(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	la := utf8.RuneCountInString(a)
	lb := utf8.RuneCountInString(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
	}
	for i := 0; i <= la; i++ {
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, min(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[la][lb]
}

// Find the length of the longest common substring between two strings
func longestCommonSubstring(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	la := utf8.RuneCountInString(a)
	lb := utf8.RuneCountInString(b)
	longest := 0
	for i := 0; i < la; i++ {
		for j := 0; j < lb; j++ {
			lcsTemp := 0
			for (i+lcsTemp < la) && (j+lcsTemp < lb) && (a[i+lcsTemp] == b[j+lcsTemp]) {
				lcsTemp++
			}
			if lcsTemp > longest {
				longest = lcsTemp
			}
		}
	}
	return longest
}

// Removes all "non-word characters" from a string so that differences in
// punctuation, spaces, etc. do not affect the comparison.
func cleanString(s string) string {
	re := regexp.MustCompile(`[\W_]+`)
	return re.ReplaceAllString(s, "")
}

// Attempts to find an account that matches the given name
func getMatchingAccount(name string) *AccountRead {
	const threshold = 3 // Adjust this threshold as needed
	var bestMatch *AccountRead
	var bestDistance = threshold + 1

	var bestSubset *AccountRead
	var bestLCS int

	cleanName := cleanString(name)

	for _, account := range accounts {
		cleanAccountName := cleanAccountNames[account.Id]

		lcs := longestCommonSubstring(cleanName, cleanAccountName)
		if lcs > bestLCS {
			bestLCS = lcs
			bestSubset = &account
		}

		distance := levenshtein(account.Attributes.Name, name)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = &account
		}
	}

	if bestSubset != nil && (bestLCS >= 5 || float64(bestLCS)/float64(len(cleanName)) >= 0.75) {
		return bestSubset
	}

	if bestMatch != nil && bestDistance <= threshold {
		return bestMatch
	}

	return nil
}

// Returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Attempts to find an existing Firefly transaction that matches the given transaction info.
// If none is found, nil is returned.
func GetExistingTransaction(t common.TransactionInfo) *TransactionRead {
	for _, or := range recentTransactions {
		o := or.Attributes
		parsedAmount, err := common.ParseMoney(o.Transactions[0].Amount)
		if err != nil {
			continue
		}

		if util.CloseDay(o.Transactions[0].Date, t.TransactionDate, 3, 3) &&
			parsedAmount == fmt.Sprintf("%d.%02d", t.Amount.Dollars, t.Amount.Cents) {
			return &or
		}
	}
	return nil
}

// Creates a transaction in Firefly according to the information provided.
func CreateTransaction(transaction common.TransactionInfo, dryRun bool) (int, *string, error) {
	noName := noNameName

	// Set up the context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sourceId := strconv.Itoa(transaction.SourceAccountId)
	matchingDestinationAccount := getMatchingAccount(transaction.DestinationName)
	var matchingAccountName *string

	var order int32 = 0
	body := StoreTransactionJSONRequestBody{
		Transactions: []TransactionSplitStore{
			{
				Date:            transaction.TransactionDate,
				Amount:          transaction.Amount.String(),
				Description:     fmt.Sprintf("Uncategorized transaction to %s", transaction.DestinationName),
				SourceId:        &sourceId,
				Order:           &order,
				DestinationId:   nil,
				DestinationName: nil,
			},
		},
	}

	// Determine the transaction type based on the matching account
	if matchingDestinationAccount == nil {
		body.Transactions[0].DestinationName = &noName
		matchingAccountName = &noName
		body.Transactions[0].Type = Withdrawal
	} else {
		body.Transactions[0].DestinationId = &matchingDestinationAccount.Id
		matchingAccountName = &matchingDestinationAccount.Attributes.Name

		if matchingDestinationAccount.Attributes.Type == "asset" {
			body.Transactions[0].Type = Transfer
		} else {
			body.Transactions[0].Type = Withdrawal
		}
	}

	if dryRun {
		return 0, matchingAccountName, nil
	}

	params := StoreTransactionParams{}
	resp, err := client.StoreTransactionWithResponse(ctx, &params, body)
	if err != nil {
		fmt.Printf("Failed to create transaction: %v", err)
		return 0, nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		fmt.Printf("Failed to create transaction: %s", resp.Status())
		return 0, nil, fmt.Errorf("failed to create transaction: %s", resp.Status())
	}

	transactionID, err := strconv.Atoi(resp.ApplicationvndApiJSON200.Data.Id)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse transaction ID: %v", err)
	}

	return transactionID, matchingAccountName, nil
}

func toFireflyType(t common.TransactionType) TransactionTypeProperty {
	switch t {
	case common.Transfer:
		return Transfer
	case common.Withdrawal:
		return Withdrawal
	case common.Deposit:
		return Deposit
	}

	panic("Should always match a transaction type. If this occurs, there is a bug")
}
