package domain

const (
	SplitModeAll     = "all"
	SplitModeCustom  = "custom"
	SplitModeAmounts = "amounts"
)

// Coverage means PayerID covers CoveredID's share in a purchase.
type Coverage struct {
	PayerID   string
	CoveredID string
}

type Purchase struct {
	ID                 string
	DealID             string
	Title              string
	Amount             int64 // in kopecks
	PaidBy             string
	SplitMode          string
	ParticipantIDs     []string
	PayerShare         int64            // payer's own share in kopecks (optional, for "all"/"custom" modes)
	ParticipantAmounts map[string]int64 // participant_id → amount in kopecks (for "amounts" mode)
}
