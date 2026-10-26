<template>
  <div class="flex h-screen w-screen overflow-hidden bg-slate-deep text-parchment font-mono select-none">
    <!-- Left Navigation Sidemenu -->
    <aside class="w-60 flex flex-col bg-slate-dark border-r border-steel z-20">
      <!-- Brand Header -->
      <div class="h-14 flex items-center gap-3 px-4 border-b border-steel bg-slate-deep">
        <img src="/icons/ringmaster-icon-transparent.svg" alt="Ringmaster Icon" class="w-7 h-7 object-contain drop-shadow-[0_0_8px_rgba(222,170,59,0.5)]" />
        <div>
          <h1 class="font-cinzel font-black tracking-wider text-gear-gold text-sm">RINGMASTER</h1>
          <p class="text-[9px] text-zinc-400 tracking-tight font-outfit uppercase">Multi-Agent Control Plane</p>
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
          <span v-if="item.badge" class="px-1.5 py-0.5 text-[9px] rounded font-bold" :class="item.badgeClass">
            {{ item.badge }}
          </span>
        </button>
      </nav>

      <!-- Active Project Info & Multi-Worker Fleet Status -->
      <div class="p-3 border-t border-steel bg-slate-inner/60">
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Active Project</span>
          <span class="text-gear-gold font-bold">demo</span>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Master Planner</span>
          <span class="text-cyan-energy">claude-3-7</span>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400">
          <span>Workers Active</span>
          <span class="text-green-400 font-bold">{{ activeWorkersCount }} Parallel</span>
        </div>
      </div>
    </aside>

    <!-- Main Content Workspace Area -->
    <main class="flex-1 flex flex-col h-full bg-slate-deep overflow-hidden">
      <!-- 1. CHAT VIEW (Mission Control with Dynamic Multi-Worker Selector) -->
      <section v-if="activeView === 'chat'" class="flex-1 flex h-full overflow-hidden">
        <!-- Dialogue Stream -->
        <div class="flex-1 flex flex-col h-full border-r border-steel">
          <!-- Top Header -->
          <div class="h-14 flex items-center justify-between px-6 border-b border-steel bg-slate-panel">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-bold text-gear-gold font-cinzel">Mission Control & Agent Stream</h2>
                <span class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
              </div>
              <p class="text-[10px] text-zinc-400">Collaborate with Master Planner, receive live reports from all workers, and steer execution.</p>
            </div>
            
            <!-- Quick View Tasks Shortcut -->
            <button 
              @click="activeView = 'tasks'" 
              class="px-3 py-1.5 bg-slate-inner hover:bg-slate-panel text-zinc-300 text-xs rounded border border-steel flex items-center gap-1.5 transition"
            >
              <span>View Tasks Pipeline ({{ tasks.length }})</span>
              <span>▸</span>
            </button>
          </div>

          <!-- Messages Stream (Filterable by Agent) -->
          <div class="flex-1 p-6 overflow-y-auto space-y-4">
            <div 
              v-for="(msg, i) in filteredMessages" 
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

                <!-- Interactive Human Review Actions for Worker Blockers / Reports -->
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

          <!-- Input Bar with Dynamic Multi-Worker Selector -->
          <div class="p-4 border-t border-steel bg-slate-panel">
            <div class="flex items-center gap-2 mb-2 overflow-x-auto pb-1">
              <span class="text-[10px] text-zinc-400 font-semibold whitespace-nowrap">Send Instruction To:</span>
              
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
                  ? 'Kirim instruksi ke Master Planner (misal: ubah arsitektur, tambahkan requirement baru)...' 
                  : `Kirim instruksi langsung ke worker '${selectedTarget}' di worktree khususnya...`"
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

        <!-- Right Panel: Multi-Worker Parallel Fleet Monitor & Brief -->
        <div class="w-80 flex flex-col h-full bg-slate-panel/40">
          <div class="h-14 flex items-center justify-between px-4 border-b border-steel bg-slate-panel">
            <span class="text-xs font-bold text-zinc-300">Active Parallel Fleet</span>
            <span class="text-[9px] px-2 py-0.5 rounded bg-slate-inner text-green-400 border border-steel">
              {{ activeWorkersList.length }} Worktrees Active
            </span>
          </div>

          <!-- Dynamic Multi-Worker Cards List -->
          <div class="flex-1 p-4 overflow-y-auto space-y-3">
            <div 
              v-for="worker in activeWorkersList"
              :key="worker.id"
              class="bg-slate-dark border border-steel rounded p-3 hover:border-cyan-dim transition cursor-pointer"
              @click="selectedTarget = worker.id"
            >
              <div class="flex items-center justify-between mb-1.5">
                <span class="text-[10px] text-zinc-300 font-bold">{{ worker.name }}</span>
                <span 
                  class="text-[9px] font-bold px-1.5 py-0.2 rounded uppercase"
                  :class="worker.status === 'RUNNING' ? 'bg-amber-950 text-amber-400 border border-amber-800' : 'bg-red-950 text-red-400 border border-red-800'"
                >
                  {{ worker.status }}
                </span>
              </div>
              
              <p class="text-xs font-semibold text-zinc-200">{{ worker.taskId }}: {{ worker.taskTitle }}</p>
              
              <!-- Progress bar -->
              <div class="w-full bg-slate-inner h-1.5 rounded mt-2 overflow-hidden">
                <div 
                  class="h-full"
                  :class="worker.status === 'RUNNING' ? 'bg-amber-400 animate-pulse' : 'bg-red-500'"
                  :style="{ width: worker.progress + '%' }"
                ></div>
              </div>
              
              <div class="flex items-center justify-between text-[9px] text-zinc-400 mt-2">
                <span>Branch: <code class="text-cyan-energy">rm/{{ worker.taskId }}</code></span>
                <span>Turn: {{ worker.turn }}/30</span>
              </div>
            </div>

            <!-- Brief Markdown Document -->
            <div class="pt-2 border-t border-steel">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-zinc-300">BRIEF.md</span>
                <button @click="activeView = 'tasks'" class="text-[9px] text-gear-gold hover:underline">Full Tasks ▸</button>
              </div>
              <div class="p-3 bg-slate-inner border border-steel rounded font-mono text-[10px] leading-relaxed text-zinc-400 whitespace-pre-wrap max-h-48 overflow-y-auto">
{{ briefPreview }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. TASKS VIEW (Pipeline) -->
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
              @click="crackWhip"
              class="px-4 py-2 bg-coat-bright hover:bg-coat-crimson text-white font-bold text-xs rounded shadow flex items-center gap-2 transition"
            >
              <span>⚡ CRACK WHIP (TAME WORKERS)</span>
            </button>
          </div>
        </div>

        <!-- Task Pipeline Cards Grid -->
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

      <!-- 3. PROVIDERS VIEW -->
      <section v-else-if="activeView === 'providers'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">AI Providers & Model Registry</h2>
          <p class="text-xs text-zinc-400">Manage API keys, Claude CLI subscription tokens, and test connection latency.</p>
        </div>
        <div class="bg-slate-panel border border-steel rounded-lg p-5">
          <p class="text-xs text-zinc-400">Integrated with local <code class="text-gear-gold">ringmaster.toml</code>.</p>
        </div>
      </section>

      <!-- 4. ANALYTICS VIEW -->
      <section v-else-if="activeView === 'analytics'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Execution Analytics & Token Usage</h2>
          <p class="text-xs text-zinc-400">Accurate token breakdown per agent, role, and task vs estimates.</p>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const activeView = ref('chat')
const selectedTarget = ref('master')

const navItems = [
  { id: 'chat', label: 'Chat', icon: '💬', badge: '1 Alert', badgeClass: 'bg-amber-950 text-amber-400 border border-amber-800' },
  { id: 'tasks', label: 'Tasks', icon: '📋' },
  { id: 'providers', label: 'Providers', icon: '🤖' },
  { id: 'analytics', label: 'Analytics', icon: '📈' }
]

// Dynamic Parallel Workers List currently operating across git worktrees
const activeWorkersList = ref([
  {
    id: 'coder-sonnet',
    name: 'coder-sonnet',
    taskId: 'T02',
    taskTitle: 'Implement Unit Tests',
    status: 'RUNNING',
    progress: 65,
    turn: 4
  },
  {
    id: 'coder-haiku',
    name: 'coder-haiku',
    taskId: 'T03',
    taskTitle: 'Generate Markdown API Docs',
    status: 'BLOCKED',
    progress: 30,
    turn: 2
  }
])

const activeWorkersCount = computed(() => activeWorkersList.value.length)

const messages = ref([
  {
    role: 'master',
    targetLabel: 'Team',
    text: 'Halo Jal! Rencana proyek sudah disetujui. Dua worker sekarang sedang bekerja secara paralel di git worktree terisolasi masing-masing.',
    time: '15:30'
  },
  {
    role: 'worker',
    workerId: 'coder-sonnet',
    taskId: 'T01',
    taskBranch: 'rm/T01',
    statusTitle: '✓ TASK T01 COMPLETED & MERGED',
    type: 'success',
    text: 'Implementasi add() dan multiply() selesai di math_helper.go. Acceptance command "go test -v ./..." lulus 100%. Telah di-merge ke branch main.',
    time: '15:32',
    actionable: false
  },
  {
    role: 'worker',
    workerId: 'coder-haiku',
    taskId: 'T03',
    taskBranch: 'rm/T03',
    statusTitle: '⚠ WORKER REPORT: MISSING DOCUMENTATION TEMPLATE',
    type: 'error',
    text: 'Aku butuh petunjuk apakah dokumentasi API fungsi matematika ingin dibuat dalam bahasa Indonesia atau bahasa Inggris?',
    time: '15:35',
    actionable: true
  }
])

const filteredMessages = computed(() => {
  return messages.value
})

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
  { id: 'T03', title: 'Generate Markdown API Documentation', desc: 'Document functions in README', worker: 'coder-haiku', status: 'BLOCKED', retries: '0/2' }
])

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
    if (target === 'master') {
      messages.value.push({
        role: 'master',
        targetLabel: 'You',
        text: `Instruksi diterima, Jal. Aku akan mengkoordinasikan strategi ini ke worker yang relevan.`,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      })
    } else {
      messages.value.push({
        role: 'worker',
        workerId: target,
        taskId: target === 'coder-haiku' ? 'T03' : 'T02',
        taskBranch: target === 'coder-haiku' ? 'rm/T03' : 'rm/T02',
        statusTitle: '⚡ WORKER ACKNOWLEDGED DIRECT INSTRUCTION',
        type: 'success',
        text: `Siap! Instruksi "${text}" langsung aku eksekusi di worktree sekarang.`,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        actionable: false
      })
    }
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
    inputMsg.value = `Untuk task ${msg.taskId}: Gunakan bahasa Indonesia. `
  } else if (action === 'via_master') {
    selectedTarget.value = 'master'
    inputMsg.value = `Master, tolong bantu arahkan ${msg.workerId} di task ${msg.taskId} mengenai template dokumentasi.`
  }
}

function crackWhip() {
  alert('💥 Crack! Ringmaster whips workers to expedite execution across all active worktrees!')
}
</script>
