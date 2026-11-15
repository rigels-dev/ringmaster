package schemas

import "testing"

func TestBudgetTokensCountCacheReadsAsATenth(t *testing.T) {
	cases := []struct {
		name string
		u    Usage
		want int
	}{
		{"no cache: input and output in full", Usage{TokensIn: 1000, TokensOut: 200}, 1200},
		{"cache reads count a tenth", Usage{TokensIn: 400000, TokensCached: 396000, TokensOut: 1000}, 4000 + 39600 + 1000},
		{"everything cached", Usage{TokensIn: 1000, TokensCached: 1000}, 100},
		{"a cached count above the input is capped", Usage{TokensIn: 100, TokensCached: 500, TokensOut: 5}, 10 + 5},
		{"nothing used", Usage{}, 0},
	}
	for _, c := range cases {
		if got := c.u.BudgetTokens(); got != c.want {
			t.Errorf("%s: BudgetTokens() = %d, want %d", c.name, got, c.want)
		}
	}

	// The raw total is unchanged; it is what the usage reports show
	if got := (Usage{TokensIn: 400000, TokensCached: 396000, TokensOut: 1000}).TokensTotal(); got != 401000 {
		t.Errorf("TokensTotal() = %d, want 401000", got)
	}
}
