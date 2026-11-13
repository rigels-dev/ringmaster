<template>
      <section class="flex-1 flex flex-col p-6 pb-24 overflow-y-auto gap-4">
        <div class="flex items-end justify-between gap-3 flex-wrap">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">Branches</h2>
            <p class="text-body text-zinc-400">Every task branch, where it forked, what each automaton committed, and when it merged into {{ data.main || 'main' }}.</p>
          </div>
          <button @click="load" class="px-3 py-1.5 bg-slate-panel hover:bg-slate-inner text-zinc-300 text-body rounded border border-steel transition">↻ Refresh</button>
        </div>

        <p v-if="error" class="p-3 rounded border border-rose-700 bg-rose-950 text-rose-300 text-body">{{ error }}</p>

        <!-- Summary -->
        <div v-if="branches.length" class="p-3 rounded-lg border border-steel bg-slate-panel flex flex-wrap gap-x-6 gap-y-2 text-body font-mono text-zinc-400">
          <span>Branches <strong class="text-zinc-200">{{ branches.length }}</strong></span>
          <span>Merged <strong class="text-zinc-200">{{ mergedCount }}</strong></span>
          <span>Open <strong class="text-zinc-200">{{ branches.length - mergedCount }}</strong></span>
          <span>Task commits <strong class="text-zinc-200">{{ taskCommitCount }}</strong></span>
          <span>Automatons <strong class="text-cyan-energy">{{ allAutomatons.join(', ') || '—' }}</strong></span>
          <span v-if="avgLifetime">Avg. fork → merge <strong class="text-zinc-200">{{ avgLifetime }}</strong></span>
          <span v-if="data.truncated" class="text-amber-300">Showing the newest {{ commits.length }} commits</span>
        </div>

        <div v-if="!error && !loading && branches.length === 0" class="p-8 text-center text-body text-zinc-500 border border-dashed border-steel rounded-lg">
          No task branches yet. Each task gets an <code class="text-zinc-300">rm/Txx</code> branch when its automaton starts working.
        </div>

        <div v-if="branches.length" class="grid gap-4 items-start" style="grid-template-columns: minmax(0, 1fr) 320px">
          <!-- Graph + commit rows -->
          <div class="bg-slate-panel border border-steel rounded-lg min-w-0">
            <div class="flex items-center justify-between gap-2 px-4 py-2.5 border-b border-steel">
              <h3 class="text-body font-bold text-zinc-200">Commit graph</h3>
              <div class="flex flex-wrap gap-1 text-xs">
                <button
                  @click="focus = ''"
                  class="px-2 py-0.5 rounded border font-bold"
                  :class="focus === '' ? 'bg-gear-gold text-ink border-gear-gold' : 'border-steel text-zinc-400 hover:text-zinc-200'"
                >All</button>
                <button
                  v-for="b in branches"
                  :key="b.name"
                  @click="focus = focus === b.name ? '' : b.name"
                  class="px-2 py-0.5 rounded border font-bold"
                  :class="focus === b.name ? 'bg-gear-gold text-ink border-gear-gold' : 'border-steel text-zinc-400 hover:text-zinc-200'"
                >{{ b.task }}</button>
              </div>
            </div>
            <div class="overflow-x-auto">
              <div class="relative min-w-[560px]" :style="{ height: commits.length * ROW + 'px' }">
                <svg class="absolute left-0 top-0 pointer-events-none" :width="lanesWidth" :height="commits.length * ROW" aria-hidden="true">
                  <!-- main -->
                  <line :x1="laneX(0)" :y1="ROW / 2" :x2="laneX(0)" :y2="(commits.length - 0.5) * ROW" :stroke="MAIN_COLOR" stroke-width="2.5" />
                  <g v-for="seg in segments" :key="seg.name" :opacity="isDim(seg.name) ? 0.15 : 1">
                    <path v-if="seg.forkPath" :d="seg.forkPath" :stroke="seg.color" stroke-width="2" fill="none" />
                    <line :x1="seg.x" :y1="seg.y1" :x2="seg.x" :y2="seg.y2" :stroke="seg.color" stroke-width="2" />
                    <path v-if="seg.mergePath" :d="seg.mergePath" :stroke="seg.color" stroke-width="2" fill="none" />
                    <line v-if="seg.open" :x1="seg.x" :y1="seg.y1" :x2="seg.x" y2="2" :stroke="seg.color" stroke-width="2" stroke-dasharray="3 4" />
                  </g>
                  <template v-for="(c, r) in commits" :key="c.sha">
                    <circle
                      v-if="c.mergeOf"
                      :cx="laneX(0)" :cy="rowY(r)" r="5.5"
                      fill="rgb(var(--c-slate-panel))" :stroke="MAIN_COLOR" stroke-width="2.5"
                      :opacity="isDim(c.mergeOf) ? 0.25 : 1"
                    />
                    <circle
                      v-else
                      :cx="laneX(laneOf(c))" :cy="rowY(r)" :r="c.sha === selected ? 6 : 4.5"
                      :fill="colorOf(c.branch)" stroke="rgb(var(--c-slate-panel))" stroke-width="2"
                      :opacity="isDim(c.branch) ? 0.25 : 1"
                    />
                  </template>
                </svg>

                <div
                  v-for="(c, r) in commits"
                  :key="c.sha"
                  @click="select(c.sha)"
                  @keydown.enter.prevent="select(c.sha)"
                  tabindex="0"
                  class="relative grid items-center gap-2.5 pr-4 font-mono text-body cursor-pointer focus:outline-none focus-visible:ring-1 focus-visible:ring-gear-gold"
                  :class="[c.sha === selected ? 'bg-slate-inner/80' : 'hover:bg-slate-inner/50', rowDim(c) ? 'opacity-30' : '']"
                  :style="{ height: ROW + 'px', gridTemplateColumns: `${lanesWidth}px minmax(0,1fr) auto 66px` }"
                >
                  <span></span>
                  <span class="truncate min-w-0">
                    <template v-if="c.mergeOf">
                      <span class="font-bold mr-1.5" :style="{ color: colorOf(c.mergeOf) }">⇢ {{ c.mergeOf }}</span>
                      <span class="text-zinc-500">merged into {{ data.main }}</span>
                    </template>
                    <template v-else>
                      <span class="font-bold mr-1.5" :style="{ color: colorOf(c.branch) }">{{ c.branch && c.branch !== 'main' ? taskOf(c.branch) : (c.branch || 'other') }}</span>
                      <span class="text-zinc-200">{{ stripTask(c.subject) }}</span>
                    </template>
                  </span>
                  <span v-if="automatonOf(c)" class="text-xs px-1.5 rounded border border-steel text-cyan-energy whitespace-nowrap">
                    {{ automatonOf(c) }}<template v-if="c.attempt > 1"> · a{{ c.attempt }}</template>
                  </span>
                  <span v-else></span>
                  <span class="text-zinc-500 text-right tabular-nums">{{ clock(c.time) }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Commit inspector -->
          <aside class="bg-slate-panel border border-steel rounded-lg" aria-live="polite">
            <div class="px-4 py-2.5 border-b border-steel"><h3 class="text-body font-bold text-zinc-200">Commit</h3></div>
            <div v-if="!selectedCommit" class="p-4 text-body text-zinc-500">Pick a commit to see its automaton, trailers and changed files.</div>
            <div v-else class="p-4 space-y-3 text-body">
              <div>
                <div class="text-xs uppercase tracking-wide text-zinc-500 font-mono">{{ selectedCommit.mergeOf ? 'Merge commit' : (selectedCommit.branch || 'commit') }}</div>
                <h4 class="text-sm font-semibold text-zinc-200 leading-snug mt-0.5">{{ selectedCommit.subject }}</h4>
              </div>
              <dl class="grid gap-x-3 gap-y-1.5 font-mono text-body" style="grid-template-columns: 80px minmax(0,1fr)">
                <dt class="text-zinc-500">sha</dt><dd class="break-all text-zinc-300">{{ selectedCommit.sha.slice(0, 12) }}</dd>
                <dt class="text-zinc-500">time</dt><dd class="text-zinc-300">{{ fullTime(selectedCommit.time) }}</dd>
                <dt class="text-zinc-500">parents</dt>
                <dd class="text-zinc-300">
                  <div v-for="p in selectedCommit.parents" :key="p">
                    {{ p.slice(0, 7) }} <span v-if="branchOfSha(p)" :style="{ color: colorOf(branchOfSha(p)) }">({{ branchOfSha(p) }})</span>
                  </div>
                </dd>
                <template v-if="automatonOf(selectedCommit)">
                  <dt class="text-zinc-500">automaton</dt>
                  <dd class="text-cyan-energy">{{ automatonOf(selectedCommit) }}<span v-if="selectedCommit.attempt"> · attempt {{ selectedCommit.attempt }}</span>
                    <span v-if="!selectedCommit.automaton" class="text-zinc-500" title="This commit has no Ringmaster-Automaton trailer; taken from the task state"> (from state)</span>
                  </dd>
                </template>
                <template v-if="selectedCommit.mergeOf && branchByName(selectedCommit.mergeOf)">
                  <dt class="text-zinc-500">branch life</dt>
                  <dd class="text-zinc-300">{{ lifetime(branchByName(selectedCommit.mergeOf)) }} · {{ branchByName(selectedCommit.mergeOf).commits }} commit(s)</dd>
                </template>
              </dl>
              <div v-if="detail.message && detail.message.includes('\n')">
                <div class="text-xs uppercase tracking-wide text-zinc-500 font-mono mb-1">Message</div>
                <pre class="p-2.5 rounded border border-steel bg-slate-inner text-body leading-relaxed text-zinc-400 whitespace-pre-wrap font-mono">{{ detail.message }}</pre>
              </div>
              <div v-if="detail.files.length">
                <div class="text-xs uppercase tracking-wide text-zinc-500 font-mono mb-1">
                  Files · <span class="text-green-400">+{{ fileTotals.add }}</span> <span class="text-red-400">−{{ fileTotals.del }}</span>
                </div>
                <div class="space-y-0.5 font-mono text-body max-h-64 overflow-y-auto">
                  <div v-for="f in detail.files" :key="f.path" class="flex justify-between gap-2">
                    <span class="truncate text-zinc-300" :title="f.path">{{ f.path }}</span>
                    <span class="shrink-0 tabular-nums">
                      <template v-if="f.add === null"><span class="text-zinc-500">binary</span></template>
                      <template v-else><span class="text-green-400">+{{ f.add }}</span> <span v-if="f.del" class="text-red-400">−{{ f.del }}</span></template>
                    </span>
                  </div>
                </div>
              </div>
              <button
                v-if="taskOfCommit(selectedCommit)"
                @click="openTask(taskOfCommit(selectedCommit))"
                class="w-full px-3 py-1.5 rounded border border-steel text-zinc-300 hover:text-gear-gold hover:border-gear-dark text-body transition"
              >Open task {{ taskOfCommit(selectedCommit) }} ▸</button>
            </div>
          </aside>
        </div>

        <!-- Branch ledger -->
        <div v-if="branches.length" class="bg-slate-panel border border-steel rounded-lg overflow-hidden">
          <div class="px-4 py-2.5 border-b border-steel flex items-center justify-between">
            <h3 class="text-body font-bold text-zinc-200">Branch ledger</h3>
            <span class="text-xs text-zinc-500">click a branch to highlight it</span>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-body font-mono min-w-[860px]">
              <thead>
                <tr class="text-xs uppercase text-zinc-500 border-b border-steel">
                  <th class="text-left font-semibold px-4 py-2">Branch</th>
                  <th class="text-left font-semibold px-2 py-2">Task</th>
                  <th class="text-left font-semibold px-2 py-2">Automaton</th>
                  <th class="text-left font-semibold px-2 py-2">Forked from</th>
                  <th class="text-left font-semibold px-2 py-2">First commit</th>
                  <th class="text-left font-semibold px-2 py-2">Merged</th>
                  <th class="text-right font-semibold px-2 py-2">Lifetime</th>
                  <th class="text-right font-semibold px-2 py-2">Commits</th>
                  <th class="text-left font-semibold px-4 py-2">Status</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="b in branches"
                  :key="b.name"
                  @click="focus = focus === b.name ? '' : b.name"
                  class="border-b border-steel/50 last:border-0 cursor-pointer"
                  :class="focus === b.name ? 'bg-slate-inner' : 'hover:bg-slate-inner/60'"
                >
                  <td class="px-4 py-2 whitespace-nowrap"><span class="inline-block w-2 h-2 rounded-full mr-1.5" :style="{ background: colorOf(b.name) }"></span>{{ b.name }}</td>
                  <td class="px-2 py-2 text-zinc-300 min-w-[180px]">{{ b.title || '—' }}</td>
                  <td class="px-2 py-2 text-cyan-energy whitespace-nowrap">{{ b.automatons.join(' → ') || '—' }}</td>
                  <td class="px-2 py-2 whitespace-nowrap text-zinc-400">
                    <template v-if="b.fork">{{ branchOfSha(b.fork) || data.main }}@{{ b.fork.slice(0, 7) }}</template><template v-else>—</template>
                  </td>
                  <td class="px-2 py-2 whitespace-nowrap text-zinc-400 tabular-nums">{{ b.firstCommit ? clock(b.firstCommit) : '—' }}</td>
                  <td class="px-2 py-2 whitespace-nowrap text-zinc-400 tabular-nums">{{ b.merge ? `${clock(b.merge.time)} · ${b.merge.sha.slice(0, 7)}` : '—' }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300">{{ lifetime(b) }}</td>
                  <td class="px-2 py-2 text-right tabular-nums text-zinc-300">{{ b.commits }}</td>
                  <td class="px-4 py-2">
                    <span class="text-2xs px-1.5 py-0.5 rounded border font-bold whitespace-nowrap" :class="statusClass(b.status)">{{ statusLabel(b.status) }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
</template>

<script setup>
import { statusLabel, statusClass } from '~/utils/status'
const { setView, openTaskDetail } = useRingmaster()

const ROW = 34
const LANE = 18
const PAD = 14
const MAIN_COLOR = 'rgb(var(--c-gear-gold))'
// Same per-task hues as the Logs view
const TASK_HUES = ['cyan-300', 'indigo-300', 'emerald-300', 'amber-300', 'rose-300', 'teal-300']

const data = ref({ main: 'main', commits: [], branches: [], truncated: false })
const loading = ref(true)
const error = ref('')
const focus = ref('')
const selected = ref(null)
const detail = ref({ message: '', files: [] })

const commits = computed(() => data.value.commits) // newest first
const branches = computed(() =>
  [...data.value.branches].sort((a, b) => a.task.localeCompare(b.task, undefined, { numeric: true }))
)

async function load() {
  try {
    const res = await fetch('/api/branches')
    const json = await res.json()
    if (!json.ok) throw new Error(json.error || 'Failed to read branches')
    data.value = json
    error.value = ''
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function select(sha) {
  selected.value = sha
  detail.value = { message: '', files: [] }
  try {
    const res = await fetch(`/api/branches?commit=${sha}`)
    const json = await res.json()
    if (json.ok && selected.value === sha) detail.value = { message: json.message, files: json.files }
  } catch {}
}

// ── Geometry ────────────────────────────────────────────────────────────────
const rowOfSha = computed(() => new Map(commits.value.map((c, r) => [c.sha, r])))
const branchOfShaMap = computed(() => new Map(commits.value.map((c) => [c.sha, c.branch])))
const branchOfSha = (sha) => branchOfShaMap.value.get(sha) || null
const branchByName = (name) => branches.value.find((b) => b.name === name)

// A branch occupies a lane from its fork row up to its merge row (or the top while open)
const laneMap = computed(() => {
  const lanes = { main: 0 }
  const used = [] // lane -> [[top, bottom], ...]
  const n = commits.value.length
  const spans = branches.value.map((b) => {
    const own = commits.value.filter((c) => c.branch === b.name).map((c) => rowOfSha.value.get(c.sha))
    const forkRow = b.fork != null && rowOfSha.value.has(b.fork) ? rowOfSha.value.get(b.fork) : (own.length ? Math.max(...own) + 1 : n)
    const top = b.merge && rowOfSha.value.has(b.merge.sha) ? rowOfSha.value.get(b.merge.sha) : 0
    return { name: b.name, top, bottom: forkRow }
  }).sort((a, b) => b.bottom - a.bottom)
  for (const s of spans) {
    let lane = 1
    while ((used[lane] || []).some(([t, btm]) => !(s.bottom < t || s.top > btm))) lane++
    ;(used[lane] ||= []).push([s.top, s.bottom])
    lanes[s.name] = lane
  }
  return lanes
})
const laneCount = computed(() => Math.max(0, ...Object.values(laneMap.value)) + 1)
const lanesWidth = computed(() => PAD * 2 + (laneCount.value - 1) * LANE)
const laneX = (lane) => PAD + lane * LANE
const rowY = (r) => r * ROW + ROW / 2
const laneOf = (c) => laneMap.value[c.branch] ?? 0

function colorOf(branch) {
  if (!branch || branch === 'main') return MAIN_COLOR
  const n = parseInt(String(branch).replace(/\D/g, ''), 10) || 0
  return `rgb(var(--c-${TASK_HUES[n % TASK_HUES.length]}))`
}

function curve(x1, y1, x2, y2) {
  const my = (y1 + y2) / 2
  return `M${x1},${y1} C${x1},${my} ${x2},${my} ${x2},${y2}`
}

const segments = computed(() => {
  const n = commits.value.length
  return branches.value.map((b) => {
    const rows = commits.value.map((c, r) => (c.branch === b.name ? r : -1)).filter((r) => r >= 0)
    if (!rows.length) return null
    const x = laneX(laneMap.value[b.name])
    const newest = Math.min(...rows)
    const oldest = Math.max(...rows)
    const seg = { name: b.name, color: colorOf(b.name), x, y1: rowY(newest), y2: rowY(oldest), open: !b.merge }
    // Fork from the base commit's lane (or from below the window when it is not loaded)
    if (b.fork && rowOfSha.value.has(b.fork)) {
      const forkRow = rowOfSha.value.get(b.fork)
      const forkLane = laneMap.value[branchOfSha(b.fork)] ?? 0
      seg.forkPath = curve(laneX(forkLane), rowY(forkRow), x, rowY(oldest))
    } else {
      seg.forkPath = `M${x},${n * ROW} L${x},${rowY(oldest)}`
    }
    if (b.merge && rowOfSha.value.has(b.merge.sha)) {
      seg.mergePath = curve(x, rowY(newest), laneX(0), rowY(rowOfSha.value.get(b.merge.sha)))
    }
    return seg
  }).filter(Boolean)
})

const isDim = (branch) => Boolean(focus.value) && branch !== focus.value && branch !== 'main'
const rowDim = (c) => Boolean(focus.value) && (c.mergeOf ? c.mergeOf !== focus.value : c.branch !== focus.value)

// ── Labels & facts ──────────────────────────────────────────────────────────
const taskOf = (branch) => String(branch).replace(/^rm\//, '')
const taskOfCommit = (c) => c.task || (c.mergeOf ? taskOf(c.mergeOf) : c.branch && c.branch !== 'main' ? taskOf(c.branch) : null)
const stripTask = (subject) => subject.replace(/^T\d{2,3}:\s*/, '')
// Trailer first; older commits without trailers fall back to the task's recorded worker
function automatonOf(c) {
  if (c.automaton) return c.automaton
  const b = c.branch && c.branch !== 'main' ? branchByName(c.branch) : null
  return b?.automatons?.[0] || null
}

const pad = (n) => String(n).padStart(2, '0')
function clock(iso) {
  const d = new Date(iso)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
const fullTime = (iso) => new Date(iso).toLocaleString(undefined, { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
function duration(ms) {
  const s = Math.max(0, Math.round(ms / 1000))
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  return m < 60 ? `${m}m ${pad(s % 60)}s` : `${Math.floor(m / 60)}h ${pad(m % 60)}m`
}
function lifetimeMs(b) {
  const forkCommit = commits.value.find((c) => c.sha === b.fork)
  const start = forkCommit?.time || b.firstCommit
  if (!b.merge || !start) return 0
  return new Date(b.merge.time) - new Date(start)
}
const lifetime = (b) => (b.merge ? duration(lifetimeMs(b)) : 'open')

const selectedCommit = computed(() => commits.value.find((c) => c.sha === selected.value) || null)
const fileTotals = computed(() => ({
  add: detail.value.files.reduce((n, f) => n + (f.add || 0), 0),
  del: detail.value.files.reduce((n, f) => n + (f.del || 0), 0)
}))
const mergedCount = computed(() => branches.value.filter((b) => b.merge).length)
const taskCommitCount = computed(() => commits.value.filter((c) => c.branch && c.branch !== 'main').length)
const allAutomatons = computed(() => [...new Set(branches.value.flatMap((b) => b.automatons))])
const avgLifetime = computed(() => {
  const merged = branches.value.filter((b) => b.merge)
  if (!merged.length) return ''
  return duration(merged.reduce((n, b) => n + lifetimeMs(b), 0) / merged.length)
})

function openTask(id) {
  setView('tasks')
  openTaskDetail(id)
}

// Keep the graph current while a run commits and merges
let timer = null
onMounted(() => {
  load()
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>
