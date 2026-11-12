<template>
      <section class="flex-1 flex flex-col p-6 overflow-hidden">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">Logs</h2>
            <p class="text-body text-zinc-400">Output of the run, with a lane for each working automaton.</p>
          </div>
          <div class="flex items-center gap-3">
            <button
              v-if="isRunActive"
              @click="stopOrchestration"
              class="px-3 py-2 bg-red-950/80 hover:bg-red-900 border border-red-700/60 text-red-200 text-body font-bold rounded transition"
              title="Stop the run"
            >
              ⏹ Stop
            </button>
            <button
              :disabled="isRunActive || runLogs.length === 0"
              @click="clearRunLogs"
              class="px-3.5 py-2 bg-slate-panel hover:bg-slate-panel/70 text-zinc-300 font-bold text-body rounded border border-steel transition disabled:opacity-40 disabled:cursor-not-allowed"
              :title="isRunActive ? 'Logs can be cleared once the run has exited.' : 'Start a fresh log; the current one is archived in .ringmaster/logs/'"
            >
              Clear logs
            </button>
          </div>
        </div>

        <!-- Run Status Strip -->
        <div class="mb-3 p-3 rounded-lg border border-steel/80 bg-slate-panel flex flex-wrap items-center gap-x-6 gap-y-2 text-body">
          <div class="flex items-center gap-2">
            <span class="relative flex w-2.5 h-2.5">
              <span v-if="isRunActive" class="absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-60 animate-ping"></span>
              <span class="relative inline-flex w-2.5 h-2.5 rounded-full" :class="isRunActive ? 'bg-cyan-400' : 'bg-zinc-500'"></span>
            </span>
            <span class="font-bold" :class="isRunActive ? 'text-cyan-300' : 'text-zinc-400'">{{ isRunActive ? 'Running' : 'Idle' }}</span>
          </div>
          <span class="text-zinc-400">PID: <strong class="text-zinc-200">{{ isRunActive ? (runPid || 'Live') : 'Exited' }}</strong></span>
          <span class="text-zinc-400">Project: <strong class="text-gear-gold">{{ statusLabel(project.state?.status || 'READY') }}</strong></span>
          <span class="text-zinc-400">Automatons working: <strong :class="activeWorkersList.length > 0 ? 'text-green-400' : 'text-zinc-500'">{{ activeWorkersList.length }}</strong></span>
          <span class="text-zinc-400">Tasks done: <strong class="text-zinc-200">{{ completedTasksCount }} / {{ tasks.length }}</strong></span>
          <span v-if="isRunActive && lastOutputAgo" class="text-zinc-400">Last output: <strong class="text-zinc-200">{{ lastOutputAgo }}</strong></span>
        </div>


        <!-- Working automaton lanes: what each worker is doing right now -->
        <div v-if="workerLanes.length" class="mb-3 grid gap-2" style="grid-template-columns: repeat(auto-fill, minmax(260px, 1fr))">
          <button
            v-for="lane in workerLanes"
            :key="lane.taskId"
            @click="selectedTask = selectedTask === lane.taskId ? '' : lane.taskId"
            class="text-left p-2.5 rounded-lg border bg-slate-panel transition"
            :class="selectedTask === lane.taskId ? lane.color.border : 'border-steel/80 hover:border-steel'"
            :title="`Filter logs to ${lane.taskId}`"
          >
            <div class="flex items-center gap-2 text-body">
              <span class="relative flex w-2 h-2 shrink-0">
                <span class="absolute inline-flex h-full w-full rounded-full opacity-60 animate-ping" :class="lane.color.dot"></span>
                <span class="relative inline-flex w-2 h-2 rounded-full" :class="lane.color.dot"></span>
              </span>
              <span class="font-bold" :class="lane.color.text">{{ lane.taskId }}</span>
              <span class="text-zinc-300 truncate">{{ lane.worker }}</span>
              <span class="ml-auto text-zinc-500 shrink-0">attempt {{ lane.attempt }}</span>
              <span class="font-bold text-zinc-200 tabular-nums shrink-0">{{ lane.elapsed }}</span>
            </div>
            <div class="mt-1.5 text-xs truncate" :class="lane.last ? 'text-zinc-400' : 'text-zinc-500 italic'">
              <template v-if="lane.last">
                <span class="mr-1">{{ lane.last.icon }}</span>{{ lane.last.text }}
                <span class="text-zinc-500"> · {{ lane.lastAgo }}</span>
              </template>
              <template v-else>waiting for first step…</template>
            </div>
          </button>
        </div>

        <!-- Task filter -->
        <div v-if="seenTasks.length" class="mb-2 flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
          <span class="text-zinc-500 mr-1 shrink-0">Show:</span>
          <button
            @click="selectedTask = ''"
            class="px-2 py-0.5 rounded border font-bold shrink-0"
            :class="selectedTask === '' ? 'bg-gear-gold text-ink border-gear-gold' : 'border-steel text-zinc-400 hover:text-zinc-200'"
          >All</button>
          <button
            v-for="id in seenTasks"
            :key="id"
            @click="selectedTask = selectedTask === id ? '' : id"
            class="px-2 py-0.5 rounded border font-bold shrink-0"
            :class="selectedTask === id ? [taskColor(id).bg, taskColor(id).border, taskColor(id).text] : 'border-steel text-zinc-400 hover:text-zinc-200'"
          >{{ id }}</button>
        </div>

        <!-- Console -->
        <div class="relative flex-1 min-h-0">
          <div
            ref="consoleRef"
            @scroll="onConsoleScroll"
            class="h-full bg-slate-inner/90 py-2 rounded-lg font-mono text-body leading-relaxed overflow-y-auto border border-steel/40 select-text"
          >
            <div v-if="visibleLines.length === 0" class="h-full flex items-center justify-center text-zinc-500 text-body">
              {{ runLogs.length === 0 ? 'No output yet. Run an approved plan from Tasks.' : `No output for ${selectedTask} yet.` }}
            </div>
            <template v-for="line in visibleLines" :key="line.idx">
            <!-- Run boundary -->
            <div v-if="line.run" class="flex items-center gap-2 px-3 my-2 text-xs text-zinc-500 select-none">
              <span class="tabular-nums">{{ line.date }} {{ line.time }}</span>
              <span class="flex-1 border-t border-dashed border-steel"></span>
              <span class="font-bold uppercase tracking-wide" :class="line.cls">{{ line.text }}</span>
              <span class="flex-1 border-t border-dashed border-steel"></span>
            </div>
            <div
              v-else
              class="flex items-start gap-2 px-3 py-[1px] hover:bg-slate-panel/50"
              :class="line.emphasis ? 'mt-1' : ''"
            >
              <span class="text-zinc-600 select-none tabular-nums shrink-0 w-[66px]">{{ line.time }}</span>
              <span class="shrink-0 w-[44px]">
                <span v-if="line.task" class="px-1 rounded text-2xs font-bold border" :class="[taskColor(line.task).bg, taskColor(line.task).border, taskColor(line.task).text]">{{ line.task }}</span>
              </span>
              <span class="shrink-0 w-4 text-center select-none">{{ line.icon }}</span>
              <span class="break-all whitespace-pre-wrap min-w-0" :class="line.cls">{{ line.text }}</span>
            </div>
            </template>
          </div>

          <button
            v-if="!followTail && newSincePause > 0"
            @click="jumpToBottom"
            class="absolute bottom-3 left-1/2 -translate-x-1/2 px-3 py-1 rounded-full bg-gear-gold text-ink text-xs font-bold shadow-lg"
          >↓ {{ newSincePause }} new line{{ newSincePause === 1 ? '' : 's' }}</button>
        </div>
      </section>
</template>

<script setup>
import { statusLabel } from '~/utils/status'
const {
  project,
  tasks,
  activeWorkersList,
  completedTasksCount,
  isRunActive,
  runPid,
  runLogs,
  runLogTimes,
  clearRunLogs,
  stopOrchestration
} = useRingmaster()

// Per-task colors; listed literally so Tailwind keeps every class
const TASK_COLORS = [
  { text: 'text-cyan-300', bg: 'bg-cyan-950', border: 'border-cyan-700', dot: 'bg-cyan-400' },
  { text: 'text-indigo-300', bg: 'bg-indigo-950', border: 'border-indigo-700', dot: 'bg-indigo-400' },
  { text: 'text-emerald-300', bg: 'bg-emerald-950', border: 'border-emerald-700', dot: 'bg-emerald-400' },
  { text: 'text-amber-300', bg: 'bg-amber-950', border: 'border-amber-700', dot: 'bg-amber-400' },
  { text: 'text-rose-300', bg: 'bg-rose-950', border: 'border-rose-700', dot: 'bg-rose-400' },
  { text: 'text-teal-300', bg: 'bg-teal-950', border: 'border-teal-700', dot: 'bg-teal-400' }
]
function taskColor(id) {
  const n = parseInt(String(id).replace(/\D/g, ''), 10) || 0
  return TASK_COLORS[n % TASK_COLORS.length]
}

const STEP_STYLE = {
  exec: { icon: '▶', cls: 'text-zinc-200' },
  read: { icon: '📖', cls: 'text-zinc-400' },
  search: { icon: '🔎', cls: 'text-zinc-400' },
  write: { icon: '📝', cls: 'text-emerald-300' },
  edit: { icon: '✏️', cls: 'text-emerald-300' },
  says: { icon: '💬', cls: 'text-zinc-300 italic' },
  call: { icon: '🔧', cls: 'text-zinc-300' }
}

// Turn one raw engine line into {task, icon, text, cls}
function parseLine(raw) {
  // Run boundaries written by `ringmaster run`: "[Run] Run started · …" / "[Run] Run ended · …"
  if (raw.startsWith('[Run]')) {
    const text = raw.slice(5).trim()
    const cls = /failed/i.test(text) ? 'text-rose-400' : /interrupted/i.test(text) ? 'text-amber-300' : 'text-zinc-400'
    return { task: null, run: true, icon: '', cls, text }
  }
  // Worker step: "[T06] step 3: exec `go test ./...`" (or "turn 3:" for tool-calling workers)
  const step = raw.match(/^\[(T\d{2,3})\] (?:step|turn) \d+: (\w+) ?(.*)$/)
  if (step) {
    const style = STEP_STYLE[step[2]] || STEP_STYLE.call
    const text = step[3].replace(/^`(.*)`$/, '$1') || step[2]
    return { task: step[1], icon: style.icon, cls: style.cls, text: step[2] === 'says' ? text : `${step[2]} ${text}` }
  }
  if (raw.startsWith('[stderr]')) {
    return { task: null, icon: '✗', cls: 'text-rose-400', text: raw.slice(8).trim() }
  }
  if (raw.startsWith('[Ringmaster UI]')) {
    return { task: null, icon: '›', cls: 'text-zinc-500', text: raw.slice(15).trim() }
  }

  const task = (raw.match(/\b(T\d{2,3})\b/) || [])[1] || null
  const text = raw.replace(/^\s*\S*\s*\[Scheduler\]\s*/, '').trim()
  const rules = [
    ['⚡', '⚡', 'text-cyan-300 font-semibold', true],
    ['★', '★', 'text-green-400 font-semibold', true],
    ['✓', '✓', 'text-green-400 font-semibold', true],
    ['❌', '✗', 'text-rose-400 font-semibold', true],
    ['⚠', '⚠', 'text-amber-300 font-semibold', true],
    ['🙋', '🙋', 'text-amber-300 font-semibold', true]
  ]
  for (const [marker, icon, cls, emphasis] of rules) {
    if (raw.includes(marker)) return { task, icon, cls, text, emphasis }
  }
  return { task, icon: '·', cls: raw.startsWith(' ') ? 'text-zinc-500' : 'text-zinc-300', text: text || raw }
}

const pad = (n) => String(n).padStart(2, '0')
function clock(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
function shortDate(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}
function duration(ms) {
  const s = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h ? `${h}h ${pad(m)}m` : `${m}:${pad(s % 60)}`
}
function ago(iso) {
  if (!iso) return ''
  const s = Math.max(0, Math.floor((now.value - new Date(iso).getTime()) / 1000))
  if (s < 5) return 'just now'
  if (s < 60) return `${s}s ago`
  return `${Math.floor(s / 60)}m ${pad(s % 60)}s ago`
}

const parsedLines = computed(() =>
  runLogs.value.map((raw, idx) => {
    const iso = runLogTimes.value[idx] || null
    return { idx, iso, time: clock(iso), date: shortDate(iso), ...parseLine(raw) }
  })
)

const selectedTask = ref('')
// Filtering by task keeps the boundaries of only the runs that touched it
const visibleLines = computed(() => {
  if (!selectedTask.value) return parsedLines.value
  const out = []
  let segment = []
  let touched = false
  const flush = () => {
    if (touched) out.push(...segment)
    segment = []
    touched = false
  }
  for (const l of parsedLines.value) {
    if (l.run && l.text.startsWith('Run started')) flush()
    if (l.run) segment.push(l)
    else if (l.task === selectedTask.value) {
      segment.push(l)
      touched = true
    }
  }
  flush()
  return out
})
const seenTasks = computed(() => {
  const ids = new Set(parsedLines.value.map((l) => l.task).filter(Boolean))
  return [...ids].sort()
})

// Ticking clock for elapsed times
const now = ref(Date.now())
let clockTimer = null

const lastByTask = computed(() => {
  const last = {}
  for (const l of parsedLines.value) if (l.task) last[l.task] = l
  return last
})

const workerLanes = computed(() =>
  activeWorkersList.value.map((w) => {
    const last = lastByTask.value[w.taskId] || null
    return {
      taskId: w.taskId,
      worker: w.name,
      attempt: w.attempt,
      color: taskColor(w.taskId),
      elapsed: w.startAt ? duration(now.value - new Date(w.startAt).getTime()) : '—',
      last,
      lastAgo: last ? ago(last.iso) : ''
    }
  })
)

const lastOutputAgo = computed(() => ago(runLogTimes.value[runLogTimes.value.length - 1]))

// Follow the tail unless the user scrolled up to read; then count what arrives
const consoleRef = ref(null)
const followTail = ref(true)
const newSincePause = ref(0)

function onConsoleScroll() {
  const el = consoleRef.value
  if (!el) return
  followTail.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40
  if (followTail.value) newSincePause.value = 0
}
function jumpToBottom() {
  const el = consoleRef.value
  if (el) el.scrollTop = el.scrollHeight
  followTail.value = true
  newSincePause.value = 0
}

// runLogs is replaced on every poll that brings output; at the buffer cap the
// length stops growing, so compare the newest entry instead of the length
watch(runLogs, (lines, prev) => {
  if (followTail.value) return nextTick(jumpToBottom)
  const prevLast = prev?.[prev.length - 1]
  const at = prevLast === undefined ? -1 : lines.lastIndexOf(prevLast)
  newSincePause.value += at < 0 ? lines.length : lines.length - 1 - at
})
watch(selectedTask, () => nextTick(jumpToBottom))

onMounted(() => {
  nextTick(jumpToBottom)
  clockTimer = setInterval(() => { now.value = Date.now() }, 1000)
})
onUnmounted(() => clearInterval(clockTimer))
</script>
