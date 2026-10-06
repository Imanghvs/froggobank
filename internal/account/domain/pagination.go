package domain

type PaginatedAccountsResponse struct {
	Accounts []Account
	Limit    int
	Offset   int
}
