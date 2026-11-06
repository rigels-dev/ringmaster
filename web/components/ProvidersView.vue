<template>
      <section class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">AI Providers & Model Fleet Registry</h2>
            <p class="text-xs text-zinc-400">Configure LLM providers, assign Master Planner vs Worker roles, and test connections.</p>
          </div>
          <span class="text-[10px] text-zinc-400 bg-slate-panel px-3 py-1.5 rounded border border-steel">
            Config File: <code class="text-gear-gold">{{ providersData.configPath || 'ringmaster.toml' }}</code>
          </span>
        </div>

        <!-- Section 1: Registered Providers -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
          <!-- Add Provider Card Form -->
          <div class="bg-slate-panel border border-steel rounded-lg p-5 flex flex-col gap-4">
            <h3 class="text-xs font-bold text-gear-gold uppercase tracking-wider">1. Add / Edit Provider</h3>

            <div class="space-y-3">
              <div>
                <label class="block text-[10px] text-zinc-400 mb-1">Provider ID / Name</label>
                <input 
                  v-model="newProv.id" 
                  type="text" 
                  placeholder="e.g. anthropic, 9router, ollama" 
                  class="w-full bg-slate-inner border border-steel text-parchment text-xs px-3 py-2 rounded focus:outline-none focus:border-gear-gold"
                />
              </div>

              <div>
                <label class="block text-[10px] text-zinc-400 mb-1">Provider Type</label>
                <select 
                  v-model="newProv.type" 
                  @change="handleTypeChange"
                  class="w-full bg-slate-inner border border-steel text-parchment text-xs px-3 py-2 rounded focus:outline-none focus:border-gear-gold"
                >
                  <option value="openai">OpenAI Compatible (OpenAI, 9router, Ollama, OpenRouter)</option>
                  <option value="anthropic">Claude API (Anthropic Messages API)</option>
                  <option value="claude_subscription">Claude Subscription (Local Claude CLI / OAuth)</option>
                </select>
              </div>

              <div>
                <label class="block text-[10px] text-zinc-400 mb-1">Base URL</label>
                <input 
                  v-model="newProv.base_url" 
                  :disabled="newProv.type === 'claude_subscription'"
                  type="text" 
                  :placeholder="newProv.type === 'claude_subscription' ? '(Not needed - uses local Claude CLI)' : 'http://localhost:20128/v1 or https://api.openai.com/v1'" 
                  class="w-full bg-slate-inner border border-steel text-parchment text-xs px-3 py-2 rounded focus:outline-none focus:border-gear-gold disabled:opacity-40 disabled:cursor-not-allowed"
                />
              </div>

              <div>
                <label class="block text-[10px] text-zinc-400 mb-1">API Key (Env Var name or Direct Key)</label>
                <div class="relative flex items-center">
                  <input 
                    v-model="newProv.api_key_env" 
                    :disabled="newProv.type === 'claude_subscription'"
                    :type="showApiKey ? 'text' : 'password'" 
                    :placeholder="newProv.type === 'claude_subscription' ? '(Not needed - uses ~/.claude/ credentials)' : 'e.g. sk-xxx or OPENAI_API_KEY'" 
                    class="w-full bg-slate-inner border border-steel text-parchment text-xs px-3 py-2 pr-9 rounded focus:outline-none focus:border-gear-gold disabled:opacity-40 disabled:cursor-not-allowed tracking-widest font-mono"
                  />
                  <button 
                    type="button" 
                    @click="showApiKey = !showApiKey"
                    class="absolute right-2.5 text-xs text-zinc-500 hover:text-zinc-300 transition"
                    title="Toggle key visibility"
                  >
                    {{ showApiKey ? '👁️' : '🔒' }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Ping status feedback -->
            <div v-if="pingStatus" class="p-2.5 rounded text-[11px] font-semibold" :class="pingStatus.ok ? 'bg-green-950/60 text-green-400 border border-green-800' : 'bg-red-950/60 text-red-400 border border-red-800'">
              {{ pingStatus.message }}
            </div>

            <!-- Buttons -->
            <div class="flex items-center gap-2 pt-2 border-t border-steel">
              <button 
                @click="testPing" 
                :disabled="isPinging || !isProviderFormValid"
                class="flex-1 py-2 bg-slate-inner hover:bg-slate-dark text-cyan-energy border border-cyan-dim/40 rounded text-xs font-bold transition disabled:opacity-40 disabled:cursor-not-allowed"
                :title="isProviderFormValid ? 'Test connection to provider' : 'Fill in all required fields to test connection'"
              >
                {{ isPinging ? '⏳ Testing...' : '⚡ Test Connection' }}
              </button>
              <button 
                @click="saveProvider" 
                :disabled="!isProviderFormValid"
                class="flex-1 py-2 bg-gear-gold hover:bg-gear-glow text-ink rounded text-xs font-bold transition disabled:opacity-40 disabled:cursor-not-allowed"
                :title="isProviderFormValid ? 'Save provider configuration' : 'Fill in all required fields to save'"
              >
                💾 Save Provider
              </button>
            </div>
          </div>

          <!-- Registered Providers Table -->
          <div class="lg:col-span-2 bg-slate-panel border border-steel rounded-lg p-5 flex flex-col">
            <h3 class="text-xs font-bold text-gear-gold uppercase tracking-wider mb-4">Active Providers in Config</h3>
            
            <div class="flex-1 overflow-x-auto">
              <table class="w-full text-left text-xs">
                <thead>
                  <tr class="border-b border-steel text-[10px] text-zinc-400 uppercase">
                    <th class="pb-2">ID</th>
                    <th class="pb-2">Type</th>
                    <th class="pb-2">Endpoint</th>
                    <th class="pb-2">API Key</th>
                    <th class="pb-2 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-steel/50">
                  <tr v-for="p in providersData.providers" :key="p.id" class="hover:bg-slate-dark/50 transition">
                    <td class="py-2.5 font-bold text-gear-gold">{{ p.id }}</td>
                    <td class="py-2.5 text-zinc-300">{{ p.type }}</td>
                    <td class="py-2.5 text-zinc-400 text-[10px] max-w-[160px] truncate">{{ p.base_url || '(local session)' }}</td>
                    <td class="py-2.5 text-zinc-400 font-mono text-[10px] tracking-wider">{{ maskApiKey(p.api_key_env) }}</td>
                    <td class="py-2.5 text-right space-x-2">
                      <button @click="pingExisting(p)" class="px-2 py-0.5 rounded text-[10px] bg-slate-inner text-cyan-energy hover:border-cyan-dim border border-steel">
                        ⚡ Ping
                      </button>
                      <button @click="deleteProvider(p.id)" class="px-2 py-0.5 rounded text-[10px] bg-red-950 text-red-400 hover:bg-red-900 border border-red-800">
                        Delete
                      </button>
                    </td>
                  </tr>
                  <tr v-if="!providersData.providers || providersData.providers.length === 0">
                    <td colspan="5" class="py-6 text-center text-zinc-500 italic">No providers saved yet. Register one on the left!</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <!-- Section 2: Agent Fleet & Role Assignment (Master vs Automaton) -->
        <div class="bg-slate-panel border border-steel rounded-lg p-5">
          <div class="flex items-center justify-between mb-4 pb-3 border-b border-steel">
            <div>
              <h3 class="text-xs font-bold text-gear-gold uppercase tracking-wider">2. Model Fleet & Role Assignments</h3>
              <p class="text-[10px] text-zinc-400 mt-0.5">Designate models as Master Planner (Puppeteer) or Automaton (Transformed Worker).</p>
            </div>
            <button 
              @click="showAddAgentModal = true" 
              class="px-3 py-1.5 bg-cyan-dim hover:bg-cyan-energy text-ink font-bold text-xs rounded transition flex items-center gap-1.5"
            >
              <span>+ Register New Agent</span>
            </button>
          </div>

          <!-- Add Agent Form inline / expandable -->
          <div v-if="showAddAgentModal" class="mb-5 p-4 bg-slate-inner border border-gear-dark/40 rounded-lg space-y-3">
            <h4 class="text-xs font-bold text-parchment">Register Model into Agent Fleet</h4>
            <div class="grid grid-cols-4 gap-3">
              <div>
                <label class="block text-[9px] text-zinc-400 mb-1">Agent Alias / ID</label>
                <input v-model="newAgent.id" type="text" placeholder="e.g. planner-gemini, coder-flash" class="w-full bg-slate-panel border border-steel text-xs px-2.5 py-1.5 rounded text-parchment focus:outline-none focus:border-gear-gold" />
              </div>
              <div>
                <label class="block text-[9px] text-zinc-400 mb-1">Provider</label>
                <select v-model="newAgent.provider" @change="fetchProviderModels" class="w-full bg-slate-panel border border-steel text-xs px-2.5 py-1.5 rounded text-parchment focus:outline-none focus:border-gear-gold">
                  <option v-for="p in providersData.providers" :key="p.id" :value="p.id">{{ p.id }} ({{ p.type }})</option>
                </select>
              </div>
              <div>
                <label class="block text-[9px] text-zinc-400 mb-1">
                  Model Name 
                  <span v-if="isLoadingModels" class="text-cyan-energy text-[8px] animate-pulse">Loading models...</span>
                </label>
                <!-- If models fetched from provider, show dropdown select, else fallback to text input -->
                <div v-if="fetchedModelList.length > 0">
                  <select v-model="newAgent.model" class="w-full bg-slate-panel border border-steel text-xs px-2.5 py-1.5 rounded text-parchment focus:outline-none focus:border-gear-gold">
                    <option value="" disabled>-- Select a fetched model --</option>
                    <option v-for="m in fetchedModelList" :key="m.id" :value="m.id">{{ m.name || m.id }}</option>
                  </select>
                </div>
                <div v-else>
                  <input v-model="newAgent.model" type="text" placeholder="e.g. ag/gemini-3.8-flash, gpt-4o" class="w-full bg-slate-panel border border-steel text-xs px-2.5 py-1.5 rounded text-parchment focus:outline-none focus:border-gear-gold" />
                </div>
              </div>
              <div>
                <label class="block text-[9px] text-zinc-400 mb-1">Role</label>
                <select v-model="newAgent.role" class="w-full bg-slate-panel border border-steel text-xs px-2.5 py-1.5 rounded text-parchment focus:outline-none focus:border-gear-gold">
                  <option value="worker">Automaton (Transformed Worker, Tool Calling)</option>
                  <option value="master">Master Planner (Puppeteer)</option>
                </select>
              </div>
            </div>
            <div class="flex items-center justify-between pt-2">
              <span class="text-[9px] text-zinc-500">
                <span v-if="fetchedModelList.length > 0" class="text-green-400">✓ {{ fetchedModelList.length }} models loaded from provider</span>
                <span v-else>Select provider to automatically pull available models list</span>
              </span>
              <div class="flex items-center gap-2">
                <!-- Ping Model Test Button -->
                <button 
                  @click="pingNewAgentModel" 
                  :disabled="isPingingNewAgent || !newAgent.provider || !newAgent.model"
                  type="button"
                  class="px-3 py-1 bg-slate-panel hover:bg-slate-dark text-cyan-energy border border-cyan-dim/50 text-xs rounded transition flex items-center gap-1.5 disabled:opacity-40 disabled:cursor-not-allowed"
                  title="Test if this model responds to live chat ping before saving"
                >
                  <svg v-if="isPingingNewAgent" class="w-3 h-3 animate-spin text-cyan-energy" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                    <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"></path>
                  </svg>
                  <span v-else>⚡</span>
                  <span>{{ isPingingNewAgent ? 'Testing Model...' : 'Ping Model' }}</span>
                </button>
                <button @click="showAddAgentModal = false" class="px-3 py-1 bg-slate-panel hover:bg-slate-dark text-zinc-400 text-xs rounded border border-steel">Cancel</button>
                <button @click="saveNewAgent" class="px-4 py-1 bg-gear-gold hover:bg-gear-glow text-ink font-bold text-xs rounded">Save Agent</button>
              </div>
            </div>
          </div>

          <!-- Agents Fleet Table -->
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead>
                <tr class="border-b border-steel text-[10px] text-zinc-400 uppercase">
                  <th class="pb-2">Status</th>
                  <th class="pb-2">Agent Alias</th>
                  <th class="pb-2">Model</th>
                  <th class="pb-2">Provider</th>
                  <th class="pb-2">Current Role</th>
                  <th class="pb-2">Tool Calling</th>
                  <th class="pb-2 text-right">Switch Role / Action</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-steel/50">
                <tr 
                  v-for="a in providersData.agents" 
                  :key="a.id" 
                  class="transition"
                  :class="a.disabled ? 'bg-slate-panel/20 opacity-50' : 'hover:bg-slate-dark/50'"
                >
                  <!-- Active / Inactive Status Toggle -->
                  <td class="py-3">
                    <button 
                      @click="toggleAgentStatus(a.id)"
                      class="px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-wider border transition flex items-center gap-1"
                      :class="a.disabled ? 'bg-slate-dark text-zinc-500 border-steel hover:text-zinc-300' : 'bg-green-950/80 text-green-400 border-green-800 hover:bg-green-900'"
                      :title="a.disabled ? 'Click to Activate this agent' : 'Click to Deactivate (suspend) this agent'"
                    >
                      <span class="w-1.5 h-1.5 rounded-full" :class="a.disabled ? 'bg-zinc-600' : 'bg-green-400 animate-ping'"></span>
                      <span>{{ a.disabled ? 'OFF' : 'ON' }}</span>
                    </button>
                  </td>
                  <td class="py-3 font-bold text-parchment flex items-center gap-2">
                    <span v-if="a.role === 'master'" class="text-gear-gold text-base" title="Master Planner">★</span>
                    <span v-else class="text-cyan-energy text-base" title="Automaton">🤖</span>
                    <span :class="a.disabled ? 'line-through text-zinc-500' : ''">{{ a.id }}</span>
                  </td>
                  <td class="py-3 font-mono text-[11px] text-zinc-300">{{ a.model }}</td>
                  <td class="py-3 text-zinc-400">{{ a.provider }}</td>
                  <td class="py-3">
                    <span 
                      class="text-[9px] font-bold px-2 py-0.5 rounded uppercase font-mono"
                      :class="a.role === 'master' ? 'bg-gear-gold text-ink' : 'bg-slate-inner text-cyan-energy border border-cyan-dim/40'"
                    >
                      {{ a.role === 'master' ? 'MASTER' : 'AUTOMATON' }}
                    </span>
                  </td>
                  <td class="py-3">
                    <span class="text-[10px] font-bold" :class="a.tool_calling ? 'text-green-400' : 'text-zinc-500'">
                      {{ a.tool_calling ? 'YES' : 'NO' }}
                    </span>
                  </td>
                  <td class="py-3 text-right space-x-1.5 whitespace-nowrap">
                    <!-- Agent Ping Test Button -->
                    <button 
                      @click="pingAgent(a)" 
                      :disabled="agentPingStates[a.id]?.loading"
                      class="px-2 py-0.5 rounded text-[10px] bg-slate-inner hover:border-cyan-dim text-cyan-energy border border-steel transition disabled:opacity-50"
                      title="Test Chat Ping with this Agent's Model"
                    >
                      <span v-if="agentPingStates[a.id]?.loading">⏳ Pinging...</span>
                      <span v-else>⚡ Ping</span>
                    </button>
                    <!-- Quick Set Master button -->
                    <button 
                      v-if="a.role !== 'master'" 
                      @click="setAgentRole(a.id, 'master')" 
                      class="px-2 py-0.5 rounded text-[10px] bg-gear-dark/40 hover:bg-gear-gold hover:text-ink text-gear-gold border border-gear-dark transition"
                      title="Promote to Master Planner"
                    >
                      ★ Set Master
                    </button>
                    <!-- Quick Set Automaton button -->
                    <button 
                      v-if="a.role !== 'worker'" 
                      @click="setAgentRole(a.id, 'worker')" 
                      class="px-2 py-0.5 rounded text-[10px] bg-slate-inner hover:bg-cyan-energy hover:text-ink text-cyan-energy border border-steel transition"
                      title="Designate as Automaton"
                    >
                      🤖 Set Automaton
                    </button>
                    <!-- Delete Agent -->
                    <button 
                      @click="deleteAgent(a.id)" 
                      class="px-2 py-0.5 rounded text-[10px] bg-red-950/60 hover:bg-red-900 text-red-400 border border-red-800 transition"
                      title="Delete Agent"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
                <tr v-if="!providersData.agents || providersData.agents.length === 0">
                  <td colspan="6" class="py-6 text-center text-zinc-500 italic">No agents registered in fleet yet. Add a model above!</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
</template>

<script setup>
const {
  providersData,
  newProv,
  isProviderFormValid,
  pingStatus,
  isPinging,
  showApiKey,
  showAddAgentModal,
  isLoadingModels,
  fetchedModelList,
  agentPingStates,
  isPingingNewAgent,
  newAgent,
  fetchProviderModels,
  maskApiKey,
  saveNewAgent,
  toggleAgentStatus,
  setAgentRole,
  deleteAgent,
  handleTypeChange,
  testPing,
  pingAgent,
  pingNewAgentModel,
  pingExisting,
  saveProvider,
  deleteProvider
} = useRingmaster()
</script>
