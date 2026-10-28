package planner

import (
	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
)

// TaskEstimate holds token usage boundaries for a single task.
type TaskEstimate struct {
	TaskID string
	Min    int
	Max    int
}

// ProjectEstimate holds cumulative token boundaries for the entire project.
type ProjectEstimate struct {
	Tasks    []TaskEstimate
	TotalMin int
	TotalMax int
}

// CalculateEstimates computes min and max token ranges per task and overall project.
func CalculateEstimates(plan *schemas.Plan, estimatesCfg map[string]config.EstimateConfig) ProjectEstimate {
	res := ProjectEstimate{
		Tasks: make([]TaskEstimate, len(plan.Tasks)),
	}

	for i, t := range plan.Tasks {
		tType := string(t.Type)
		cfgVal, ok := estimatesCfg[tType]
		minTok := cfgVal.TokensIn[0] + cfgVal.TokensOut[0]
		maxTok := cfgVal.TokensIn[1] + cfgVal.TokensOut[1]

		if !ok || (minTok == 0 && maxTok == 0) {
			// Sensible defaults if not explicitly configured
			switch t.Type {
			case schemas.TaskTypeBackend:
				minTok, maxTok = 40000, 120000
			case schemas.TaskTypeFrontend:
				minTok, maxTok = 30000, 90000
			case schemas.TaskTypeTest:
				minTok, maxTok = 20000, 60000
			case schemas.TaskTypeDocs:
				minTok, maxTok = 10000, 30000
			case schemas.TaskTypeInfra:
				minTok, maxTok = 15000, 45000
			default:
				minTok, maxTok = 20000, 80000
			}
		}

		res.Tasks[i] = TaskEstimate{
			TaskID: t.ID,
			Min:    minTok,
			Max:    maxTok,
		}
		res.TotalMin += minTok
		res.TotalMax += maxTok
	}

	return res
}
