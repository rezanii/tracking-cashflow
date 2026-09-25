package utils

import "github.com/shopspring/decimal"

// Money keeps every amount at two decimal places, matching DECIMAL(18,2) in the database.
const moneyScale = 2

func Zero() decimal.Decimal {
	return decimal.NewFromInt(0).Round(moneyScale)
}

func Round(value decimal.Decimal) decimal.Decimal {
	return value.Round(moneyScale)
}

// Balance is income minus expense. Transfers never reach this function: they move money
// between accounts and are neither a gain nor a loss.
func Balance(totalIncome, totalExpense decimal.Decimal) decimal.Decimal {
	return Round(totalIncome.Sub(totalExpense))
}
