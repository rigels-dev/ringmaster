package schemas

type Usage struct {
	Calls     int `json:"calls" yaml:"calls"`
	TokensIn  int `json:"tokens_in" yaml:"tokens_in"` // includes TokensCached
	TokensOut int `json:"tokens_out" yaml:"tokens_out"`

	// TokensCached is the part of TokensIn read from the prompt cache
	TokensCached int     `json:"tokens_cached,omitempty" yaml:"tokens_cached,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty" yaml:"cost_usd,omitempty"`
}

func (u Usage) TokensTotal() int {
	return u.TokensIn + u.TokensOut
}

// CachedTokenWeight is how much one token read from the prompt cache counts
// toward a budget: a tenth, which is what providers charge for it.
const CachedTokenWeight = 10

// BudgetTokens is what a budget is measured against. A token read from the
// prompt cache counts as a tenth of a token; everything else counts in full.
//
// An agentic provider re-reads its whole conversation from the cache on
// every step, so TokensIn grows by the full context each time even though
// almost nothing new is processed. Counting those reads in full made small
// tasks hit a budget they were nowhere near in cost.
func (u Usage) BudgetTokens() int {
	cached := u.TokensCached
	if cached > u.TokensIn {
		cached = u.TokensIn
	}
	return u.TokensIn - cached + cached/CachedTokenWeight + u.TokensOut
}

func (u *Usage) Add(other Usage) {
	u.Calls += other.Calls
	u.TokensIn += other.TokensIn
	u.TokensOut += other.TokensOut
	u.TokensCached += other.TokensCached
	u.CostUSD += other.CostUSD
}
