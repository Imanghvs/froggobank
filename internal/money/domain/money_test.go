package domain

import (
	"errors"
	"math"
	"testing"
)

func TestCurrencyRules(t *testing.T) {
	for _, currency := range []Currency{CurrencyEUR, CurrencyUSD, CurrencyGBP} {
		t.Run(string(currency), func(t *testing.T) {
			parsed, err := ParseCurrency(string(currency))
			if err != nil || parsed != currency || !currency.IsSupported() {
				t.Fatalf("expected supported currency %s, got %s, %v", currency, parsed, err)
			}
			scale, err := currency.Scale()
			if err != nil || scale != 2 {
				t.Fatalf("expected scale 2 for %s, got %d, %v", currency, scale, err)
			}
		})
	}
	for _, value := range []string{"", "JPY", "eur", " EUR "} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseCurrency(value); !errors.Is(err, ErrInvalidCurrency) {
				t.Errorf("expected ErrInvalidCurrency, got %v", err)
			}
			if _, err := Currency(value).Scale(); !errors.Is(err, ErrInvalidCurrency) {
				t.Errorf("expected unsupported scale error, got %v", err)
			}
			if Currency(value).IsSupported() {
				t.Error("unexpected supported currency")
			}
		})
	}
}

func TestMoneyPreservesIntegerMinorUnits(t *testing.T) {
	for _, units := range []int64{0, 1, -1, 12345, math.MaxInt64, math.MinInt64} {
		m, err := New(units, CurrencyEUR)
		if err != nil {
			t.Fatalf("construct money: %v", err)
		}
		if m.MinorUnits() != units || m.Currency() != CurrencyEUR {
			t.Errorf("expected %d EUR minor units, got %+v", units, m)
		}
	}
}

func TestMoneyRejectsUnsupportedCurrency(t *testing.T) {
	if _, err := New(100, "JPY"); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("expected ErrInvalidCurrency, got %v", err)
	}
	if err := (Money{}).Validate(); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("expected zero-value money to be invalid, got %v", err)
	}
}

func TestMoneyArithmetic(t *testing.T) {
	for _, tc := range []struct {
		name     string
		a, b     int64
		subtract bool
		expected int64
		err      error
	}{
		{name: "add", a: 100, b: 25, expected: 125},
		{name: "add negative", a: 100, b: -25, expected: 75},
		{name: "add extremes", a: math.MaxInt64, b: math.MinInt64, expected: -1},
		{name: "add upper overflow", a: math.MaxInt64, b: 1, err: ErrOverflow},
		{name: "add lower overflow", a: math.MinInt64, b: -1, err: ErrOverflow},
		{name: "add at upper bound", a: math.MaxInt64 - 1, b: 1, expected: math.MaxInt64},
		{name: "add at lower bound", a: math.MinInt64 + 1, b: -1, expected: math.MinInt64},
		{name: "subtract", a: 100, b: 25, subtract: true, expected: 75},
		{name: "subtract negative", a: 100, b: -25, subtract: true, expected: 125},
		{name: "subtract upper overflow", a: math.MaxInt64, b: -1, subtract: true, err: ErrOverflow},
		{name: "subtract lower overflow", a: math.MinInt64, b: 1, subtract: true, err: ErrOverflow},
		{name: "subtract minimum overflow", a: 0, b: math.MinInt64, subtract: true, err: ErrOverflow},
		{name: "subtract minimum safely", a: -1, b: math.MinInt64, subtract: true, expected: math.MaxInt64},
		{name: "subtract equal minimum", a: math.MinInt64, b: math.MinInt64, subtract: true, expected: 0},
		{name: "subtract equal maximum", a: math.MaxInt64, b: math.MaxInt64, subtract: true, expected: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := New(tc.a, CurrencyEUR)
			if err != nil {
				t.Fatal(err)
			}
			b, err := New(tc.b, CurrencyEUR)
			if err != nil {
				t.Fatal(err)
			}
			var result Money
			if tc.subtract {
				result, err = a.Subtract(b)
			} else {
				result, err = a.Add(b)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected error %v, got %v", tc.err, err)
			}
			if tc.err == nil && (result.MinorUnits() != tc.expected || result.Currency() != CurrencyEUR) {
				t.Errorf("expected %d EUR minor units, got %+v", tc.expected, result)
			}
			if a.MinorUnits() != tc.a || b.MinorUnits() != tc.b {
				t.Error("arithmetic changed its operands")
			}
		})
	}
}

func TestMoneyArithmeticRejectsCurrencyMismatchAndInvalidValues(t *testing.T) {
	eur, _ := New(100, CurrencyEUR)
	usd, _ := New(100, CurrencyUSD)
	for _, tc := range []struct {
		a, b Money
		err  error
	}{
		{eur, usd, ErrCurrencyMismatch},
		{Money{}, eur, ErrInvalidCurrency},
		{eur, Money{}, ErrInvalidCurrency},
	} {
		if _, err := tc.a.Add(tc.b); !errors.Is(err, tc.err) {
			t.Errorf("addition: expected %v, got %v", tc.err, err)
		}
		if _, err := tc.a.Subtract(tc.b); !errors.Is(err, tc.err) {
			t.Errorf("subtraction: expected %v, got %v", tc.err, err)
		}
	}
}
