package schemas

type Usage struct {
	Calls     int `json:"calls" yaml:"calls"`
	TokensIn  int `json:"tokens_in" yaml:"tokens_in"`
	TokensOut int `json:"tokens_out" yaml:"tokens_out"`
}

func (u Usage) TokensTotal() int {
	return u.TokensIn + u.TokensOut
}

func (u *Usage) Add(other Usage) {
	u.Calls += other.Calls
	u.TokensIn += other.TokensIn
	u.TokensOut += other.TokensOut
}
