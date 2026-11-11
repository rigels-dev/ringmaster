package tui

// The providers and agent templates the wizard offers.

type AuthMethodOption struct {
	ID          string
	Label       string
	Description string
	Type        string
	BaseURL     string
	DefaultEnv  string
	NeedsInput  bool
}

type ProviderOption struct {
	ID          string
	Name        string
	Description string
	AuthMethods []AuthMethodOption
}

var popularProviders = []ProviderOption{
	{
		ID:          "anthropic",
		Name:        "Anthropic",
		Description: "Claude 5.5 Sonnet, Opus, Haiku",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "claude_login_cli",
				Label:       "Launch Official Claude Login in Terminal (claude auth login)",
				Description: "Suspends TUI, opens official login in browser, then returns here",
				Type:        "claude_subscription",
				BaseURL:     "",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
			{
				ID:          "claude_cli",
				Label:       "Reuse Existing Session (Already logged in on this host)",
				Description: "Checks local ~/.claude credentials and uses them directly",
				Type:        "claude_subscription",
				BaseURL:     "",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
			{
				ID:          "api_key",
				Label:       "Anthropic API key (ANTHROPIC_API_KEY)",
				Description: "Direct Anthropic Messages API (api.anthropic.com/v1)",
				Type:        "anthropic",
				BaseURL:     "https://api.anthropic.com/v1",
				DefaultEnv:  "ANTHROPIC_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "openai",
		Name:        "OpenAI",
		Description: "GPT-4o, GPT-4o-mini, o1",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "OpenAI API key",
				Description: "Official api.openai.com endpoint (OPENAI_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://api.openai.com/v1",
				DefaultEnv:  "OPENAI_API_KEY",
				NeedsInput:  false,
			},
			{
				ID:          "custom_url",
				Label:       "Custom OpenAI-compatible URL",
				Description: "Reverse proxy, gateway, or enterprise OpenAI deployment",
				Type:        "openai",
				BaseURL:     "https://api.openai.com/v1",
				DefaultEnv:  "OPENAI_API_KEY",
				NeedsInput:  true,
			},
		},
	},
	{
		ID:          "openrouter",
		Name:        "OpenRouter",
		Description: "Unified gateway to Claude, Llama, Gemini, etc.",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "OpenRouter API key",
				Description: "openrouter.ai/api/v1 (OPENROUTER_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://openrouter.ai/api/v1",
				DefaultEnv:  "OPENROUTER_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "ollama",
		Name:        "Ollama",
		Description: "Local LLMs running on localhost:11434",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "local",
				Label:       "Local Ollama instance",
				Description: "http://localhost:11434/v1 (No API key needed)",
				Type:        "openai",
				BaseURL:     "http://localhost:11434/v1",
				DefaultEnv:  "",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "9router",
		Name:        "9router",
		Description: "Local multi-provider proxy on localhost:20128",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "local",
				Label:       "Local 9router instance",
				Description: "http://localhost:20128/v1 (NINEROUTER_API_KEY)",
				Type:        "openai",
				BaseURL:     "http://localhost:20128/v1",
				DefaultEnv:  "NINEROUTER_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "deepseek",
		Name:        "DeepSeek",
		Description: "DeepSeek-V3, DeepSeek-R1",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "api_key",
				Label:       "DeepSeek API key",
				Description: "api.deepseek.com/v1 (DEEPSEEK_API_KEY)",
				Type:        "openai",
				BaseURL:     "https://api.deepseek.com/v1",
				DefaultEnv:  "DEEPSEEK_API_KEY",
				NeedsInput:  false,
			},
		},
	},
	{
		ID:          "custom",
		Name:        "Custom Provider",
		Description: "Custom self-hosted model or private gateway",
		AuthMethods: []AuthMethodOption{
			{
				ID:          "custom_openai",
				Label:       "OpenAI-compatible Endpoint",
				Description: "Manually specify base URL and API key environment variable",
				Type:        "openai",
				BaseURL:     "http://localhost:8000/v1",
				DefaultEnv:  "",
				NeedsInput:  true,
			},
			{
				ID:          "custom_anthropic",
				Label:       "Anthropic-compatible Endpoint",
				Description: "Manually specify Anthropic /v1/messages URL and API key",
				Type:        "anthropic",
				BaseURL:     "https://api.anthropic.com/v1",
				DefaultEnv:  "",
				NeedsInput:  true,
			},
		},
	},
}

type AgentTemplate struct {
	Title       string
	Description string
	Alias       string
	Role        string
	Model       string
	ToolCalling bool
}

var agentTemplates = []AgentTemplate{
	{
		Title:       "Master Planner (Sonnet / Opus)",
		Description: "Control Plane Ringmaster agent that analyzes briefs and breaks tasks into DAG",
		Alias:       "master",
		Role:        "master",
		Model:       "sonnet",
		ToolCalling: false,
	},
	{
		Title:       "Worker: Fast Coder (Haiku / Mini)",
		Description: "Fast, token-efficient worker for scaffold and smaller edits",
		Alias:       "coder-fast",
		Role:        "worker",
		Model:       "haiku",
		ToolCalling: true,
	},
	{
		Title:       "Worker: Local / Cheap Coder (Qwen / DeepSeek)",
		Description: "Cheap or local worker executing code within isolated worktrees",
		Alias:       "coder-cheap",
		Role:        "worker",
		Model:       "qwen2.5-coder:7b",
		ToolCalling: true,
	},
	{
		Title:       "Custom Agent",
		Description: "Manually specify alias, provider binding, model name, and role",
		Alias:       "",
		Role:        "worker",
		Model:       "",
		ToolCalling: true,
	},
}
