package httpapi

type getUserAccountsQuery struct {
	Limit    int     `form:"limit,default=20" binding:"min=1,max=100"`
	Offset   int     `form:"offset,default=0" binding:"min=0"`
	Currency *string `form:"currency"`
}
