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

func (u *Usage) Add(other Usage) {
	u.Calls += other.Calls
	u.TokensIn += other.TokensIn
	u.TokensOut += other.TokensOut
	u.TokensCached += other.TokensCached
	u.CostUSD += other.CostUSD
}
