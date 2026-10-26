<template>
  <div class="flex h-screen w-screen overflow-hidden bg-slate-deep text-parchment font-mono select-none">
    <!-- Left Navigation Sidemenu -->
    <aside class="w-60 flex flex-col bg-slate-dark border-r border-steel z-20">
      <!-- Brand Header -->
      <div class="h-14 flex items-center gap-3 px-4 border-b border-steel bg-slate-deep">
        <span class="text-xl">🎪</span>
        <div>
          <h1 class="font-cinzel font-black tracking-wider text-gear-gold text-sm">RINGMASTER</h1>
          <p class="text-[9px] text-zinc-400 tracking-tight font-outfit uppercase">Agent Control Plane</p>
        </div>
      </div>

      <!-- Navigation Links (Chat, Tasks, Providers, Analytics) -->
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

      <!-- Active Project Info / Quick Status -->
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
          <span class="text-green-400 font-bold">1 Running</span>
        </div>
      </div>
    </aside>

    <!-- Main Content Workspace Area -->
    <main class="flex-1 flex flex-col h-full bg-slate-deep overflow-hidden">
      <!-- 1. CHAT VIEW (Unified Mission Control: Briefing + Worker Streams + Human Review) -->
      <section v-if="activeView === 'chat'" class="flex-1 flex h-full overflow-hidden">
        <!-- Dialogue Stream -->
        <div class="flex-1 flex flex-col h-full border-r border-steel">
          <!-- Top Header -->
          <div class="h-14 flex items-center justify-between px-6 border-b border-steel bg-slate-panel">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-bold text-gear-gold font-cinzel">Mission Control & Agent Channel</h2>
                <span class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
              </div>
              <p class="text-[10px] text-zinc-400">Collaborate with Master Planner, receive live worker reports, and steer execution.</p>
            </div>
            <div class="flex items-center gap-2">
              <button 
                @click="activeView = 'tasks'" 
                class="px-3 py-1.5 bg-slate-inner hover:bg-slate-panel text-zinc-300 text-xs rounded border border-steel flex items-center gap-1.5 transition"
              >
                <span>View Tasks (3)</span>
                <span>▸</span>
              </button>
            </div>
          </div>

          <!-- Messages Stream -->
          <div class="flex-1 p-6 overflow-y-auto space-y-4">
            <div 
              v-for="(msg, i) in messages" 
              :key="i"
              class="flex flex-col max-w-2xl"
              :class="msg.role === 'user' ? 'ml-auto items-end' : 'mr-auto items-start'"
            >
              <!-- Role / Origin Badge -->
              <div class="flex items-center gap-1.5 mb-1 text-[9px] uppercase tracking-wider">
                <span v-if="msg.role === 'user'" class="text-zinc-400 font-bold">You (Human Reviewer)</span>
                <span v-else-if="msg.role === 'master'" class="text-gear-gold font-bold">🎪 Master Planner</span>
                <span v-else-if="msg.role === 'worker'" class="text-cyan-energy font-bold">⚡ Worker ({{ msg.workerName }}) · Task {{ msg.taskId }}</span>
                <span v-else-if="msg.role === 'system'" class="text-zinc-500">System Notification</span>
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
                    class="px-2.5 py-1 bg-slate-dark hover:bg-slate-panel text-zinc-200 border border-steel rounded text-[10px] font-semibold transition"
                  >
                    💬 Direct Worker
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

          <!-- Input Bar with Route Selector (Send to Master or Direct to Worker) -->
          <div class="p-4 border-t border-steel bg-slate-panel">
            <div class="flex items-center gap-2 mb-2">
              <span class="text-[10px] text-zinc-400">Target Agent:</span>
              <button 
                @click="chatTarget = 'master'"
                class="px-2 py-0.5 rounded text-[10px] font-bold transition"
                :class="chatTarget === 'master' ? 'bg-gear-gold text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel'"
              >
                🎪 Master Planner
              </button>
              <button 
                @click="chatTarget = 'worker'"
                class="px-2 py-0.5 rounded text-[10px] font-bold transition"
                :class="chatTarget === 'worker' ? 'bg-cyan-energy text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel'"
              >
                ⚡ Active Worker (coder-sonnet)
              </button>
            </div>
            <form @submit.prevent="sendMessage" class="flex items-center gap-3">
              <input 
                v-model="inputMsg"
                type="text" 
                :placeholder="chatTarget === 'master' ? 'Beri instruksi ke Master Planner (arsitektur, scope, orkestrasi)...' : 'Beri instruksi langsung ke Worker (fix kode, acceptance, debug)...'"
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

        <!-- Live Context Sidebar (BRIEF & Live Tasks Stream) -->
        <div class="w-80 flex flex-col h-full bg-slate-panel/40">
          <div class="h-14 flex items-center justify-between px-4 border-b border-steel bg-slate-panel">
            <span class="text-xs font-bold text-zinc-300">Project Context</span>
            <span class="text-[9px] px-2 py-0.5 rounded bg-slate-inner text-gear-gold border border-steel">Live State</span>
          </div>

          <!-- Collapsible Sections -->
          <div class="flex-1 p-4 overflow-y-auto space-y-4">
            <!-- Active Task Monitor Mini-card -->
            <div class="bg-slate-dark border border-steel rounded p-3">
              <div class="flex items-center justify-between mb-1.5">
                <span class="text-[10px] text-zinc-400">Current Running Task</span>
                <span class="text-[9px] text-amber-400 font-bold bg-amber-950 px-1.5 py-0.2 rounded">RUNNING</span>
              </div>
              <p class="text-xs font-bold text-zinc-200">T02: Implement Unit Tests</p>
              <div class="w-full bg-slate-inner h-1.5 rounded mt-2 overflow-hidden">
                <div class="bg-amber-400 h-full w-2/3 animate-pulse"></div>
              </div>
              <div class="flex items-center justify-between text-[9px] text-zinc-400 mt-2">
                <span>Branch: <code class="text-cyan-energy">rm/T02</code></span>
                <span>Turn: 4/30</span>
              </div>
            </div>

            <!-- Brief Markdown Document -->
            <div>
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-zinc-300">BRIEF.md</span>
                <button @click="activeView = 'tasks'" class="text-[9px] text-gear-gold hover:underline">Full Tasks ▸</button>
              </div>
              <div class="p-3 bg-slate-inner border border-steel rounded font-mono text-[10px] leading-relaxed text-zinc-400 whitespace-pre-wrap">
{{ briefPreview }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. TASKS VIEW (Replaced 'DAG' with Clean Task Board) -->
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

        <!-- Task Pipeline Cards -->
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
                  :class="task.status === 'DONE' ? 'bg-green-950 text-green-400 border border-green-800' : (task.status === 'RUNNING' ? 'bg-amber-950 text-amber-400 border border-amber-800' : 'bg-slate-inner text-zinc-400 border border-steel')"
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
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-xs font-bold text-zinc-200">Registered Providers</h3>
            <button class="px-3 py-1.5 bg-gear-gold text-slate-deep font-bold text-xs rounded">+ Add Provider</button>
          </div>
          <p class="text-xs text-zinc-400">Integrated with local <code class="text-gear-gold">ringmaster.toml</code>.</p>
        </div>
      </section>

      <!-- 4. ANALYTICS VIEW -->
      <section v-else-if="activeView === 'analytics'" class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Execution Analytics & Token Usage</h2>
          <p class="text-xs text-zinc-400">Accurate token breakdown per agent, role, and task vs estimates.</p>
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Total Tokens Consumed</span>
            <p class="text-xl font-bold text-gear-gold mt-1">42,850</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Estimated Range</span>
            <p class="text-xl font-bold text-cyan-energy mt-1">40k – 90k</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Project Budget Limit</span>
            <p class="text-xl font-bold text-zinc-300 mt-1">300,000</p>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const activeView = ref('chat')
const chatTarget = ref('master')

const navItems = [
  { id: 'chat', label: 'Chat', icon: '💬', badge: '1 Alert', badgeClass: 'bg-amber-950 text-amber-400 border border-amber-800' },
  { id: 'tasks', label: 'Tasks', icon: '📋' },
  { id: 'providers', label: 'Providers', icon: '🤖' },
  { id: 'analytics', label: 'Analytics', icon: '📈' }
]

const messages = ref([
  {
    role: 'master',
    text: 'Halo Jal! Aku Master Architect Ringmaster. Aku sudah menyusun rencana tugas awal untuk modul kalkulator matematika. Task T01 sudah selesai dan task T02 saat ini sedang dikerjakan oleh Worker.',
    time: '15:30'
  },
  {
    role: 'worker',
    workerName: 'coder-sonnet',
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
    workerName: 'coder-sonnet',
    taskId: 'T02',
    taskBranch: 'rm/T02',
    statusTitle: '⚠ WORKER REPORT: UNIT TESTS EDGE CASES BLOCKED',
    type: 'error',
    text: 'Aku menemukan divide-by-zero handling belum didefinisikan dalam brief. Apakah fungsi harus mengembalikan error eksplisit (result, err) atau panic?',
    time: '15:35',
    actionable: true
  }
])

const inputMsg = ref('')

const briefPreview = ref(`# BRIEF: Simple Math Module

## Objective
Membangun modul kalkulasi matematika sederhana di Go dengan unit test lengkap.

## Scope & Requirements
- File: math_helper.go
- Functions: add(a, b), multiply(a, b), divide(a, b)
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
  const text = inputMsg.value
  messages.value.push({
    role: 'user',
    text: text,
    time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  })
  inputMsg.value = ''

  setTimeout(() => {
    if (chatTarget.value === 'master') {
      messages.value.push({
        role: 'master',
        text: `Instruksi diterima, Jal. Aku akan mengkoordinasikan keputusan ini ke Worker di task T02 dan memperbarui file brief.`,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      })
    } else {
      messages.value.push({
        role: 'worker',
        workerName: 'coder-sonnet',
        taskId: 'T02',
        taskBranch: 'rm/T02',
        statusTitle: '⚡ WORKER ACKNOWLEDGED',
        type: 'success',
        text: `Siap! Instruksi "${text}" langsung aku terapkan ke kode dan test suite di worktree rm/T02.`,
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
      text: `Approved report for ${msg.taskId}. Lanjutkan ke langkah berikutnya.`,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    })
    msg.actionable = false
  } else if (action === 'direct') {
    chatTarget.value = 'worker'
    inputMsg.value = `Untuk task ${msg.taskId}: `
  } else if (action === 'via_master') {
    chatTarget.value = 'master'
    inputMsg.value = `Master, tolong tinjau ulang blocker pada task ${msg.taskId}: `
  }
}

function crackWhip() {
  alert('💥 Crack! Ringmaster whips workers to accelerate execution!')
}
</script>
