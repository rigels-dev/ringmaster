package registry

import (
	"fmt"
	"ringmaster/internal/config"
	"ringmaster/internal/llm"
)

type Registry struct {
	Config    *config.Config
	Providers map[string]llm.Provider
}

func NewRegistry(cfg *config.Config) *Registry {
	providers := make(map[string]llm.Provider)
	for name, p := range cfg.Providers {
		providers[name] = llm.NewProvider(p.Type, p.BaseURL, p.APIKeyEnv)
	}

	return &Registry{
		Config:    cfg,
		Providers: providers,
	}
}

func (r *Registry) GetAgent(alias string) (*config.AgentConfig, error) {
	agent, ok := r.Config.Agents[alias]
	if !ok {
		return nil, fmt.Errorf("agent '%s' not found in registry", alias)
	}
	return &agent, nil
}

func (r *Registry) GetProviderForAgent(alias string) (llm.Provider, *config.AgentConfig, error) {
	agent, err := r.GetAgent(alias)
	if err != nil {
		return nil, nil, err
	}
	provider, ok := r.Providers[agent.Provider]
	if !ok {
		return nil, nil, fmt.Errorf("provider '%s' for agent '%s' not registered", agent.Provider, alias)
	}
	return provider, agent, nil
}

func (r *Registry) ListWorkers() map[string]config.AgentConfig {
	res := make(map[string]config.AgentConfig)
	for name, a := range r.Config.Agents {
		if a.Role == "worker" {
			res[name] = a
		}
	}
	return res
}

func (r *Registry) ListMasters() map[string]config.AgentConfig {
	res := make(map[string]config.AgentConfig)
	for name, a := range r.Config.Agents {
		if a.Role == "master" {
			res[name] = a
		}
	}
	return res
}
