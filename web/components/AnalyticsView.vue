<template>
      <section class="flex-1 flex flex-col p-6 pb-24 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Analytics</h2>
          <p class="text-body text-zinc-400">Token usage, cost and time per task and per agent.</p>
        </div>

        <!-- Summary -->
        <div class="grid gap-4 mb-6" style="grid-template-columns: repeat(auto-fit, minmax(180px, 1fr))">
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-xs text-zinc-400 uppercase">Total Tokens</span>
            <p class="text-xl font-bold text-gear-gold mt-1 tabular-nums">{{ fmt(totals.total) }}</p>
            <p class="text-xs text-zinc-500 mt-1 tabular-nums">
              {{ fmt(totals.in) }} in · {{ fmt(totals.out) }} out
              <span v-if="totals.cached"> · {{ pct(totals.cached, totals.in) }} cached</span>
            </p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-xs text-zinc-400 uppercase">Cost</span>
            <p class="text-xl font-bold text-zinc-200 mt-1 tabular-nums">{{ totals.cost ? usd(totals.cost) : '—' }}</p>
            <p class="text-xs text-zinc-500 mt-1">{{ totals.cost ? 'as reported by providers' : 'no provider reported a cost' }}</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-xs text-zinc-400 uppercase">Automaton time</span>
            <p class="text-xl font-bold text-zinc-200 mt-1 tabular-nums">{{ totals.ms ? duration(totals.ms) : '—' }}</p>
            <p class="text-xs text-zinc-500 mt-1 tabular-nums">{{ fmt(totals.calls) }} LLM calls</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-xs text-zinc-400 uppercase">Tasks Completed</span>
            <p class="text-xl font-bold text-zinc-200 mt-1 tabular-nums">{{ completedTasksCount }} / {{ tasks.length }}</p>
            <p class="text-xs text-zinc-500 mt-1">{{ project.isBlank ? 'no plan yet' : statusLabel(project.state?.status || 'READY') }}</p>
          </div>
        </div>

        <!-- Per task -->
        <div class="bg-slate-panel border border-steel rounded-lg mb-6 overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2.5 border-b border-steel">
            <h3 class="text-body font-bold text-zinc-200">Usage per Task</h3>
            <div class="flex items-center gap-1 text-xs">
              <span class="text-zinc-500 mr-1">Sort:</span>
              <button
                v-for="opt in SORTS"
                :key="opt.id"
                @click="sortBy = opt.id"
                class="px-2 py-0.5 rounded border font-bold"
                :class="sortBy === opt.id ? 'bg-gear-gold text-ink border-gear-gold' : 'border-steel text-zinc-400 hover:text-zinc-200'"
              >{{ opt.label }}</button>
            </div>
          </div>

          <div v-if="taskRows.length === 0" class="p-6 text-center text-body text-zinc-500">No tasks yet. Generate and approve a plan first.</div>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-body font-mono">
              <thead>
                <tr class="text-xs uppercase text-zinc-500 border-b border-steel">
                  <th class="text-left font-semibold px-4 py-2">Task</th>
                  <th class="text-left font-semibold px-2 py-2">Automaton</th>
                  <th class="text-left font-semibold px-2 py-2">Status</th>
                  <th class="text-right font-semibold px-2 py-2" title="Attempts in the latest run">Att.</th>
                  <th class="text-right font-semibold px-2 py-2">Calls</th>
                  <th class="text-right font-semibold px-2 py-2" title="All input the model processed, including prompt-cache reads">Input</th>
                  <th class="text-right font-semibold px-2 py-2">Output</th>
                  <th class="text-left font-semibold px-2 py-2 w-48">Total</th>
                  <th class="text-right font-semibold px-2 py-2">Time</th>
                  <th class="text-right font-semibold px-4 py-2">Cost</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in taskRows"
                  :key="r.id"
                  @click="openTask(r.id)"
                  class="border-b border-steel/50 hover:bg-slate-inner/60 cursor-pointer"
                  title="Open task details"
                >
                  <td class="px-4 py-2 max-w-[280px]">
                    <div class="flex items-baseline gap-2 min-w-0">
                      <span class="text-gear-gold font-bold shrink-0">{{ r.id }}</span>
                      <span class="text-zinc-300 truncate">{{ r.title }}</span>
                    </div>
                  </td>
                  <td class="px-2 py-2 text-cyan-energy whitespace-nowrap">{{ r.worker || '—' }}</td>
                  <td class="px-2 py-2">
                    <span class="text-2xs px-1.5 py-0.5 rounded border font-bold whitespace-nowrap" :class="statusClass(r.status)">{{ statusLabel(r.status) }}</span>
                  </td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400">{{ r.attempt || '—' }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400">{{ r.calls ? fmt(r.calls) : '—' }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300 whitespace-nowrap">
                    {{ r.in ? fmt(r.in) : '—' }}
                    <span v-if="r.cached" class="text-2xs text-zinc-500">({{ pct(r.cached, r.in) }} cached)</span>
                  </td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300">{{ r.out ? fmt(r.out) : '—' }}</td>
                  <td class="px-2 py-2">
                    <div class="flex items-center gap-2">
                      <div class="flex-1 h-1.5 rounded-full bg-slate-inner overflow-hidden">
                        <div class="h-full rounded-full bg-gear-gold" :style="{ width: barWidth(r.total) }"></div>
                      </div>
                      <span class="tabular-nums text-zinc-200 font-bold w-14 text-right">{{ r.total ? fmt(r.total) : '—' }}</span>
                    </div>
                  </td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400 whitespace-nowrap">{{ r.ms ? duration(r.ms) : '—' }}</td>
                  <td class="px-4 py-2 text-right tabular-nums text-zinc-400">{{ r.cost ? usd(r.cost) : '—' }}</td>
                </tr>
              </tbody>
              <tfoot>
                <tr class="text-zinc-200 font-bold">
                  <td class="px-4 py-2" colspan="4">Total · {{ taskRows.length }} tasks</td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ fmt(taskTotals.calls) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ fmt(taskTotals.in) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ fmt(taskTotals.out) }}</td>
                  <td class="px-2 py-2 tabular-nums"><span class="block text-right">{{ fmt(taskTotals.total) }}</span></td>
                  <td class="px-2 py-2 text-right tabular-nums">{{ taskTotals.ms ? duration(taskTotals.ms) : '—' }}</td>
                  <td class="px-4 py-2 text-right tabular-nums">{{ taskTotals.cost ? usd(taskTotals.cost) : '—' }}</td>
                </tr>
              </tfoot>
            </table>
          </div>
        </div>

        <!-- Per agent -->
        <div class="bg-slate-panel border border-steel rounded-lg overflow-hidden">
          <div class="px-4 py-2.5 border-b border-steel flex items-center justify-between">
            <h3 class="text-body font-bold text-zinc-200">Usage per Agent</h3>
            <span v-if="planning.total" class="text-xs text-zinc-500 tabular-nums">
              Master planning: {{ fmt(planning.total) }} tokens · {{ planning.calls }} call{{ planning.calls === 1 ? '' : 's' }}
            </span>
          </div>
          <div v-if="agentRows.length === 0" class="p-6 text-center text-body text-zinc-500">No automaton has run a task yet.</div>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-body font-mono">
              <thead>
                <tr class="text-xs uppercase text-zinc-500 border-b border-steel">
                  <th class="text-left font-semibold px-4 py-2">Automaton</th>
                  <th class="text-right font-semibold px-2 py-2">Tasks</th>
                  <th class="text-right font-semibold px-2 py-2">Calls</th>
                  <th class="text-right font-semibold px-2 py-2">Input</th>
                  <th class="text-right font-semibold px-2 py-2">Output</th>
                  <th class="text-left font-semibold px-2 py-2 w-48">Total</th>
                  <th class="text-right font-semibold px-2 py-2">Avg / task</th>
                  <th class="text-right font-semibold px-4 py-2">Cost</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="a in agentRows" :key="a.name" class="border-b border-steel/50 last:border-0">
                  <td class="px-4 py-2 text-cyan-energy">{{ a.name }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400">{{ a.tasks }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400">{{ fmt(a.calls) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300">{{ fmt(a.in) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300">{{ fmt(a.out) }}</td>
                  <td class="px-2 py-2">
                    <div class="flex items-center gap-2">
                      <div class="flex-1 h-1.5 rounded-full bg-slate-inner overflow-hidden">
                        <div class="h-full rounded-full bg-cyan-dim" :style="{ width: share(a.total, maxAgentTotal) }"></div>
                      </div>
                      <span class="tabular-nums text-zinc-200 font-bold w-14 text-right">{{ fmt(a.total) }}</span>
                    </div>
                  </td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-400">{{ fmt(Math.round(a.total / a.tasks)) }}</td>
                  <td class="px-4 py-2 text-right tabular-nums text-zinc-400">{{ a.cost ? usd(a.cost) : '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
</template>

<script setup>
import { statusLabel, statusClass } from '~/utils/status'
const {
  project,
  tasks,
  completedTasksCount,
  setView,
  openTaskDetail
} = useRingmaster()

const SORTS = [
  { id: 'id', label: 'Task' },
  { id: 'total', label: 'Tokens' },
  { id: 'ms', label: 'Time' },
  { id: 'cost', label: 'Cost' }
]
const sortBy = ref('id')

// Ticks so running tasks' durations stay current
const now = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { now.value = Date.now() }, 1000) })
onUnmounted(() => clearInterval(timer))

function taskMs(ts) {
  if (!ts?.start_at) return 0
  const start = new Date(ts.start_at).getTime()
  const end = ts.status === 'RUNNING' ? now.value : ts.finish_at ? new Date(ts.finish_at).getTime() : 0
  // A re-dispatched task gets a new start_at but keeps its old finish_at until it ends
  return end > start ? end - start : 0
}

const taskRows = computed(() => {
  const states = project.value.state?.task_states || {}
  const rows = (project.value.plan?.tasks || []).map((t) => {
    const ts = states[t.id] || {}
    const u = ts.usage || {}
    const inTok = u.tokens_in || 0
    const outTok = u.tokens_out || 0
    return {
      id: t.id,
      title: t.title,
      worker: ts.worker || t.worker,
      status: ts.status || 'PENDING',
      attempt: ts.attempt || 0,
      calls: u.calls || 0,
      in: inTok,
      out: outTok,
      cached: u.tokens_cached || 0,
      total: inTok + outTok,
      cost: u.cost_usd || 0,
      ms: taskMs(ts)
    }
  })
  if (sortBy.value !== 'id') rows.sort((a, b) => b[sortBy.value] - a[sortBy.value])
  return rows
})

const sum = (rows, key) => rows.reduce((n, r) => n + (r[key] || 0), 0)
const taskTotals = computed(() => {
  const rows = taskRows.value
  return { calls: sum(rows, 'calls'), in: sum(rows, 'in'), out: sum(rows, 'out'), cached: sum(rows, 'cached'), total: sum(rows, 'total'), cost: sum(rows, 'cost'), ms: sum(rows, 'ms') }
})

const planning = computed(() => {
  const p = project.value.state?.planning_usage || {}
  return { calls: p.calls || 0, in: p.tokens_in || 0, out: p.tokens_out || 0, cached: p.tokens_cached || 0, total: (p.tokens_in || 0) + (p.tokens_out || 0), cost: p.cost_usd || 0 }
})

// Project totals = tasks + master planning
const totals = computed(() => {
  const t = taskTotals.value
  const p = planning.value
  return { in: t.in + p.in, out: t.out + p.out, cached: t.cached + p.cached, total: t.total + p.total, cost: t.cost + p.cost, calls: t.calls + p.calls, ms: t.ms }
})

const agentRows = computed(() => {
  const byAgent = {}
  for (const r of taskRows.value) {
    if (!r.calls && !r.total) continue
    const name = r.worker || 'unassigned'
    const a = byAgent[name] || (byAgent[name] = { name, tasks: 0, calls: 0, in: 0, out: 0, total: 0, cost: 0 })
    a.tasks++
    a.calls += r.calls
    a.in += r.in
    a.out += r.out
    a.total += r.total
    a.cost += r.cost
  }
  return Object.values(byAgent).sort((a, b) => b.total - a.total)
})

const maxTaskTotal = computed(() => Math.max(1, ...taskRows.value.map((r) => r.total)))
const maxAgentTotal = computed(() => Math.max(1, ...agentRows.value.map((a) => a.total)))
const share = (n, max) => `${Math.max(n ? 2 : 0, Math.round((n / max) * 100))}%`
const barWidth = (n) => share(n, maxTaskTotal.value)

function fmt(n) {
  if (!n) return '0'
  if (n >= 1e6) return `${(n / 1e6).toFixed(n >= 1e7 ? 1 : 2)}M`
  if (n >= 1e4) return `${(n / 1e3).toFixed(n >= 1e5 ? 0 : 1)}k`
  return n.toLocaleString()
}
const pct = (part, whole) => (whole ? `${Math.round((part / whole) * 100)}%` : '0%')
const usd = (n) => (n < 0.01 ? '<$0.01' : `$${n.toFixed(2)}`)
function duration(ms) {
  const s = Math.round(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${String(s % 60).padStart(2, '0')}s`
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m`
}

// Task details live on the Tasks view
function openTask(id) {
  setView('tasks')
  openTaskDetail(id)
}
</script>
