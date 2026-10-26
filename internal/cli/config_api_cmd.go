package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"ringmaster/internal/config"
)

type ConfigAPIRequest struct {
	Action      string `json:"action"` // get, add_provider, delete_provider, add_agent, set_role, delete_agent
	ConfigPath  string `json:"config_path,omitempty"`
	ID          string `json:"id,omitempty"`
	Type        string `json:"type,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	APIKeyEnv   string `json:"api_key_env,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	Role        string `json:"role,omitempty"`
	ToolCalling bool   `json:"tool_calling,omitempty"`
}

var configAPICmd = &cobra.Command{
	Use:    "config-api",
	Short:  "Headless programmatic JSON API to safely manage ringmaster.toml using Go parser",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var req ConfigAPIRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			return outputJSON(false, fmt.Sprintf("invalid request payload: %v", err), nil)
		}

		targetPath := req.ConfigPath
		if targetPath == "" {
			targetPath = cfgFile
		}
		if targetPath == "" {
			found, err := config.FindConfigFile()
			if err != nil {
				targetPath = "ringmaster.toml"
			} else {
				targetPath = found
			}
		}

		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			absPath = targetPath
		}

		cfgObj, err := config.LoadFile(absPath)
		if err != nil || cfgObj == nil {
			cfgObj = config.DefaultConfig()
		}

		switch req.Action {
		case "get":
			return outputJSON(true, "", map[string]any{
				"config_path": absPath,
				"config":      cfgObj,
			})

		case "add_provider":
			if req.ID == "" {
				return outputJSON(false, "provider id is required", nil)
			}
			if cfgObj.Providers == nil {
				cfgObj.Providers = make(map[string]config.ProviderConfig)
			}
			pType := req.Type
			if pType == "" {
				pType = "openai"
			}
			cfgObj.Providers[req.ID] = config.ProviderConfig{
				Type:      pType,
				BaseURL:   req.BaseURL,
				APIKeyEnv: req.APIKeyEnv,
			}
			if err := config.SaveFile(absPath, cfgObj); err != nil {
				return outputJSON(false, err.Error(), nil)
			}
			return outputJSON(true, fmt.Sprintf("Provider '%s' saved successfully via Go core", req.ID), nil)

		case "delete_provider":
			if req.ID == "" {
				return outputJSON(false, "provider id is required", nil)
			}
			delete(cfgObj.Providers, req.ID)
			// Cascade delete agents bound to this provider
			for aID, a := range cfgObj.Agents {
				if a.Provider == req.ID {
					delete(cfgObj.Agents, aID)
				}
			}
			if err := config.SaveFile(absPath, cfgObj); err != nil {
				return outputJSON(false, err.Error(), nil)
			}
			return outputJSON(true, fmt.Sprintf("Provider '%s' deleted", req.ID), nil)

		case "add_agent":
			if req.ID == "" || req.Model == "" {
				return outputJSON(false, "agent id and model are required", nil)
			}
			if cfgObj.Agents == nil {
				cfgObj.Agents = make(map[string]config.AgentConfig)
			}
			role := req.Role
			if role == "" {
				role = "worker"
			}
			cfgObj.Agents[req.ID] = config.AgentConfig{
				Provider:    req.Provider,
				Model:       req.Model,
				Role:        role,
				ToolCalling: role == "worker",
			}
			if role == "master" {
				cfgObj.Routing.DefaultMaster = req.ID
			}
			if err := config.SaveFile(absPath, cfgObj); err != nil {
				return outputJSON(false, err.Error(), nil)
			}
			return outputJSON(true, fmt.Sprintf("Agent '%s' configured successfully via Go core", req.ID), nil)

		case "set_role":
			if req.ID == "" {
				return outputJSON(false, "agent id is required", nil)
			}
			agent, exists := cfgObj.Agents[req.ID]
			if !exists {
				return outputJSON(false, fmt.Sprintf("agent '%s' not found", req.ID), nil)
			}
			agent.Role = req.Role
			agent.ToolCalling = req.Role == "worker"
			cfgObj.Agents[req.ID] = agent

			if req.Role == "master" {
				cfgObj.Routing.DefaultMaster = req.ID
			} else if req.Role == "worker" {
				if cfgObj.Routing.DefaultMaster == req.ID {
					cfgObj.Routing.DefaultMaster = ""
				}
				cfgObj.Routing.DefaultWorker = req.ID
			}
			if err := config.SaveFile(absPath, cfgObj); err != nil {
				return outputJSON(false, err.Error(), nil)
			}
			return outputJSON(true, fmt.Sprintf("Agent '%s' role updated to %s via Go core", req.ID, req.Role), nil)

		case "delete_agent":
			if req.ID == "" {
				return outputJSON(false, "agent id is required", nil)
			}
			delete(cfgObj.Agents, req.ID)
			if cfgObj.Routing.DefaultMaster == req.ID {
				cfgObj.Routing.DefaultMaster = ""
			}
			if cfgObj.Routing.DefaultWorker == req.ID {
				cfgObj.Routing.DefaultWorker = ""
			}
			if err := config.SaveFile(absPath, cfgObj); err != nil {
				return outputJSON(false, err.Error(), nil)
			}
			return outputJSON(true, fmt.Sprintf("Agent '%s' deleted", req.ID), nil)

		default:
			return outputJSON(false, fmt.Sprintf("unknown action '%s'", req.Action), nil)
		}
	},
}

func outputJSON(ok bool, msg string, data any) error {
	resp := map[string]any{
		"ok": ok,
	}
	if msg != "" {
		if ok {
			resp["message"] = msg
		} else {
			resp["error"] = msg
		}
	}
	if data != nil {
		resp["data"] = data
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func init() {
	RootCmd.AddCommand(configAPICmd)
}
