package registry

import (
	"ringmaster/internal/config"
	"testing"
)

func TestRegistry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["ninerouter"] = config.ProviderConfig{BaseURL: "http://localhost:20128/v1"}
	cfg.Agents["master"] = config.AgentConfig{Provider: "ninerouter", Model: "opus", Role: "master"}
	cfg.Agents["coder"] = config.AgentConfig{Provider: "ninerouter", Model: "qwen", Role: "worker"}

	reg := NewRegistry(cfg)

	workers := reg.ListWorkers()
	if len(workers) != 1 || workers["coder"].Model != "qwen" {
		t.Errorf("unexpected workers: %+v", workers)
	}

	masters := reg.ListMasters()
	if len(masters) != 1 || masters["master"].Model != "opus" {
		t.Errorf("unexpected masters: %+v", masters)
	}

	p, a, err := reg.GetProviderForAgent("coder")
	if err != nil || p == nil || a.Model != "qwen" {
		t.Errorf("failed to get provider for agent: %v", err)
	}

	// Unknown agent
	_, _, err = reg.GetProviderForAgent("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent agent, got nil")
	}
}
