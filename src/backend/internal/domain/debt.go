package domain

type DebtItem struct {
	FromUserID string
	ToUserID   string
	Amount     int64 // in kopecks
}

type PaymentSummary struct {
	TotalPaid          int64 // total amount paid by this user across purchases
	OwnShare           int64 // this user's own share of all purchases
	ExpectedFromOthers int64 // total amount others owe this user
	PaymentsReceived   int64 // total payments received from others
	StillAwaiting      int64 // ExpectedFromOthers - PaymentsReceived
}

type CalculationResult struct {
	Debts     []DebtItem
	Balances  map[string]int64           // user_id -> balance in kopecks
	Summaries map[string]*PaymentSummary // user_id -> payment summary
}
