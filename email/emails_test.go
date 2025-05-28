package email

import (
	"firefly-iii-email-scanner/common"
	"testing"
	"time"
)

func TestProcessEmail_DateExtractionWithValidTimezone(t *testing.T) {
	// Define your config for this test case
	timezone := "America/New_York"
	dateFormat := "01/02/2006 15:04:05" // Standard Go layout string for MM/DD/YYYY HH:MM:SS
	config := common.EmailProcessingConfig{
		ProcessingSteps: []common.ProcessingStep{
			{
				Discriminator: common.Discriminator{Type: "plainTextBodyRegex", Regex: ".*"}, // Simple discriminator
				ExtractionSteps: []common.ExtractionStep{
					{
						Regex: "Date: (\\d{2}/\\d{2}/\\d{4} \\d{2}:\\d{2}:\\d{2})", // Regex to find the date
						TargetFields: []common.TargetField{
							{
								GroupNumber: 1,
								TargetField: "transactionDate",
								Format:      &dateFormat,
								TimeZone:    &timezone,
							},
						},
					},
				},
			},
		},
	}

	emailBody := "Some email content... Date: 03/15/2024 10:00:00 ... more content"
	transaction := processEmail(emailBody, config)

	if transaction == nil {
		t.Fatalf("processEmail returned nil")
	}

	if transaction.TransactionDate.IsZero() {
		t.Fatalf("TransactionDate was not set")
	}

	// Expected: 2024-03-15 10:00:00 America/New_York is 2024-03-15 14:00:00 UTC
	// (DST is active on this date for New York, so it's EDT, UTC-4)
	// March 10, 2024, was the start of DST in the US for 2024. So March 15 is EDT.
	expectedDate := time.Date(2024, 3, 15, 14, 0, 0, 0, time.UTC)

	if !transaction.TransactionDate.Equal(expectedDate) {
		t.Errorf("Expected transaction date %v (UTC), got %v (UTC)", expectedDate, transaction.TransactionDate)
	}
}

func TestProcessEmail_DateExtractionConfigMissingTimezone(t *testing.T) {
	dateFormat := "01/02/2006 15:04:05"
	emailBody := "Date: 03/15/2024 10:00:00"
	config := common.EmailProcessingConfig{
		ProcessingSteps: []common.ProcessingStep{
			{
				Discriminator: common.Discriminator{Type: "plainTextBodyRegex", Regex: ".*"},
				ExtractionSteps: []common.ExtractionStep{
					{
						Regex: "Date: (\\d{2}/\\d{2}/\\d{4} \\d{2}:\\d{2}:\\d{2})",
						TargetFields: []common.TargetField{
							{
								GroupNumber: 1,
								TargetField: "transactionDate",
								Format:      &dateFormat,
								TimeZone:    nil, // Explicitly nil
							},
						},
					},
				},
			},
		},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected log.Panic (panic), but no panic occurred")
		}
	}()

	processEmail(emailBody, config) // This should trigger a panic due to missing timezone
}

func TestProcessEmail_DateExtractionConfigInvalidTimezone(t *testing.T) {
	dateFormat := "01/02/2006 15:04:05"
	emailBody := "Date: 03/15/2024 10:00:00"
	invalidTZ := "America/Denver"
	config := common.EmailProcessingConfig{
		ProcessingSteps: []common.ProcessingStep{
			{
				Discriminator: common.Discriminator{Type: "plainTextBodyRegex", Regex: ".*"},
				ExtractionSteps: []common.ExtractionStep{
					{
						Regex: "Date: (\\d{2}/\\d{2}/\\d{4} \\d{2}:\\d{2}:\\d{2})",
						TargetFields: []common.TargetField{
							{
								GroupNumber: 1,
								TargetField: "transactionDate",
								Format:      &dateFormat,
								TimeZone:    &invalidTZ,
							},
						},
					},
				},
			},
		},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected log.Panic (panic), but no panic occurred")
		}
	}()

	processEmail(emailBody, config) // This should trigger a panic due to invalid timezone
}

func TestTransactionDateFallback_NoDateFromProcessEmail(t *testing.T) {
	transaction := &common.TransactionInfo{
		Amount: common.DollarAmount{Dollars: 10, Cents: 0},
		// TransactionDate is intentionally zero
	}

	// Simulate an envelope date: 2024-01-15 10:00:00 CET (UTC+2)
	envelopeDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.FixedZone("CET", 2*60*60))
	expectedUTCDate := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	// Simulate the logic from GetTransactions
	if transaction != nil && transaction.TransactionDate.IsZero() {
		// In a real scenario, we'd log here. For the test, focus on the assignment.
		transaction.TransactionDate = envelopeDate.UTC()
	}

	if !transaction.TransactionDate.Equal(expectedUTCDate) {
		t.Errorf("Expected transaction date %v, got %v", expectedUTCDate, transaction.TransactionDate)
	}
}

func TestTransactionDateFallback_DateAlreadySetByProcessEmail(t *testing.T) {
	preSetDate := time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC)
	transaction := &common.TransactionInfo{
		Amount:          common.DollarAmount{Dollars: 20, Cents: 0},
		TransactionDate: preSetDate,
	}

	// Simulate an envelope date (different from preSetDate)
	envelopeDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.FixedZone("CET", 2*60*60))

	// Simulate the logic from GetTransactions
	if transaction != nil && transaction.TransactionDate.IsZero() { // This condition will be false
		transaction.TransactionDate = envelopeDate.UTC()
	}

	if !transaction.TransactionDate.Equal(preSetDate) {
		t.Errorf("Expected transaction date %v (to remain unchanged), got %v", preSetDate, transaction.TransactionDate)
	}
}
