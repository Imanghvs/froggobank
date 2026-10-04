package domain

import "fmt"

type Currency string

const (
	CurrencyEUR Currency = "EUR"
	CurrencyUSD Currency = "USD"
	CurrencyGBP Currency = "GBP"
)

// Scale is the number of decimal places in the currency's minor unit.
// EUR, USD and GBP each have 100 minor units per major unit.
func (c Currency) Scale() (uint8, error) {
	switch c {
	case CurrencyEUR, CurrencyUSD, CurrencyGBP:
		return 2, nil
	default:
		return 0, fmt.Errorf("%w: %q is not a supported currency", ErrInvalidCurrency, c)
	}
}

func (c Currency) IsSupported() bool {
	_, err := c.Scale()
	return err == nil
}

func ParseCurrency(value string) (Currency, error) {
	currency := Currency(value)
	if _, err := currency.Scale(); err != nil {
		return "", err
	}
	return currency, nil
}
