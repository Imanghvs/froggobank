package domain

type AccountType string

const (
	Asset     AccountType = "asset"
	Liability AccountType = "liability"
	Equity    AccountType = "equity"
	Revenue   AccountType = "revenue"
	Expense   AccountType = "expense"
)

type NormalSide string

const (
	DebitNormal  NormalSide = "debit"
	CreditNormal NormalSide = "credit"
)

func ParseAccountType(value string) (AccountType, error) {
	accountType := AccountType(value)
	if _, err := accountType.NormalSide(); err != nil {
		return "", err
	}
	return accountType, nil
}

func (t AccountType) NormalSide() (NormalSide, error) {
	switch t {
	case Asset, Expense:
		return DebitNormal, nil
	case Liability, Equity, Revenue:
		return CreditNormal, nil
	default:
		return "", ErrInvalidAccountType
	}
}
