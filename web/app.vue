<template>
  <div class="flex h-screen w-screen overflow-hidden bg-slate-deep text-parchment font-mono select-none">
    <!-- Left Navigation Sidemenu -->
    <aside class="w-64 flex flex-col bg-slate-dark border-r border-steel z-20">
      <!-- Brand Header -->
      <div class="h-14 flex items-center gap-3 px-4 border-b border-steel bg-slate-deep">
        <span class="text-xl">🎪</span>
        <div>
          <h1 class="font-cinzel font-black tracking-wider text-gear-gold text-sm">RINGMASTER</h1>
          <p class="text-[9px] text-zinc-400 tracking-tight font-outfit uppercase">Autonomous Control Plane</p>
        </div>
      </div>

      <!-- Navigation Links -->
      <nav class="flex-1 p-3 space-y-1.5 overflow-y-auto">
        <button
          v-for="item in navItems"
          :key="item.id"
          @click="activeView = item.id"
          class="w-full flex items-center gap-3 px-3 py-2.5 rounded text-xs font-semibold transition-all"
          :class="activeView === item.id 
            ? 'bg-slate-panel text-gear-gold border border-gear-dark/40 shadow-sm shadow-gear-dark/20' 
            : 'text-zinc-400 hover:text-zinc-200 hover:bg-slate-panel/50'"
        >
          <span class="text-base">{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <!-- Active Project Info / Quick Status -->
      <div class="p-3 border-t border-steel bg-slate-inner/60">
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Active Project</span>
          <span class="text-gear-gold font-bold">demo</span>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400">
          <span>Master Planner</span>
          <span class="text-cyan-energy">claude-3-7</span>
        </div>
      </div>
    </aside>

    <!-- Main Content Workspace Area -->
    <main class="flex-1 flex flex-col h-full bg-slate-deep overflow-hidden">
      <!-- 1. CHAT & BRIEFING VIEW (Default Landing) -->
      <section v-if="activeView === 'chat'" class="flex-1 flex h-full overflow-hidden">
        <!-- Chat Dialogue Pane -->
        <div class="flex-1 flex flex-col h-full border-r border-steel">
          <!-- Top Header -->
          <div class="h-14 flex items-center justify-between px-6 border-b border-steel bg-slate-panel">
            <div>
              <h2 class="text-sm font-bold text-gear-gold font-cinzel">Project Briefing & Architecture Discussion</h2>
              <p class="text-[10px] text-zinc-400">Collaborate with Master Planner to define project scope and requirements.</p>
            </div>
            <button 
              @click="activeView = 'board'" 
              class="px-3.5 py-1.5 bg-gear-gold hover:bg-gear-glow text-slate-deep font-bold text-xs rounded transition flex items-center gap-1.5 shadow"
            >
              <span>Accept Brief & View Tasks</span>
              <span>▸</span>
            </button>
          </div>

          <!-- Messages Stream -->
          <div class="flex-1 p-6 overflow-y-auto space-y-4">
            <div 
              v-for="(msg, i) in messages" 
              :key="i"
              class="flex flex-col max-w-2xl"
              :class="msg.role === 'user' ? 'ml-auto items-end' : 'mr-auto items-start'"
            >
              <span class="text-[9px] uppercase tracking-wider text-zinc-500 mb-1">
                {{ msg.role === 'user' ? 'You (Architect)' : '🎪 Master Planner AI' }}
              </span>
              <div 
                class="p-3.5 rounded-lg text-xs leading-relaxed"
                :class="msg.role === 'user' 
                  ? 'bg-slate-panel text-parchment border border-steel' 
                  : 'bg-slate-dark text-parchment border border-gear-dark/40 shadow-sm'"
              >
                <p class="whitespace-pre-wrap">{{ msg.text }}</p>
              </div>
            </div>
          </div>

          <!-- Input Bar -->
          <div class="p-4 border-t border-steel bg-slate-panel">
            <form @submit.prevent="sendMessage" class="flex items-center gap-3">
              <input 
                v-model="inputMsg"
                type="text" 
                placeholder="Diskusikan fitur, stack, atau requirement dengan Master Planner..."
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

        <!-- Live BRIEF.md Preview Sidebar -->
        <div class="w-80 flex flex-col h-full bg-slate-panel/40">
          <div class="h-14 flex items-center justify-between px-4 border-b border-steel bg-slate-panel">
            <span class="text-xs font-bold text-zinc-300">Live BRIEF.md Preview</span>
            <span class="text-[9px] px-2 py-0.5 rounded bg-slate-inner text-gear-gold border border-steel">Auto-Sync</span>
          </div>
          <div class="flex-1 p-4 overflow-y-auto font-mono text-[11px] leading-relaxed text-zinc-300 whitespace-pre-wrap">
            {{ briefPreview }}
          </div>
        </div>
      </section>

      <!-- 2. TASK BOARD & ORCHESTRATION VIEW -->
      <section v-else-if="activeView === 'board'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">DAG Task Execution Board</h2>
            <p class="text-xs text-zinc-400">Monitor parallel worker worktrees and acceptance verifications.</p>
          </div>
          <div class="flex items-center gap-3">
            <button 
              @click="crackWhip"
              class="px-4 py-2 bg-coat-bright hover:bg-coat-crimson text-white font-bold text-xs rounded shadow flex items-center gap-2 transition"
            >
              <span>⚡ CRACK WHIP (TAME WORKERS)</span>
            </button>
          </div>
        </div>

        <!-- Task Grid -->
        <div class="grid grid-cols-3 gap-4">
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
                  :class="task.status === 'DONE' ? 'bg-green-950 text-green-400 border border-green-800' : 'bg-amber-950 text-amber-400 border border-amber-800'"
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

      <!-- 3. MODELS & PROVIDER CONFIG VIEW -->
      <section v-else-if="activeView === 'models'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">AI Providers & Model Registry</h2>
          <p class="text-xs text-zinc-400">Configure multi-provider endpoints and ping latency.</p>
        </div>
        <div class="bg-slate-panel border border-steel rounded-lg p-5">
          <p class="text-xs text-zinc-300">Model management interface integrated with ringmaster.toml.</p>
        </div>
      </section>

      <!-- 4. TOKEN USAGE ANALYTICS VIEW -->
      <section v-else-if="activeView === 'usage'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Token Usage & Budget Tracking</h2>
          <p class="text-xs text-zinc-400">Cumulative token breakdown per agent, role, and task vs estimates.</p>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const activeView = ref('chat')

const navItems = [
  { id: 'chat', label: 'Briefing & Chat', icon: '💬' },
  { id: 'board', label: 'DAG Task Board', icon: '📊' },
  { id: 'models', label: 'Models & Providers', icon: '🤖' },
  { id: 'usage', label: 'Token Analytics', icon: '📈' }
]

const messages = ref([
  {
    role: 'assistant',
    text: 'Halo Jal! Aku Master Architect Ringmaster. Proyek apa yang mau kita bangun hari ini? Berikan gambaran ide dasar atau fiturnya, nanti aku bantu susunkan arsitektur dan spesifikasi teknisnya sampai matang.'
  }
])

const inputMsg = ref('')

const briefPreview = ref(`# BRIEF: Simple Math Module

## Objective
Membangun modul kalkulasi matematika sederhana di Go dengan unit test lengkap.

## Scope & Requirements
- File: math_helper.go
- Functions: add(a, b), multiply(a, b)
- Unit tests: math_helper_test.go

## Acceptance Criteria
- [ ] go test -v ./... lolos 100%`)

const tasks = ref([
  { id: 'T01', title: 'Setup Repository & Core Math Functions', desc: 'Implement add() and multiply()', worker: 'coder-sonnet', status: 'DONE', retries: '0/2' },
  { id: 'T02', title: 'Implement Comprehensive Unit Tests', desc: 'Add test suite for edge cases', worker: 'coder-sonnet', status: 'RUNNING', retries: '1/2' },
  { id: 'T03', title: 'Generate Markdown API Documentation', desc: 'Document functions in README', worker: 'coder-sonnet', status: 'PENDING', retries: '0/2' }
])

function sendMessage() {
  if (!inputMsg.value.trim()) return
  messages.value.push({ role: 'user', text: inputMsg.value })
  const userText = inputMsg.value
  inputMsg.value = ''

  setTimeout(() => {
    messages.value.push({
      role: 'assistant',
      text: `Paham, Jal! Ide untuk "${userText}" sudah kucatat ke dalam draft brief. Apakah ada batasan teknologi khusus (misal database atau library) yang wajib dipakai?`
    })
  }, 600)
}

function crackWhip() {
  alert('💥 Crack! Ringmaster whips workers to accelerate execution!')
}
</script>
