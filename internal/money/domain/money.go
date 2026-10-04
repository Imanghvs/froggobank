package domain

import "math"

// Money is an immutable, signed quantity of integer minor units.
// Its zero value is invalid; use New to specify a supported currency.
type Money struct {
	minorUnits int64
	currency   Currency
}

func New(minorUnits int64, currency Currency) (Money, error) {
	m := Money{minorUnits: minorUnits, currency: currency}
	if err := m.Validate(); err != nil {
		return Money{}, err
	}
	return m, nil
}

func (m Money) MinorUnits() int64  { return m.minorUnits }
func (m Money) Currency() Currency { return m.currency }

func (m Money) Validate() error {
	_, err := m.currency.Scale()
	return err
}

func (m Money) Add(other Money) (Money, error) {
	if err := m.checkCurrency(other); err != nil {
		return Money{}, err
	}
	a, b := m.minorUnits, other.minorUnits
	if (b > 0 && a > (math.MaxInt64-b)) || (b < 0 && a < (math.MinInt64-b)) {
		return Money{}, ErrOverflow
	}
	return New(a+b, m.currency)
}

func (m Money) Subtract(other Money) (Money, error) {
	if err := m.checkCurrency(other); err != nil {
		return Money{}, err
	}
	a, b := m.minorUnits, other.minorUnits
	if (b > 0 && a < (math.MinInt64+b)) || (b < 0 && a > (math.MaxInt64+b)) {
		return Money{}, ErrOverflow
	}
	return New(a-b, m.currency)
}

func (m Money) checkCurrency(other Money) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}
	if m.currency != other.currency {
		return ErrCurrencyMismatch
	}
	return nil
}
