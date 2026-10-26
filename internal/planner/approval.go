package planner

import (
	"fmt"
	"os"
	"path/filepath"

	"ringmaster/internal/schemas"
	"ringmaster/internal/storage"
)

// ReadPlanFile reads and parses plan.json from disk.
func ReadPlanFile(path string) (*schemas.Plan, error) {
	return storage.ReadJSON[schemas.Plan](path)
}

// ApprovedPlan stores the plan and its canonical plan hash.
type ApprovedPlan struct {
	PlanHash string        `json:"plan_hash"`
	Plan     *schemas.Plan `json:"plan"`
}

// ApprovePlan creates plan.approved.json with the canonical PlanHash.
func ApprovePlan(projectRoot string, plan *schemas.Plan) (string, error) {
	hash, err := plan.PlanHash()
	if err != nil {
		return "", fmt.Errorf("failed to compute plan hash: %w", err)
	}

	approved := ApprovedPlan{
		PlanHash: hash,
		Plan:     plan,
	}

	approvedPath := filepath.Join(projectRoot, "plan.approved.json")
	if err := storage.WriteJSONAtomic(approvedPath, approved); err != nil {
		return "", fmt.Errorf("failed to write plan.approved.json: %w", err)
	}

	return hash, nil
}

// VerifyPlanApproval verifies that plan.approved.json exists and matches current plan.json.
func VerifyPlanApproval(projectRoot string, currentPlan *schemas.Plan) (bool, error) {
	approvedPath := filepath.Join(projectRoot, "plan.approved.json")
	approved, err := storage.ReadJSON[ApprovedPlan](approvedPath)
	if err != nil {
		return false, fmt.Errorf("plan is not approved (run 'ringmaster plan' first): %w", err)
	}

	currentHash, err := currentPlan.PlanHash()
	if err != nil {
		return false, fmt.Errorf("failed to hash current plan: %w", err)
	}

	if approved.PlanHash != currentHash {
		return false, fmt.Errorf("plan.json has been modified since approval (approved hash: %s, current: %s)",
			approved.PlanHash[:min(8, len(approved.PlanHash))], currentHash[:min(8, len(currentHash))])
	}

	return true, nil
}

// RevokeApproval removes plan.approved.json (e.g. after editing plan.json).
func RevokeApproval(projectRoot string) error {
	approvedPath := filepath.Join(projectRoot, "plan.approved.json")
	_ = os.Remove(approvedPath)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
