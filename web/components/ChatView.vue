<template>
      <section class="flex-1 flex h-full overflow-hidden">
        <!-- Dialogue Stream -->
        <div class="flex-1 flex flex-col h-full border-r border-steel">
          <!-- Top Header -->
          <div class="h-14 flex items-center justify-between px-6 border-b border-steel bg-slate-panel">
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-bold text-gear-gold font-cinzel">Chat</h2>
                <span class="w-2 h-2 rounded-full" :class="project.isBlank ? 'bg-amber-400' : 'bg-green-500'"></span>
              </div>
              <p class="text-xs text-zinc-400">
                {{ project.isBlank ? 'Tell the Master what to build. The conversation becomes your brief.' : 'Talk to the Master and see what automatons report.' }}
              </p>
            </div>
            
            <div class="flex items-center gap-2">
            <!-- Master Planner activity, shown here rather than next to the sidebar logo -->
            <span
              v-if="isThinking || isGeneratingPlan"
              class="px-2 py-1 rounded border border-gear-dark/60 bg-gear-dark/20 text-gear-gold text-xs font-bold uppercase tracking-wide flex items-center gap-1.5 whitespace-nowrap"
            >
              <span class="w-1.5 h-1.5 rounded-full bg-gear-gold"></span>
              {{ isGeneratingPlan ? 'Generating plan' : 'Planning' }}
            </span>
            <!-- Orchestration activity; click through to the live logs -->
            <button
              v-if="isRunActive"
              @click="activeView = 'logs'"
              class="px-2 py-1 rounded border border-cyan-700 bg-cyan-950 text-cyan-300 hover:border-cyan-500 text-xs font-bold uppercase tracking-wide flex items-center gap-1.5 whitespace-nowrap transition"
              title="Open live logs"
            >
              <span class="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
              Running
              <span class="font-normal normal-case text-cyan-200/80">
                · {{ activeWorkersList.length }} automaton{{ activeWorkersList.length === 1 ? '' : 's' }} · {{ completedTasksCount }}/{{ tasks.length }} done
              </span>
            </button>
            <button 
              v-if="!project.isBlank"
              @click="activeView = 'tasks'" 
              class="px-3 py-1.5 bg-slate-inner hover:bg-slate-panel text-zinc-300 text-body rounded border border-steel flex items-center gap-1.5 transition"
            >
              <span>Tasks ({{ tasks.length }})</span>
              <span>▸</span>
            </button>
            </div>
          </div>

          <!-- Messages Stream (opens at the newest message and follows new ones) -->
          <div class="relative flex-1 min-h-0 flex flex-col">
          <div ref="chatContainerRef" @scroll="onChatScroll" class="flex-1 p-6 overflow-y-auto space-y-4">
            <!-- Welcome Hero Animated SVG when chat is clean/blank -->
            <div v-if="messages.length <= 1" class="flex flex-col items-center justify-center py-6 text-center select-none">
              <img src="/icons/ringmaster-animated.svg" alt="" class="w-44 h-44 object-contain drop-shadow-[0_0_25px_rgba(222,170,59,0.35)] mb-3" />
              <h3 class="font-cinzel text-base font-bold text-gear-gold tracking-wide">Start with a brief</h3>
              <p class="text-body text-zinc-400 max-w-md mt-1">Tell the Master what you want to build. It asks what it needs, then splits the work into tasks for automatons.</p>
            </div>

            <div 
              v-for="(msg, i) in messages" 
              :key="i"
              class="flex flex-col max-w-2xl"
              :class="msg.role === 'user' ? 'ml-auto items-end' : 'mr-auto items-start'"
            >
              <!-- Role / Origin Badge -->
              <div class="flex items-center gap-1.5 mb-1 text-2xs uppercase tracking-wider">
                <span v-if="msg.role === 'user'" class="text-zinc-400 font-bold">You → {{ msg.targetLabel }}</span>
                <span v-else-if="msg.role === 'master'" class="text-gear-gold font-bold">🎪 Master</span>
                <span v-else-if="msg.role === 'worker'" class="text-cyan-energy font-bold">🤖 Automaton {{ msg.workerId }} · {{ msg.taskId }}</span>
                <span class="text-zinc-600">· {{ msg.time }}</span>
              </div>

              <!-- Message Bubble -->
              <div 
                class="p-4 rounded-lg text-body leading-relaxed"
                :class="{
                  'bg-slate-panel text-parchment border border-steel': msg.role === 'user',
                  'bg-slate-dark text-parchment border border-gear-dark/40 shadow-sm': msg.role === 'master',
                  'bg-slate-inner text-parchment border border-cyan-dim/40 shadow-sm': msg.role === 'worker' && msg.type !== 'error',
                  'bg-coat-dim/30 text-parchment border border-coat-crimson shadow-sm': msg.type === 'error'
                }"
              >
                <!-- Automaton Report Card Header -->
                <div v-if="msg.role === 'worker'" class="flex items-center justify-between pb-2 mb-2 border-b border-steel/50">
                  <span class="text-xs font-bold tracking-wider" :class="msg.type === 'error' ? 'text-coat-bright' : 'text-cyan-energy'">
                    {{ msg.statusTitle }}
                  </span>
                  <span class="text-2xs px-1.5 py-0.5 rounded bg-slate-panel text-zinc-400">
                    {{ msg.taskBranch }}
                  </span>
                </div>

                <!-- The Master writes markdown; your own messages and automaton reports stay as typed -->
                <MarkdownText v-if="msg.role === 'master'" :source="msg.text" />
                <p v-else class="whitespace-pre-wrap">{{ msg.text }}</p>

                <!-- Shortcut button to navigate directly to Tasks tab if plan was generated -->
                <div v-if="msg.role === 'master' && (msg.planReady || (msg.text && msg.text.includes('DAG Plan successfully generated')))" class="mt-3 pt-2.5 border-t border-gear-dark/40 flex items-center gap-2">
                  <button 
                    @click="setView('tasks')"
                    class="px-3.5 py-1.5 bg-gradient-to-r from-gear-gold to-amber-500 hover:from-amber-400 hover:to-gear-gold text-ink font-bold text-body rounded transition flex items-center gap-1.5 shadow"
                  >
                    <span>Review the plan</span>
                    <span>▸</span>
                  </button>
                </div>

                <!-- Actionable Buttons if worker needs review -->
                <div v-if="msg.actionable" class="mt-3 pt-3 border-t border-steel/60 flex items-center gap-2">
                  <button 
                    @click="instructWorker(msg, 'answer')" 
                    class="px-2.5 py-1 bg-green-950 hover:bg-green-900 text-green-400 border border-green-800 rounded text-xs font-bold transition"
                  >
                    🙋 Answer
                  </button>
                  <button 
                    @click="instructWorker(msg, 'via_master')" 
                    class="px-2.5 py-1 bg-gear-dark/30 hover:bg-gear-dark/50 text-gear-gold border border-gear-dark rounded text-xs font-semibold transition"
                  >
                    🎪 Ask the Master
                  </button>
                </div>
              </div>
            </div>

            <!-- Thinking / Typing Indicator with Animated Kinematics Emblem -->
            <div v-if="isThinking" class="flex flex-col mr-auto items-start max-w-xl animate-fade-in">
              <div class="flex items-center gap-1.5 mb-1 text-2xs uppercase tracking-wider">
                <span class="text-gear-gold font-bold">🎪 Master</span>
                <span class="text-zinc-500">· thinking</span>
              </div>
              <div class="p-3.5 rounded-lg text-body leading-relaxed bg-slate-dark border border-gear-gold/40 shadow-md flex items-center gap-3">
                <!-- Kinematic Emblem with moving hands, rotating gears & strings -->
                <RingmasterEmblem :status="isGeneratingPlan ? 'dag' : 'execution'" size="sm" />
                <div class="flex flex-col">
                  <span class="text-gear-gold font-semibold text-body tracking-wide">
                    {{ isGeneratingPlan ? 'Generating the plan…' : 'Thinking…' }}
                  </span>
                  <span class="text-zinc-400 text-xs">
                    {{ isGeneratingPlan ? 'Splitting the brief into tasks with acceptance checks' : 'Reading your message' }}
                  </span>
                </div>
              </div>
            </div>
          </div>

            <button
              v-if="!atBottom && unseen > 0"
              @click="jumpToLatest"
              class="absolute bottom-3 left-1/2 -translate-x-1/2 px-3 py-1 rounded-full bg-gear-gold text-ink text-xs font-bold shadow-lg"
            >↓ {{ unseen }} new message{{ unseen === 1 ? '' : 's' }}</button>
          </div>

          <!-- Input Bar with Dynamic Target Selector -->
          <div class="relative p-4 border-t border-steel bg-slate-panel">
            <!-- Drag handle: pull the top edge to resize the input, double-click to reset -->
            <div
              class="group absolute -top-1.5 left-0 right-0 h-3 cursor-row-resize z-10 flex items-center justify-center touch-none"
              title="Drag to resize · double-click to reset"
              @pointerdown="startResize"
              @dblclick="resetInputHeight"
            >
              <div
                class="h-1 w-12 rounded-full transition-colors"
                :class="isResizing ? 'bg-gear-gold' : 'bg-steel group-hover:bg-gear-dark'"
              ></div>
            </div>

            <!-- Warning if no Master Planner is configured -->
            <div v-if="!hasMasterPlanner" class="mb-3 p-3 bg-amber-950/60 border border-amber-800 rounded flex items-center justify-between text-body">
              <div class="flex items-center gap-2 text-amber-300">
                <span>⚠</span>
                <span>No Master is set, so chat is off.</span>
              </div>
              <button @click="activeView = 'providers'" class="px-2.5 py-1 bg-gear-gold text-ink font-bold rounded text-xs">
                Set a Master in Providers ▸
              </button>
            </div>

            <div class="flex items-center gap-2 mb-2 overflow-x-auto pb-1 justify-between">
              <div class="flex items-center gap-2">
                <span class="text-xs text-zinc-400 font-semibold whitespace-nowrap">To:</span>
                
                <!-- Master Option -->
                <button 
                  @click="selectedTarget = 'master'"
                  :disabled="!hasMasterPlanner"
                  class="px-2.5 py-1 rounded text-xs font-bold transition flex items-center gap-1 whitespace-nowrap disabled:opacity-40 disabled:cursor-not-allowed"
                  :class="selectedTarget === 'master' ? 'bg-gear-gold text-ink' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
                >
                  <span>🎪 Master</span>
                  <span v-if="!hasMasterPlanner" class="text-2xs text-amber-300">(None)</span>
                </button>

                <!-- Dynamic Active Automaton Options -->
                <button 
                  v-for="worker in activeWorkersList"
                  :key="worker.taskId"
                  @click="selectedTarget = worker.id"
                  class="px-2.5 py-1 rounded text-xs font-bold transition flex items-center gap-1.5 whitespace-nowrap"
                  :class="selectedTarget === worker.id ? 'bg-cyan-energy text-ink' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
                >
                  <span>🤖 {{ worker.name }} · {{ worker.taskId }}</span>
                </button>
              </div>

              <!-- Dedicated Generate DAG Plan Button (Only active when Master signals CAN_GENERATE) -->
              <div class="flex items-center gap-2">
                <span 
                  v-if="!canGeneratePlan && !isGeneratingPlan" 
                  class="text-xs text-zinc-500 italic hidden sm:inline"
                >
                  Unlocks when the Master has enough detail
                </span>
                <button
                  @click="triggerGeneratePlan"
                  :disabled="isGeneratingPlan || !hasMasterPlanner || !canGeneratePlan"
                  class="px-3.5 py-1.5 rounded flex items-center gap-1.5 shadow transition font-bold text-body whitespace-nowrap"
                  :class="canGeneratePlan 
                    ? 'bg-gradient-to-r from-gear-gold to-amber-500 hover:from-amber-400 hover:to-gear-gold text-ink cursor-pointer' 
                    : 'bg-slate-dark text-zinc-500 border border-steel/60 opacity-60 cursor-not-allowed'"
                  :title="canGeneratePlan ? 'The Master has enough detail. Generate the plan.' : 'The Master is still asking questions. Keep going to unlock this.'"
                >
                  <svg v-if="isGeneratingPlan" class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                    <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"></path>
                  </svg>
                  <span v-else>{{ canGeneratePlan ? '⚡' : '🔒' }}</span>
                  <span>{{ isGeneratingPlan ? 'Generating plan…' : 'Generate plan' }}</span>
                </button>
              </div>
            </div>

            <!-- Input Form -->
            <form @submit.prevent="sendMessage" class="flex items-end gap-3">
              <div class="flex-1 relative">
                <textarea 
                  v-model="inputMsg"
                  :disabled="selectedTarget === 'master' && !hasMasterPlanner"
                  rows="2"
                  :style="{ height: inputHeight + 'px' }"
                  @keydown.enter.exact.prevent="sendMessage"
                  @keydown.enter.shift.exact="inputMsg += '\n'"
                  :placeholder="selectedTarget === 'master' 
                    ? (hasMasterPlanner ? 'Message the Master…\nEnter to send, Shift+Enter for a new line' : 'Chat is off: set a Master in Providers first.')
                    : `Message automaton '${selectedTarget}'…\nEnter to send, Shift+Enter for a new line`"
                  class="w-full bg-slate-inner border border-steel text-parchment text-body px-4 py-2.5 rounded focus:outline-none focus:border-gear-gold disabled:opacity-40 disabled:cursor-not-allowed resize-none leading-relaxed"
                ></textarea>
              </div>
              <button 
                type="submit" 
                :disabled="(selectedTarget === 'master' && !hasMasterPlanner) || !inputMsg.trim()"
                class="px-5 py-3 bg-cyan-dim hover:bg-cyan-energy text-ink font-bold text-body rounded transition flex items-center gap-2 disabled:opacity-40 disabled:cursor-not-allowed h-[52px] shadow-sm mb-1"
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
            <span class="text-body font-bold text-zinc-300">Project Context</span>
            <span class="text-2xs px-2 py-0.5 rounded bg-slate-inner text-gear-gold border border-steel">
              {{ project.isBlank ? 'No plan yet' : 'Plan ready' }}
            </span>
          </div>

          <div class="flex-1 p-4 pb-24 overflow-y-auto space-y-4">
            <!-- Parallel Workers (Show if active) -->
            <div v-if="activeWorkersList.length > 0" class="space-y-2">
              <span class="text-xs uppercase font-bold text-zinc-400">Working automatons</span>
              <div 
                v-for="worker in activeWorkersList"
                :key="worker.taskId"
                class="bg-slate-dark border border-steel rounded p-3 hover:border-cyan-dim transition cursor-pointer"
                @click="selectedTarget = worker.id"
              >
                <div class="flex items-center justify-between mb-1">
                  <span class="text-xs text-zinc-300 font-bold">{{ worker.name }}</span>
                  <span class="text-2xs font-bold px-1.5 py-0.2 rounded border" :class="statusClass(worker.status)">
                    {{ statusLabel(worker.status) }}
                  </span>
                </div>
                <p class="text-body font-semibold text-zinc-200">{{ worker.taskId }}: {{ worker.taskTitle }}</p>
                <div class="flex items-center justify-between text-2xs text-zinc-400 mt-2">
                  <span>Branch: <code class="text-cyan-energy">rm/{{ worker.taskId }}</code></span>
                  <span>Attempt: {{ worker.attempt }}</span>
                </div>
              </div>
            </div>

            <div v-else class="p-3 bg-slate-dark/60 border border-steel/60 rounded text-center">
              <p class="text-body text-zinc-400">No automaton is working.</p>
              <p class="text-2xs text-zinc-500 mt-1">Tasks start after you approve a plan and run it.</p>
            </div>

            <!-- BRIEF.md Preview -->
            <div class="pt-2 border-t border-steel">
              <div class="flex items-center justify-between mb-2">
                <span class="text-body font-bold text-zinc-300">BRIEF.md</span>
                <span class="text-2xs text-zinc-500">{{ project.brief ? 'Found' : 'Empty' }}</span>
              </div>
              <div class="p-3 bg-slate-inner border border-steel rounded text-xs leading-relaxed text-zinc-400 max-h-56 overflow-y-auto">
                <MarkdownText v-if="project.brief" :source="project.brief" />
                <p v-else>Empty. The brief is written from your conversation with the Master.</p>
              </div>
            </div>
          </div>
        </div>
      </section>
</template>

<script setup>
import { statusLabel, statusClass } from '~/utils/status'
const {
  activeView,
  selectedTarget,
  inputMsg,
  setView,
  project,
  hasMasterPlanner,
  messages,
  isThinking,
  chatContainerRef,
  canGeneratePlan,
  tasks,
  activeWorkersList,
  isRunActive,
  completedTasksCount,
  isGeneratingPlan,
  triggerGeneratePlan,
  sendMessage,
  instructWorker
} = useRingmaster()

// Resizable input: dragging the panel's top edge up grows the textarea and
// shrinks the message list. Height is remembered per browser.
const INPUT_HEIGHT_KEY = 'rm-chat-input-height'
const MIN_INPUT_HEIGHT = 52
const DEFAULT_INPUT_HEIGHT = 64

const inputHeight = ref(DEFAULT_INPUT_HEIGHT)
const isResizing = ref(false)

const maxInputHeight = () => Math.max(MIN_INPUT_HEIGHT, Math.round(window.innerHeight * 0.7))
const clampHeight = (h) => Math.min(maxInputHeight(), Math.max(MIN_INPUT_HEIGHT, Math.round(h)))

const saveInputHeight = () => {
  try {
    localStorage.setItem(INPUT_HEIGHT_KEY, String(inputHeight.value))
  } catch {}
}

const startResize = (e) => {
  e.preventDefault()
  const startY = e.clientY
  const startHeight = inputHeight.value
  isResizing.value = true
  document.body.style.cursor = 'row-resize'
  document.body.style.userSelect = 'none'

  const onMove = (ev) => {
    inputHeight.value = clampHeight(startHeight + (startY - ev.clientY))
  }
  const onUp = () => {
    isResizing.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    saveInputHeight()
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}

const resetInputHeight = () => {
  inputHeight.value = DEFAULT_INPUT_HEIGHT
  saveInputHeight()
}

// The chat opens at the newest message and follows new ones, unless the user
// has scrolled up to read; then new messages are counted on a jump button.
const atBottom = ref(true)
const unseen = ref(0)

function onChatScroll() {
  const el = chatContainerRef.value
  if (!el) return
  atBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80
  if (atBottom.value) unseen.value = 0
}

function jumpToLatest() {
  const el = chatContainerRef.value
  if (el) el.scrollTop = el.scrollHeight
  atBottom.value = true
  unseen.value = 0
}

watch(() => messages.value.length, (n, prev) => {
  const last = messages.value[n - 1]
  // Always follow your own message; otherwise only when already at the bottom
  if (atBottom.value || last?.role === 'user') nextTick(jumpToLatest)
  else if (n > prev) unseen.value += n - prev
})
watch(isThinking, () => { if (atBottom.value) nextTick(jumpToLatest) })
watch(inputHeight, () => { if (atBottom.value) nextTick(jumpToLatest) })

onMounted(() => {
  try {
    const saved = Number(localStorage.getItem(INPUT_HEIGHT_KEY))
    if (saved) inputHeight.value = clampHeight(saved)
  } catch {}
  nextTick(jumpToLatest)
})
</script>
