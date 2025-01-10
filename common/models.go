package common

import (
	"fmt"
	"strconv"
	"time"
)

type DollarAmount struct {
	Dollars int
	Cents   int
}

type TransactionType int

const (
	Transfer TransactionType = iota
	Withdrawal
	Deposit
)

type EmailTransactionInfo struct {
	Uid    uint32
	MailId string
	Info   *TransactionInfo
}

type TransactionInfo struct {
	Amount          DollarAmount
	TransactionDate time.Time
	SourceAccountId int
	DestinationName string
	Type            TransactionType
}

func ParseMoney(value string) (string, error) {
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%.2f", amount), nil
}

func (da *DollarAmount) String() string {
	return fmt.Sprintf("%d.%02d", da.Dollars, da.Cents)
}
