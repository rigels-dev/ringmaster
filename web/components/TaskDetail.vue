<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" @click.self="$emit('close')">
    <div class="bg-slate-dark border border-gear-gold/60 rounded-xl w-full max-w-4xl max-h-[90vh] shadow-2xl flex flex-col">
      <!-- Header -->
      <div class="flex items-start justify-between gap-3 p-4 border-b border-steel/60">
        <div class="min-w-0">
          <div class="flex items-center gap-2 text-body">
            <span class="font-bold text-gear-gold">{{ taskId }}</span>
            <span v-if="detail?.state?.status" class="text-2xs px-2 py-0.5 rounded font-bold border" :class="statusClass(detail.state.status)">
              {{ statusLabel(detail.state.status) }}
            </span>
          </div>
          <h3 class="text-sm font-semibold text-zinc-200 mt-1">{{ title }}</h3>
        </div>
        <button @click="$emit('close')" class="text-zinc-400 hover:text-zinc-200 text-sm px-2" title="Close">✕</button>
      </div>

      <div v-if="error" class="p-4 text-body text-red-300">{{ error }}</div>
      <div v-else-if="!detail" class="p-4 text-body text-zinc-400">Loading task details…</div>

      <div v-else class="flex-1 overflow-y-auto p-4 space-y-4 text-body">
        <!-- Facts -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
          <div class="bg-slate-inner border border-steel/50 rounded p-2">
            <div class="text-zinc-500 text-2xs uppercase">Automaton</div>
            <div class="text-cyan-energy truncate">{{ detail.state?.worker || '—' }}</div>
          </div>
          <div class="bg-slate-inner border border-steel/50 rounded p-2">
            <div class="text-zinc-500 text-2xs uppercase">Attempts</div>
            <div class="text-zinc-200">{{ detail.state?.attempt ?? 0 }}</div>
          </div>
          <div class="bg-slate-inner border border-steel/50 rounded p-2">
            <div class="text-zinc-500 text-2xs uppercase">Tokens in / out</div>
            <div class="text-zinc-200">{{ fmtTokens(detail.state?.usage?.tokens_in) }} / {{ fmtTokens(detail.state?.usage?.tokens_out) }}</div>
            <div
              v-if="detail.state?.usage?.tokens_cached"
              class="text-zinc-500 text-2xs"
              title="Read from the prompt cache. These are part of the input count, cost about a tenth, and count as a tenth toward the token budget."
            >
              {{ fmtTokens(detail.state.usage.tokens_cached) }} of the input from cache
            </div>
          </div>
          <div class="bg-slate-inner border border-steel/50 rounded p-2">
            <div class="text-zinc-500 text-2xs uppercase">Duration</div>
            <div class="text-zinc-200">{{ duration }}</div>
          </div>
        </div>

        <!-- Full reason -->
        <div v-if="detail.state?.error" class="p-3 bg-red-950/70 border border-red-800/80 rounded text-red-200">
          <div class="font-bold text-red-300 mb-1">⚠ Reason</div>
          <pre class="whitespace-pre-wrap break-words font-mono text-body text-red-200/90">{{ detail.state.error }}</pre>
        </div>

        <!-- Answer sent: the scheduler applies it on its next tick, then the task runs again -->
        <div v-if="needsYou && answerPending" role="status" aria-live="polite"
          class="p-3 bg-emerald-950/40 border border-emerald-700/70 rounded space-y-1.5">
          <div class="flex items-center gap-2 font-bold text-emerald-200">
            <span class="inline-block w-3.5 h-3.5 rounded-full border-2 border-emerald-300 border-t-transparent animate-spin" aria-hidden="true"></span>
            Answer sent. Waiting for Ringmaster to pick it up…
          </div>
          <p class="text-xs text-emerald-100/80">
            There is no need to send it again. The task starts over with your answer as soon as the run applies it,
            usually within a few seconds, and this panel updates by itself.
          </p>
          <p v-if="!isRunActive" class="text-xs text-amber-300">
            No run is active, so nothing will pick the answer up yet. Start or resume the run from the header.
          </p>
        </div>

        <!-- Needs you: answer requests, approve commands, or give guidance -->
        <div v-else-if="needsYou" class="p-3 bg-rose-950/40 border border-rose-700/70 rounded space-y-3">
          <div class="font-bold text-rose-200">🙋 This task is waiting on you</div>

          <div v-for="r in requests" :key="r.id" class="bg-slate-inner border border-steel/60 rounded p-2.5 space-y-2">
            <!-- Command approval -->
            <template v-if="r.type === 'approval'">
              <div class="text-zinc-300"><span class="text-zinc-500">[{{ r.id }}]</span> Run a command outside the sandbox allow list?</div>
              <code class="block bg-slate-inner text-amber-200 px-2 py-1 rounded break-all">{{ r.command }}</code>
              <div v-if="r.reason" class="text-zinc-400">Why: {{ r.reason }}</div>
              <div class="text-xs text-zinc-500">Runs in the task worktree, without a shell. Approving also allows this command with extra arguments appended.</div>
              <div class="flex flex-wrap gap-2">
                <button v-for="opt in approvalOptions" :key="opt.value" @click="form.approvals[r.id] = opt.value"
                  class="px-2.5 py-1 rounded border text-xs font-bold"
                  :class="form.approvals[r.id] === opt.value ? opt.active : 'border-steel text-zinc-400 hover:text-zinc-200'">
                  {{ opt.label }}
                </button>
              </div>
              <input v-if="form.approvals[r.id] === 'deny'" v-model="form.replies[r.id]" placeholder="Optional: what should the automaton do instead?"
                class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200" />
            </template>

            <!-- Choice -->
            <template v-else-if="r.type === 'choice'">
              <div class="text-2xs uppercase tracking-wide text-zinc-500">Pick one · or write your own below</div>
              <div class="text-zinc-300"><span v-if="r.id !== 'next'" class="text-zinc-500">[{{ r.id }}]</span> {{ r.question }}</div>
              <div class="flex flex-wrap gap-2">
                <button v-for="opt in r.options" :key="opt" @click="form.replies[r.id] = opt"
                  class="px-2.5 py-1 rounded border text-xs font-bold"
                  :class="form.replies[r.id] === opt ? 'bg-cyan-950 border-cyan-500 text-cyan-200' : 'border-steel text-zinc-400 hover:text-zinc-200'">
                  {{ opt }}<span v-if="opt === r.recommended" class="text-emerald-400"> ★ recommended</span>
                </button>
              </div>
              <!-- New acceptance commands: Ringmaster runs these, and only the user can change them -->
              <div v-if="form.replies[r.id] === CHANGE_ACCEPTANCE" class="space-y-1">
                <label class="text-xs text-zinc-400" :for="`acceptance-${r.id}`">
                  Acceptance commands, one per line. Ringmaster runs only these to decide whether the task is done.
                </label>
                <textarea :id="`acceptance-${r.id}`" v-model="form.acceptance" rows="4" spellcheck="false"
                  class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200 font-mono text-xs"></textarea>
                <div class="text-xs text-zinc-500">
                  Run without a shell: no pipes, &amp;&amp;, ; or redirects. A line you delete is no longer checked.
                </div>
              </div>
              <input v-else :value="r.options.includes(form.replies[r.id]) ? '' : form.replies[r.id]"
                @input="form.replies[r.id] = $event.target.value" placeholder="…or write what it should do instead"
                class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200" />
            </template>

            <!-- Free input; secrets are referenced by env var name only -->
            <template v-else>
              <div class="text-2xs uppercase tracking-wide text-zinc-500">{{ r.secret ? 'A variable name is needed' : 'Your answer is needed' }}</div>
              <div class="text-zinc-300"><span v-if="r.id !== 'next'" class="text-zinc-500">[{{ r.id }}]</span> {{ r.question }}</div>
              <div v-if="r.secret" class="text-xs text-amber-300">Secret: enter the NAME of the environment variable that holds it, never the value.</div>
              <textarea v-if="!r.secret && r.id === 'next'" v-model="form.replies[r.id]" rows="3" placeholder="Your answer"
                class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200"></textarea>
              <input v-else v-model="form.replies[r.id]" :placeholder="r.secret ? 'e.g. MY_SERVICE_API_KEY' : 'Your answer'"
                class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200" />
            </template>
          </div>

          <div>
            <label class="text-zinc-400 text-xs">{{ requests.length ? 'Extra notes (optional)' : noteLabel }}</label>
            <textarea v-model="form.text" rows="3" class="w-full mt-1 bg-slate-dark border border-steel rounded px-2 py-1 text-zinc-200"
              :placeholder="requests.length ? 'Anything else the automaton should know' : 'Tell the automaton what to do differently, or what you have already fixed yourself.'"></textarea>
          </div>

          <div class="flex items-center justify-between gap-2">
            <span class="text-xs text-rose-200/80">{{ formError }}</span>
            <button @click="submitAnswer" :disabled="sending" :aria-busy="sending"
              class="inline-flex items-center gap-2 px-4 py-1.5 rounded text-body font-bold bg-emerald-500 hover:bg-emerald-400 text-ink disabled:opacity-60 disabled:cursor-wait">
              <span v-if="sending" class="inline-block w-3.5 h-3.5 rounded-full border-2 border-ink border-t-transparent animate-spin" aria-hidden="true"></span>
              {{ sending ? 'Sending…' : detail.state?.status === 'PAUSED' && !requests.length ? '▶ Resume task' : '✓ Send answer & resume' }}
            </button>
          </div>
        </div>

        <!-- Paths for manual inspection -->
        <div class="bg-slate-inner border border-steel/50 rounded p-2 space-y-1 text-xs">
          <div v-if="detail.worktree"><span class="text-zinc-500">Worktree:</span> <code class="text-zinc-300 break-all select-all">{{ detail.worktree }}</code></div>
          <div><span class="text-zinc-500">Branch:</span> <code class="text-zinc-300 select-all">{{ detail.branch }}</code></div>
          <div v-if="detail.specFile"><span class="text-zinc-500">Spec:</span> <code class="text-zinc-300 break-all select-all">{{ detail.specFile }}</code></div>
          <div v-if="detail.reportFile"><span class="text-zinc-500">Report:</span> <code class="text-zinc-300 break-all select-all">{{ detail.reportFile }}</code></div>
          <div v-for="a in detail.attempts" :key="a.file"><span class="text-zinc-500">Transcript #{{ a.attempt }}:</span> <code class="text-zinc-300 break-all select-all">{{ a.file }}</code></div>
        </div>

        <!-- Tabs -->
        <div class="flex gap-1 border-b border-steel/60">
          <button
            v-for="t in tabs"
            :key="t.id"
            @click="tab = t.id"
            class="px-3 py-1.5 text-xs font-bold rounded-t border border-b-0"
            :class="tab === t.id ? 'bg-slate-panel text-gear-gold border-gear-dark' : 'text-zinc-400 border-transparent hover:text-zinc-200'"
          >
            {{ t.label }}
          </button>
        </div>

        <!-- Transcript -->
        <div v-if="tab === 'transcript'" class="space-y-3">
          <p v-if="!detail.attempts.length" class="text-zinc-500">No transcript yet. The task has not started, or its worktree was removed after merging.</p>
          <div v-for="a in detail.attempts" :key="a.file" class="space-y-2">
            <div class="text-gear-gold font-bold">Attempt {{ a.attempt }}</div>
            <div v-for="(e, i) in a.entries" :key="i" class="border border-steel/50 rounded p-2 bg-slate-inner">
              <div class="text-2xs text-zinc-500 mb-1">turn {{ e.turn }} · {{ e.role }} · {{ e.timestamp }}</div>
              <pre v-if="e.content" class="whitespace-pre-wrap break-words text-zinc-200">{{ e.content }}</pre>
              <div v-for="tc in e.tool_calls || []" :key="tc.id" class="text-cyan-energy break-all font-mono">→ {{ tc.function?.name }} {{ tc.function?.arguments }}</div>
              <details v-if="e.tool_response">
                <summary class="cursor-pointer text-zinc-400">tool output ({{ e.tool_response.length }} chars)</summary>
                <pre class="whitespace-pre-wrap break-words text-zinc-300 mt-1">{{ e.tool_response }}</pre>
              </details>
              <pre v-if="e.raw" class="whitespace-pre-wrap break-words text-zinc-400">{{ e.raw }}</pre>
            </div>
          </div>
        </div>

        <!-- Report -->
        <div v-else-if="tab === 'report'">
          <div v-if="detail.report" class="text-zinc-200 bg-slate-inner border border-steel/50 rounded p-3"><MarkdownText :source="detail.report" /></div>
          <p v-else class="text-zinc-500">The automaton has not written .ringmaster/outputs/{{ taskId }}.md.</p>
        </div>

        <!-- Events -->
        <div v-else-if="tab === 'events'" class="space-y-1.5">
          <p v-if="!detail.events.length" class="text-zinc-500">No audit events recorded for this task.</p>
          <div v-for="(e, i) in detail.events" :key="i" class="border border-steel/50 rounded p-2 bg-slate-inner">
            <div class="flex gap-2 text-xs">
              <span class="text-zinc-500">{{ e.timestamp }}</span>
              <span class="text-gear-gold font-bold">{{ e.action }}</span>
              <span class="text-zinc-400">attempt {{ e.attempt }}</span>
            </div>
            <pre v-if="e.payload" class="whitespace-pre-wrap break-words text-zinc-300 mt-1">{{ JSON.stringify(e.payload, null, 2) }}</pre>
          </div>
        </div>

        <!-- Spec -->
        <div v-else-if="tab === 'spec'">
          <div v-if="detail.spec" class="text-zinc-200 bg-slate-inner border border-steel/50 rounded p-3"><MarkdownText :source="detail.spec" /></div>
          <p v-else class="text-zinc-500">No spec file found in tasks/.</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { statusLabel, statusClass } from '~/utils/status'
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'

const { answerTask, isRunActive } = useRingmaster()

const props = defineProps({
  taskId: { type: String, required: true },
  title: { type: String, default: '' }
})
const emit = defineEmits(['close'])

const detail = ref(null)
const error = ref('')
const tab = ref('transcript')
const tabs = [
  { id: 'transcript', label: 'Transcript' },
  { id: 'report', label: 'Report' },
  { id: 'events', label: 'Events' },
  { id: 'spec', label: 'Spec' }
]

async function load() {
  try {
    const res = await fetch(`/api/task?id=${encodeURIComponent(props.taskId)}`)
    const data = await res.json()
    if (data.ok) {
      detail.value = data
      error.value = ''
    } else {
      error.value = data.error || 'Failed to load task details'
    }
  } catch (err) {
    error.value = err.message
  }
}

const NEEDS_YOU = ['AWAITING_INPUT', 'BLOCKED', 'NEEDS_REVIEW', 'PAUSED']
const needsYou = computed(() => NEEDS_YOU.includes(detail.value?.state?.status))
const requests = computed(() => detail.value?.state?.requests || [])
const noteLabel = computed(() => ({
  BLOCKED: 'What does the automaton need to get unblocked?',
  NEEDS_REVIEW: 'Guidance for the next attempt',
  PAUSED: 'Optional note before resuming'
}[detail.value?.state?.status] || 'Your answer'))

const approvalOptions = [
  { value: 'task', label: '✓ Allow for this task', active: 'bg-emerald-950 border-emerald-600 text-emerald-300' },
  { value: 'project', label: '✓ Allow for whole project', active: 'bg-emerald-950 border-emerald-600 text-emerald-300' },
  { value: 'deny', label: '✗ Deny', active: 'bg-red-950 border-red-700 text-red-300' }
]

// Must match verify.OptionChangeAcceptance in Go
const CHANGE_ACCEPTANCE = 'Change the acceptance commands, then check again'

const form = reactive({ approvals: {}, replies: {}, text: '', acceptance: '' })

// Ringmaster's own question after a stop comes with its recommended answer
// picked, so the usual case is one click; a single option is always picked.
// The automaton's own choices are left for the user to make.
watch(requests, (list) => {
  for (const r of list) {
    if (r.type !== 'choice' || form.replies[r.id] !== undefined) continue
    if (r.options?.length === 1) form.replies[r.id] = r.options[0]
    else if (r.id === 'next' && r.recommended) form.replies[r.id] = r.recommended
  }
}, { immediate: true })

// The editor starts with the task's current commands
watch(() => detail.value?.acceptance, (list) => {
  if (!form.acceptance && list?.length) form.acceptance = list.join('\n')
}, { immediate: true })
const sending = ref(false)
const formError = ref('')
// Set the moment an answer is accepted, until the reload that follows: from
// then on the server's answerQueued says whether it still waits to be applied,
// across reloads and tabs, and a new stop shows its new questions
const justSent = ref(false)
const answerPending = computed(() => justSent.value || Boolean(detail.value?.answerQueued))

async function submitAnswer() {
  if (sending.value) return
  formError.value = ''
  const replies = {}
  const allow = []
  const scopes = []
  for (const r of requests.value) {
    if (r.type === 'approval') {
      const decision = form.approvals[r.id]
      if (!decision) {
        formError.value = `Approve or deny ${r.id} first.`
        return
      }
      if (decision === 'deny') {
        const note = (form.replies[r.id] || '').trim()
        replies[r.id] = note ? `denied: ${note}` : 'denied; find another way that stays within the allowed commands'
      } else {
        replies[r.id] = `approve (${decision} scope)`
        allow.push(r.command)
        scopes.push(decision)
      }
    } else {
      const v = (form.replies[r.id] || '').trim()
      if (!v) {
        formError.value = `Answer ${r.id} first.`
        return
      }
      replies[r.id] = v
    }
  }
  let acceptance
  if (requests.value.some(r => replies[r.id] === CHANGE_ACCEPTANCE)) {
    acceptance = form.acceptance.split('\n').map(c => c.trim()).filter(Boolean)
    if (!acceptance.length) {
      formError.value = 'Keep at least one acceptance command.'
      return
    }
  }
  // One answer carries one scope: mixed choices fall back to the narrower task scope
  const scope = scopes.length && scopes.every(s => s === 'project') ? 'project' : 'task'
  const status = detail.value?.state?.status
  let text = form.text.trim()
  if (!requests.value.length && !text) {
    if (status !== 'PAUSED') {
      formError.value = 'Write some guidance for the automaton first.'
      return
    }
    text = 'Resumed by the user.'
  }

  sending.value = true
  const ok = await answerTask(props.taskId, { replies, allow, scope, text, ...(acceptance ? { acceptance } : {}) })
  sending.value = false
  if (ok) {
    justSent.value = true
    form.approvals = {}
    form.replies = {}
    form.acceptance = ''
    form.text = ''
    await load()
    justSent.value = false
  }
}

const fmtTokens = (n) => Number(n || 0).toLocaleString('en-US')

const duration = computed(() => {
  const s = detail.value?.state
  if (!s?.start_at) return '—'
  const end = s.finish_at ? new Date(s.finish_at) : new Date()
  const secs = Math.max(0, Math.round((end - new Date(s.start_at)) / 1000))
  return secs >= 60 ? `${Math.floor(secs / 60)}m ${secs % 60}s` : `${secs}s`
})

// Keep refreshing while open so a running task's transcript stays live
let timer = null
function onKey(e) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => {
  load()
  timer = setInterval(load, 3000)
  window.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  clearInterval(timer)
  window.removeEventListener('keydown', onKey)
})
</script>
