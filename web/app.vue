<template>
  <div class="flex h-screen w-screen overflow-hidden bg-slate-deep text-parchment font-mono select-none">
    <!-- Left Navigation Sidemenu -->
    <aside class="w-64 flex flex-col bg-slate-dark border-r border-steel z-20">
      <!-- Brand Header -->
      <div class="h-14 flex items-center gap-3 px-4 border-b border-steel bg-slate-deep">
        <img src="/icons/ringmaster-icon-transparent.svg" alt="Ringmaster Icon" class="w-7 h-7 object-contain drop-shadow-[0_0_8px_rgba(222,170,59,0.5)]" />
        <div>
          <h1 class="font-cinzel font-black tracking-wider text-gear-gold text-sm">RINGMASTER</h1>
          <p class="text-[9px] text-zinc-400 tracking-tight font-outfit uppercase">Agent Control Plane</p>
        </div>
      </div>

      <!-- Navigation Links -->
      <nav class="flex-1 p-3 space-y-1.5 overflow-y-auto">
        <button
          v-for="item in navItems"
          :key="item.id"
          @click="activeView = item.id"
          class="w-full flex items-center justify-between px-3 py-2.5 rounded text-xs font-semibold transition-all"
          :class="activeView === item.id 
            ? 'bg-slate-panel text-gear-gold border border-gear-dark/40 shadow-sm shadow-gear-dark/20' 
            : 'text-zinc-400 hover:text-zinc-200 hover:bg-slate-panel/50'"
        >
          <div class="flex items-center gap-3">
            <span class="text-base">{{ item.icon }}</span>
            <span>{{ item.label }}</span>
          </div>
          <span v-if="item.badge && !project.isBlank" class="px-1.5 py-0.5 text-[9px] rounded font-bold" :class="item.badgeClass">
            {{ item.badge }}
          </span>
        </button>
      </nav>

      <!-- Active Project Info & Path (Dynamic!) -->
      <div class="p-3 border-t border-steel bg-slate-inner/80">
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Project Name</span>
          <span class="text-gear-gold font-bold truncate max-w-[120px]" :title="project.name">{{ project.name || 'Loading...' }}</span>
        </div>
        <div class="text-[9px] text-zinc-500 mb-2 truncate" :title="project.path">
          <span class="text-zinc-400">Path:</span> <code class="text-zinc-400">{{ project.path || '-' }}</code>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Master Planner</span>
          <span class="text-cyan-energy font-semibold">{{ project.master || 'default' }}</span>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400">
          <span>Active Workers</span>
          <span :class="activeWorkersList.length > 0 ? 'text-green-400 font-bold' : 'text-zinc-500'">
            {{ activeWorkersList.length > 0 ? activeWorkersList.length + ' Running' : '0 Idle' }}
          </span>
        </div>
      </div>
    </aside>

    <!-- Global Floating Toast Notification System (Replaces browser alert) -->
    <div class="fixed bottom-5 right-5 z-50 flex flex-col gap-2 pointer-events-none max-w-sm">
      <div 
        v-for="toast in toastList" 
        :key="toast.id"
        class="p-4 rounded-xl shadow-2xl border backdrop-blur-md font-mono text-xs pointer-events-auto transition-all animate-bounce"
        :class="{
          'bg-slate-dark/95 border-green-500/50 text-parchment shadow-green-500/10': toast.type === 'success',
          'bg-slate-dark/95 border-red-500/50 text-parchment shadow-red-500/10': toast.type === 'error',
          'bg-slate-dark/95 border-gear-gold/50 text-parchment shadow-gear-gold/10': toast.type === 'info'
        }"
      >
        <div class="flex items-start gap-3">
          <span class="text-base" v-if="toast.type === 'success'">✓</span>
          <span class="text-base" v-else-if="toast.type === 'error'">✗</span>
          <span class="text-base" v-else>ℹ</span>
          <div class="flex-1">
            <h4 class="font-bold text-xs mb-1" :class="toast.type === 'success' ? 'text-green-400' : (toast.type === 'error' ? 'text-red-400' : 'text-gear-gold')">
              {{ toast.title }}
            </h4>
            <p class="text-[11px] text-zinc-300 leading-relaxed whitespace-pre-wrap">{{ toast.message }}</p>
            <div v-if="toast.meta" class="mt-2 pt-2 border-t border-steel/60 flex items-center justify-between text-[10px] text-zinc-500">
              <span>Latency: <strong class="text-cyan-energy">{{ toast.meta.latency }}ms</strong></span>
              <span v-if="toast.meta.tokens">Tokens: <strong class="text-zinc-300">{{ toast.meta.tokens }}</strong></span>
            </div>
          </div>
          <button @click="dismissToast(toast.id)" class="text-zinc-500 hover:text-zinc-300 text-xs">✕</button>
        </div>
      </div>
    </div>

    <!-- Main Content Workspace Area -->
    <main class="flex-1 flex flex-col h-full bg-slate-deep overflow-hidden">
      <!-- 1. CHAT VIEW (Blank on init until user starts) -->
      <section v-if="activeView === 'chat'" class="flex-1 flex h-full overflow-hidden">
        <!-- Dialogue Stream -->
        <div class="flex-1 flex flex-col h-full border-r border-steel">
          <!-- Top Header -->
          <div class="h-14 flex items-center justify-between px-6 border-b border-steel bg-slate-panel">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-bold text-gear-gold font-cinzel">Mission Control & Agent Stream</h2>
                <span class="w-2 h-2 rounded-full" :class="project.isBlank ? 'bg-amber-400' : 'bg-green-500 animate-pulse'"></span>
              </div>
              <p class="text-[10px] text-zinc-400">
                {{ project.isBlank ? 'Diskusikan ide proyek dengan Master Planner untuk mematangkan BRIEF.md.' : 'Live mission control dan komunikasi dengan multi-agent fleet.' }}
              </p>
            </div>
            
            <button 
              v-if="!project.isBlank"
              @click="activeView = 'tasks'" 
              class="px-3 py-1.5 bg-slate-inner hover:bg-slate-panel text-zinc-300 text-xs rounded border border-steel flex items-center gap-1.5 transition"
            >
              <span>View Tasks Pipeline ({{ tasks.length }})</span>
              <span>▸</span>
            </button>
          </div>

          <!-- Messages Stream -->
          <div class="flex-1 p-6 overflow-y-auto space-y-4">
            <!-- Welcome Hero Animated SVG when chat is clean/blank -->
            <div v-if="messages.length <= 1" class="flex flex-col items-center justify-center py-6 text-center select-none">
              <img src="/icons/ringmaster-animated.svg" alt="Ringmaster Kinematics" class="w-44 h-44 object-contain drop-shadow-[0_0_25px_rgba(222,170,59,0.35)] mb-3" />
              <h3 class="font-cinzel text-base font-bold text-gear-gold tracking-wide">The Stage is Ready</h3>
              <p class="text-xs text-zinc-400 max-w-md mt-1">Diskusikan kebutuhan proyek dengan Master Planner untuk mulai merancang arsitektur dan mengorkestrasi agen pekerja.</p>
            </div>

            <div 
              v-for="(msg, i) in messages" 
              :key="i"
              class="flex flex-col max-w-2xl"
              :class="msg.role === 'user' ? 'ml-auto items-end' : 'mr-auto items-start'"
            >
              <!-- Role / Origin Badge -->
              <div class="flex items-center gap-1.5 mb-1 text-[9px] uppercase tracking-wider">
                <span v-if="msg.role === 'user'" class="text-zinc-400 font-bold">You (Reviewer) ➔ To: {{ msg.targetLabel }}</span>
                <span v-else-if="msg.role === 'master'" class="text-gear-gold font-bold">🎪 Master Planner</span>
                <span v-else-if="msg.role === 'worker'" class="text-cyan-energy font-bold">⚡ Worker: {{ msg.workerId }} · Task {{ msg.taskId }}</span>
                <span class="text-zinc-600">· {{ msg.time }}</span>
              </div>

              <!-- Message Bubble -->
              <div 
                class="p-4 rounded-lg text-xs leading-relaxed"
                :class="{
                  'bg-slate-panel text-parchment border border-steel': msg.role === 'user',
                  'bg-slate-dark text-parchment border border-gear-dark/40 shadow-sm': msg.role === 'master',
                  'bg-slate-inner text-parchment border border-cyan-dim/40 shadow-sm': msg.role === 'worker' && msg.type !== 'error',
                  'bg-coat-dim/30 text-parchment border border-coat-crimson shadow-sm': msg.type === 'error'
                }"
              >
                <!-- Worker Report Card Header -->
                <div v-if="msg.role === 'worker'" class="flex items-center justify-between pb-2 mb-2 border-b border-steel/50">
                  <span class="text-[10px] font-bold tracking-wider" :class="msg.type === 'error' ? 'text-coat-bright' : 'text-cyan-energy'">
                    {{ msg.statusTitle }}
                  </span>
                  <span class="text-[9px] px-1.5 py-0.5 rounded bg-slate-panel text-zinc-400">
                    {{ msg.taskBranch }}
                  </span>
                </div>

                <p class="whitespace-pre-wrap">{{ msg.text }}</p>

                <!-- Actionable Buttons if worker needs review -->
                <div v-if="msg.actionable" class="mt-3 pt-3 border-t border-steel/60 flex items-center gap-2">
                  <button 
                    @click="instructWorker(msg, 'approve')" 
                    class="px-2.5 py-1 bg-green-950 hover:bg-green-900 text-green-400 border border-green-800 rounded text-[10px] font-bold transition"
                  >
                    ✓ Approve & Continue
                  </button>
                  <button 
                    @click="instructWorker(msg, 'direct')" 
                    class="px-2.5 py-1 bg-slate-dark hover:bg-slate-panel text-cyan-energy border border-steel rounded text-[10px] font-semibold transition"
                  >
                    💬 Direct {{ msg.workerId }}
                  </button>
                  <button 
                    @click="instructWorker(msg, 'via_master')" 
                    class="px-2.5 py-1 bg-gear-dark/30 hover:bg-gear-dark/50 text-gear-gold border border-gear-dark rounded text-[10px] font-semibold transition"
                  >
                    🎪 Escalate to Master
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Input Bar with Dynamic Target Selector -->
          <div class="p-4 border-t border-steel bg-slate-panel">
            <div class="flex items-center gap-2 mb-2 overflow-x-auto pb-1">
              <span class="text-[10px] text-zinc-400 font-semibold whitespace-nowrap">Target Agent:</span>
              
              <!-- Master Option -->
              <button 
                @click="selectedTarget = 'master'"
                class="px-2.5 py-1 rounded text-[10px] font-bold transition flex items-center gap-1 whitespace-nowrap"
                :class="selectedTarget === 'master' ? 'bg-gear-gold text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
              >
                <span>🎪 Master Planner</span>
              </button>

              <!-- Dynamic Active Worker Options -->
              <button 
                v-for="worker in activeWorkersList"
                :key="worker.id"
                @click="selectedTarget = worker.id"
                class="px-2.5 py-1 rounded text-[10px] font-bold transition flex items-center gap-1.5 whitespace-nowrap"
                :class="selectedTarget === worker.id ? 'bg-cyan-energy text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
              >
                <span class="w-1.5 h-1.5 rounded-full bg-green-400 animate-ping"></span>
                <span>⚡ {{ worker.name }} (Task {{ worker.taskId }})</span>
              </button>
            </div>

            <!-- Input Form -->
            <form @submit.prevent="sendMessage" class="flex items-center gap-3">
              <input 
                v-model="inputMsg"
                type="text" 
                :placeholder="selectedTarget === 'master' 
                  ? 'Kirim pesan ke Master Planner (misal: fitur apa yang ingin dibuat di ' + (project.name || 'project') + ')...' 
                  : `Kirim instruksi langsung ke worker '${selectedTarget}'...`"
                class="flex-1 bg-slate-inner border border-steel text-parchment text-xs px-4 py-2.5 rounded focus:outline-none focus:border-gear-gold"
              />
              <button 
                type="submit" 
                class="px-5 py-2.5 bg-cyan-dim hover:bg-cyan-energy text-slate-deep font-bold text-xs rounded transition flex items-center gap-2"
              >
                <span>Send</span>
                <span>↑</span>
              </button>
            </form>
          </div>
        </div>

        <!-- Right Panel: Context Sidebar -->
        <div class="w-80 flex flex-col h-full bg-slate-panel/40">
          <div class="h-14 flex items-center justify-between px-4 border-b border-steel bg-slate-panel">
            <span class="text-xs font-bold text-zinc-300">Project Context</span>
            <span class="text-[9px] px-2 py-0.5 rounded bg-slate-inner text-gear-gold border border-steel">
              {{ project.isBlank ? 'Awaiting Plan' : 'Active Plan' }}
            </span>
          </div>

          <div class="flex-1 p-4 overflow-y-auto space-y-4">
            <!-- Parallel Workers (Show if active) -->
            <div v-if="activeWorkersList.length > 0" class="space-y-2">
              <span class="text-[10px] uppercase font-bold text-zinc-400">Active Worktrees</span>
              <div 
                v-for="worker in activeWorkersList"
                :key="worker.id"
                class="bg-slate-dark border border-steel rounded p-3 hover:border-cyan-dim transition cursor-pointer"
                @click="selectedTarget = worker.id"
              >
                <div class="flex items-center justify-between mb-1">
                  <span class="text-[10px] text-zinc-300 font-bold">{{ worker.name }}</span>
                  <span class="text-[9px] font-bold px-1.5 py-0.2 rounded uppercase bg-amber-950 text-amber-400 border border-amber-800">
                    {{ worker.status }}
                  </span>
                </div>
                <p class="text-xs font-semibold text-zinc-200">{{ worker.taskId }}: {{ worker.taskTitle }}</p>
                <div class="flex items-center justify-between text-[9px] text-zinc-400 mt-2">
                  <span>Branch: <code class="text-cyan-energy">rm/{{ worker.taskId }}</code></span>
                  <span>Turn: {{ worker.turn }}/30</span>
                </div>
              </div>
            </div>

            <div v-else class="p-3 bg-slate-dark/60 border border-steel/60 rounded text-center">
              <p class="text-[11px] text-zinc-400">Belum ada task berjalan.</p>
              <p class="text-[9px] text-zinc-500 mt-1">Diskusikan requirement di chat untuk menyusun task.</p>
            </div>

            <!-- BRIEF.md Preview -->
            <div class="pt-2 border-t border-steel">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-zinc-300">BRIEF.md</span>
                <span class="text-[9px] text-zinc-500">{{ project.brief ? 'Found' : 'Empty' }}</span>
              </div>
              <div class="p-3 bg-slate-inner border border-steel rounded font-mono text-[10px] leading-relaxed text-zinc-400 whitespace-pre-wrap max-h-56 overflow-y-auto">
{{ project.brief || '(BRIEF.md belum diisi. Kirim pesan ke Master Planner untuk menyusunnya otomatis.)' }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. TASKS VIEW -->
      <section v-else-if="activeView === 'tasks'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">Tasks & Worktree Pipeline</h2>
            <p class="text-xs text-zinc-400">Visual progress of isolated git worktrees, acceptance testing, and merge queue.</p>
          </div>
          <div class="flex items-center gap-3">
            <button 
              @click="activeView = 'chat'" 
              class="px-3.5 py-2 bg-slate-panel hover:bg-slate-panel/70 text-zinc-300 font-bold text-xs rounded border border-steel transition"
            >
              💬 Open Chat Channel
            </button>
            <button 
              v-if="tasks.length > 0"
              @click="crackWhip"
              class="px-4 py-2 bg-coat-bright hover:bg-coat-crimson text-white font-bold text-xs rounded shadow flex items-center gap-2 transition"
            >
              <span>⚡ CRACK WHIP (TAME WORKERS)</span>
            </button>
          </div>
        </div>

        <!-- If Blank Project -->
        <div v-if="tasks.length === 0" class="flex-1 flex flex-col items-center justify-center p-12 bg-slate-panel/30 border border-dashed border-steel rounded-lg text-center">
          <img src="/icons/ringmaster-animated.svg" alt="Ringmaster Kinematics" class="w-36 h-36 object-contain drop-shadow-[0_0_20px_rgba(61,245,240,0.35)] mb-4" />
          <h3 class="text-sm font-bold text-gear-gold mb-1">Belum Ada Task yang Terdaftar</h3>
          <p class="text-xs text-zinc-400 max-w-md mb-4">
            Proyek ini baru di-init. Buka menu Chat untuk berdiskusi dengan Master Planner atau jalankan <code class="text-gear-gold">ringmaster plan</code> di terminal untuk membuat DAG task.
          </p>
          <button @click="activeView = 'chat'" class="px-4 py-2 bg-gear-gold text-slate-deep font-bold text-xs rounded">
            Mulai Diskusi di Chat ▸
          </button>
        </div>

        <!-- Task Cards Grid -->
        <div v-else class="grid grid-cols-3 gap-4">
          <div 
            v-for="task in tasks" 
            :key="task.id"
            class="bg-slate-panel border border-steel p-4 rounded-lg flex flex-col justify-between hover:border-gear-gold transition"
          >
            <div>
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-gear-gold">{{ task.id }}</span>
                <span 
                  class="text-[9px] px-2 py-0.5 rounded font-bold uppercase"
                  :class="{
                    'bg-green-950 text-green-400 border border-green-800': task.status === 'DONE',
                    'bg-amber-950 text-amber-400 border border-amber-800': task.status === 'RUNNING',
                    'bg-red-950 text-red-400 border border-red-800': task.status === 'BLOCKED',
                    'bg-slate-inner text-zinc-400 border border-steel': task.status === 'PENDING'
                  }"
                >
                  {{ task.status }}
                </span>
              </div>
              <h3 class="text-xs font-semibold text-zinc-200 mb-1">{{ task.title }}</h3>
              <p class="text-[10px] text-zinc-400 mb-3">{{ task.desc }}</p>
            </div>
            <div class="flex items-center justify-between text-[9px] text-zinc-500 border-t border-steel pt-2">
              <span>Worker: <strong class="text-cyan-energy">{{ task.worker }}</strong></span>
              <span>Retries: {{ task.retries }}</span>
            </div>
          </div>
        </div>
      </section>

      <!-- 3. PROVIDERS & MODELS / AGENTS CONFIG -->
      <section v-else-if="activeView === 'providers'" class="flex-1 flex flex-col p-6 overflow-y-auto">
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
                :disabled="isPinging"
                class="flex-1 py-2 bg-slate-inner hover:bg-slate-dark text-cyan-energy border border-cyan-dim/40 rounded text-xs font-bold transition disabled:opacity-50"
              >
                {{ isPinging ? '⏳ Testing...' : '⚡ Test Connection' }}
              </button>
              <button 
                @click="saveProvider" 
                class="flex-1 py-2 bg-gear-gold hover:bg-gear-glow text-slate-deep rounded text-xs font-bold transition"
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
                    <td colspan="5" class="py-6 text-center text-zinc-500 italic">Belum ada provider yang disimpan.</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <!-- Section 2: Agent Fleet & Role Assignment (Master vs Worker) -->
        <div class="bg-slate-panel border border-steel rounded-lg p-5">
          <div class="flex items-center justify-between mb-4 pb-3 border-b border-steel">
            <div>
              <h3 class="text-xs font-bold text-gear-gold uppercase tracking-wider">2. Model Fleet & Role Assignments</h3>
              <p class="text-[10px] text-zinc-400 mt-0.5">Tentukan model mana yang menjadi Master Planner (Arsitek) dan Worker (Coder/Executor).</p>
            </div>
            <button 
              @click="showAddAgentModal = true" 
              class="px-3 py-1.5 bg-cyan-dim hover:bg-cyan-energy text-slate-deep font-bold text-xs rounded transition flex items-center gap-1.5"
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
                  <option value="worker">Worker (Coder, Tool Calling)</option>
                  <option value="master">Master Planner (Architect)</option>
                </select>
              </div>
            </div>
            <div class="flex items-center justify-between pt-2">
              <span class="text-[9px] text-zinc-500">
                <span v-if="fetchedModelList.length > 0" class="text-green-400">✓ {{ fetchedModelList.length }} models loaded from provider</span>
                <span v-else>Select provider to automatically pull available models list</span>
              </span>
              <div class="flex items-center gap-2">
                <button @click="showAddAgentModal = false" class="px-3 py-1 bg-slate-panel hover:bg-slate-dark text-zinc-400 text-xs rounded border border-steel">Cancel</button>
                <button @click="saveNewAgent" class="px-4 py-1 bg-gear-gold hover:bg-gear-glow text-slate-deep font-bold text-xs rounded">Save Agent</button>
              </div>
            </div>
          </div>

          <!-- Agents Fleet Table -->
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs">
              <thead>
                <tr class="border-b border-steel text-[10px] text-zinc-400 uppercase">
                  <th class="pb-2">Agent Alias</th>
                  <th class="pb-2">Model</th>
                  <th class="pb-2">Provider</th>
                  <th class="pb-2">Current Role</th>
                  <th class="pb-2">Tool Calling</th>
                  <th class="pb-2 text-right">Switch Role / Action</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-steel/50">
                <tr v-for="a in providersData.agents" :key="a.id" class="hover:bg-slate-dark/50 transition">
                  <td class="py-3 font-bold text-parchment flex items-center gap-2">
                    <span v-if="a.role === 'master'" class="text-gear-gold" title="Master Planner">★</span>
                    <span v-else class="text-cyan-energy" title="Worker">⚡</span>
                    <span>{{ a.id }}</span>
                    <span v-if="providersData.defaultMaster === a.id" class="text-[8px] bg-gear-dark/40 text-gear-gold px-1 rounded border border-gear-dark">DEFAULT MASTER</span>
                  </td>
                  <td class="py-3 font-mono text-[11px] text-zinc-300">{{ a.model }}</td>
                  <td class="py-3 text-zinc-400">{{ a.provider }}</td>
                  <td class="py-3">
                    <span 
                      class="text-[9px] font-bold px-2 py-0.5 rounded uppercase"
                      :class="a.role === 'master' ? 'bg-gear-gold text-slate-deep' : 'bg-slate-inner text-cyan-energy border border-cyan-dim/40'"
                    >
                      {{ a.role }}
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
                      class="px-2 py-0.5 rounded text-[10px] bg-gear-dark/40 hover:bg-gear-gold hover:text-slate-deep text-gear-gold border border-gear-dark transition"
                      title="Promote to Master Planner"
                    >
                      ★ Set Master
                    </button>
                    <!-- Quick Set Worker button -->
                    <button 
                      v-if="a.role !== 'worker'" 
                      @click="setAgentRole(a.id, 'worker')" 
                      class="px-2 py-0.5 rounded text-[10px] bg-slate-inner hover:bg-cyan-energy hover:text-slate-deep text-cyan-energy border border-steel transition"
                      title="Set as Worker"
                    >
                      ⚡ Set Worker
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
                  <td colspan="6" class="py-6 text-center text-zinc-500 italic">Belum ada agent yang terdaftar. Daftarkan model di atas!</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <!-- 4. ANALYTICS VIEW -->
      <section v-else-if="activeView === 'analytics'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Execution Analytics & Token Usage</h2>
          <p class="text-xs text-zinc-400">Accurate token breakdown per agent, role, and task vs estimates.</p>
        </div>

        <div class="grid grid-cols-3 gap-4 mb-6">
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Total Tokens Consumed</span>
            <p class="text-xl font-bold text-gear-gold mt-1">{{ totalTokensConsumed }}</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Project Status</span>
            <p class="text-xl font-bold text-cyan-energy mt-1">{{ project.isBlank ? 'PENDING INIT' : (project.state?.status || 'READY') }}</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Tasks Completed</span>
            <p class="text-xl font-bold text-zinc-300 mt-1">{{ completedTasksCount }} / {{ tasks.length }}</p>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const activeView = ref('chat')
const selectedTarget = ref('master')
const inputMsg = ref('')

const project = ref({
  name: '',
  path: '',
  isBlank: true,
  brief: '',
  plan: null,
  state: null,
  master: '',
  workers: []
})

const providersData = ref({
  configPath: '',
  providers: [],
  agents: []
})

const newProv = ref({
  id: '',
  type: 'openai',
  base_url: '',
  api_key_env: ''
})

const pingStatus = ref(null)
const isPinging = ref(false)
const showApiKey = ref(false)
const showAddAgentModal = ref(false)
const isLoadingModels = ref(false)
const fetchedModelList = ref([])
const agentPingStates = ref({})
const toastList = ref([])

function showToast(title, message, type = 'success', meta = null) {
  const id = Date.now() + Math.random()
  toastList.value.push({ id, title, message, type, meta })
  setTimeout(() => {
    dismissToast(id)
  }, 5000)
}

function dismissToast(id) {
  toastList.value = toastList.value.filter(t => t.id !== id)
}

const newAgent = ref({
  id: '',
  provider: '',
  model: '',
  role: 'worker'
})

async function fetchProviderModels() {
  if (!newAgent.value.provider) return
  isLoadingModels.value = true
  fetchedModelList.value = []
  try {
    const res = await fetch(`/api/models?provider=${encodeURIComponent(newAgent.value.provider)}`)
    const data = await res.json()
    if (data.ok && Array.isArray(data.models)) {
      fetchedModelList.value = data.models
      if (data.models.length > 0) {
        newAgent.value.model = data.models[0].id
      }
    }
  } catch (err) {
    console.error('Failed to fetch provider models:', err)
  } finally {
    isLoadingModels.value = false
  }
}

function maskApiKey(val) {
  if (!val) return '—'
  if (val.startsWith('sk-') || val.length > 15) {
    return '••••••••••••••••••••'
  }
  return val
}

const navItems = [
  { id: 'chat', label: 'Chat', icon: '💬', badge: '1 Alert', badgeClass: 'bg-amber-950 text-amber-400 border border-amber-800' },
  { id: 'tasks', label: 'Tasks', icon: '📋' },
  { id: 'providers', label: 'Providers', icon: '🤖' },
  { id: 'analytics', label: 'Analytics', icon: '📈' }
]

const messages = ref([])

async function loadProjectData() {
  try {
    const res = await fetch('/api/project')
    const data = await res.json()
    project.value = data

    // If blank project, initialize blank welcoming chat
    if (data.isBlank) {
      messages.value = [
        {
          role: 'master',
          targetLabel: 'You',
          text: `Halo Jal! Proyek '${data.name}' telah terhubung. Saat ini belum ada task yang disusun. Silakan tulis ide atau deskripsi fitur yang ingin kamu buat di bawah, nanti aku bantu susunkan dokumen BRIEF.md dan DAG task-nya.`,
          time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        }
      ]
    }
  } catch {}
}

async function loadProvidersData() {
  try {
    const res = await fetch('/api/providers')
    const data = await res.json()
    if (data.ok) {
      providersData.value = data
      if (data.providers && data.providers.length > 0 && !newAgent.value.provider) {
        newAgent.value.provider = data.providers[0].id
        fetchProviderModels()
      }
    }
  } catch {}
}

onMounted(() => {
  loadProjectData()
  loadProvidersData()
})

const tasks = computed(() => {
  if (!project.value.plan || !project.value.plan.tasks) return []
  return project.value.plan.tasks.map(t => {
    const ts = project.value.state?.task_states?.[t.id] || {}
    return {
      id: t.id,
      title: t.title,
      desc: t.description || 'No description',
      worker: t.worker || 'worker',
      status: ts.status || 'PENDING',
      retries: `${ts.attempt || 0}/${t.max_retries || 2}`
    }
  })
})

const activeWorkersList = computed(() => {
  if (!project.value.state || !project.value.state.task_states) return []
  const list = []
  for (const [id, ts] of Object.entries(project.value.state.task_states)) {
    if (ts.status === 'RUNNING' || ts.status === 'BLOCKED') {
      list.push({
        id: ts.worker || 'worker',
        name: ts.worker || 'worker',
        taskId: id,
        taskTitle: 'Active Task',
        status: ts.status,
        turn: ts.attempt || 1
      })
    }
  }
  return list
})

const totalTokensConsumed = computed(() => {
  if (!project.value.state) return '0'
  let total = 0
  if (project.value.state.planning) {
    total += (project.value.state.planning.tokens_in || 0) + (project.value.state.planning.tokens_out || 0)
  }
  if (project.value.state.task_states) {
    for (const ts of Object.values(project.value.state.task_states)) {
      if (ts.usage) {
        total += (ts.usage.tokens_in || 0) + (ts.usage.tokens_out || 0)
      }
    }
  }
  return total.toLocaleString()
})

const completedTasksCount = computed(() => {
  return tasks.value.filter(t => t.status === 'DONE').length
})

async function saveNewAgent() {
  if (!newAgent.value.id.trim() || !newAgent.value.model.trim()) {
    showToast('Validation Error', 'Agent alias and model name are required', 'error')
    return
  }
  try {
    const res = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        action: 'add_agent',
        ...newAgent.value,
        tool_calling: newAgent.value.role === 'worker'
      })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Agent Registered', `Agent '${newAgent.value.id}' (${newAgent.value.model}) added to fleet.`, 'success')
      showAddAgentModal.value = false
      newAgent.value.id = ''
      newAgent.value.model = ''
      loadProvidersData()
      loadProjectData()
    } else {
      showToast('Registration Error', data.error, 'error')
    }
  } catch (err) {
    showToast('Failed to register agent', err.message, 'error')
  }
}

async function setAgentRole(id, role) {
  try {
    const res = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'set_role', id, role })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Role Updated', `Agent '${id}' is now designated as ${role.toUpperCase()}.`, 'info')
      loadProvidersData()
      loadProjectData()
    } else {
      showToast('Update Failed', data.error, 'error')
    }
  } catch (err) {
    showToast('Role Switch Error', err.message, 'error')
  }
}

async function deleteAgent(id) {
  if (!confirm(`Delete agent '${id}' from fleet?`)) return
  try {
    const res = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'delete_agent', id })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Agent Removed', `Agent '${id}' removed from fleet.`, 'info')
      loadProvidersData()
      loadProjectData()
    } else {
      showToast('Delete Failed', data.error, 'error')
    }
  } catch (err) {
    showToast('Failed to delete agent', err.message, 'error')
  }
}

function handleTypeChange() {
  if (newProv.value.type === 'claude_subscription') {
    newProv.value.base_url = ''
    newProv.value.api_key_env = ''
  }
}

async function testPing() {
  isPinging.value = true
  pingStatus.value = null
  try {
    const res = await fetch('/api/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newProv.value)
    })
    const data = await res.json()
    if (data.ok) {
      pingStatus.value = { ok: true, message: `✓ Ping Succeeded (${data.latency_ms}ms) — ${data.message}` }
    } else {
      pingStatus.value = { ok: false, message: `✗ Ping Failed (${data.latency_ms || 0}ms): ${data.error}` }
    }
  } catch (err) {
    pingStatus.value = { ok: false, message: '✗ Network error: ' + err.message }
  } finally {
    isPinging.value = false
  }
}

async function pingAgent(a) {
  const p = providersData.value.providers?.find(prov => prov.id === a.provider)
  if (!p) {
    showToast('Provider Not Found', `Provider '${a.provider}' is not configured in ringmaster.toml`, 'error')
    return
  }

  agentPingStates.value[a.id] = { loading: true }
  try {
    const res = await fetch('/api/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: p.type,
        base_url: p.base_url,
        api_key_env: p.api_key_env,
        model: a.model
      })
    })
    const data = await res.json()
    if (data.ok) {
      showToast(
        `Agent '${a.id}' Connected`,
        `Model ${a.model} successfully responded to chat request.`,
        'success',
        { latency: data.latency_ms, tokens: 1 }
      )
    } else {
      showToast(
        `Agent '${a.id}' Ping Failed`,
        data.error || 'Unknown error occurred while contacting model.',
        'error',
        { latency: data.latency_ms || 0 }
      )
    }
  } catch (err) {
    showToast(`Connection Error`, err.message, 'error')
  } finally {
    agentPingStates.value[a.id] = { loading: false }
  }
}

async function pingExisting(p) {
  try {
    const res = await fetch('/api/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(p)
    })
    const data = await res.json()
    if (data.ok) {
      showToast(
        `Provider '${p.id}' Reachable`,
        data.message,
        'success',
        { latency: data.latency_ms }
      )
    } else {
      showToast(
        `Provider '${p.id}' Offline`,
        data.error,
        'error',
        { latency: data.latency_ms || 0 }
      )
    }
  } catch (err) {
    showToast(`Ping Error`, err.message, 'error')
  }
}

async function saveProvider() {
  if (!newProv.value.id.trim()) {
    showToast('Validation Error', 'Provider ID is required', 'error')
    return
  }
  try {
    const res = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        action: 'add_provider',
        ...newProv.value
      })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Provider Saved', data.message, 'success')
      newProv.value = { id: '', type: 'openai', base_url: '', api_key_env: '' }
      pingStatus.value = null
      loadProvidersData()
    } else {
      showToast('Save Failed', data.error, 'error')
    }
  } catch (err) {
    showToast('Failed to save provider', err.message, 'error')
  }
}

async function deleteProvider(id) {
  if (!confirm(`Delete provider '${id}'?`)) return
  try {
    const res = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'delete_provider', id })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Provider Deleted', `Provider '${id}' removed from configuration.`, 'info')
      loadProvidersData()
    }
  } catch (err) {
    showToast('Delete Failed', err.message, 'error')
  }
}

function sendMessage() {
  if (!inputMsg.value.trim()) return
  const text = inputMsg.value
  const target = selectedTarget.value

  messages.value.push({
    role: 'user',
    targetLabel: target === 'master' ? 'Master Planner' : target,
    text: text,
    time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  })
  inputMsg.value = ''

  setTimeout(() => {
    messages.value.push({
      role: 'master',
      targetLabel: 'You',
      text: `Instruksi diterima, Jal. Aku mencatat kebutuhan: "${text}". Dokumen BRIEF.md akan aku perbarui.`,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    })
  }, 600)
}

function instructWorker(msg, action) {
  if (action === 'approve') {
    messages.value.push({
      role: 'user',
      targetLabel: msg.workerId,
      text: `Approved report for ${msg.taskId}. Lanjutkan ke langkah berikutnya.`,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    })
    msg.actionable = false
  } else if (action === 'direct') {
    selectedTarget.value = msg.workerId
    inputMsg.value = `Untuk task ${msg.taskId}: `
  } else if (action === 'via_master') {
    selectedTarget.value = 'master'
    inputMsg.value = `Master, tolong bantu arahkan ${msg.workerId} di task ${msg.taskId}.`
  }
}

function crackWhip() {
  alert('💥 Crack! Ringmaster whips workers to accelerate execution!')
}
</script>
