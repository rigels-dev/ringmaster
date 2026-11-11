<template>
      <section class="flex-1 flex flex-col p-6 overflow-y-auto">
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
            <!-- Clean Direct Action Button: Execute Plan / Spur Workers -->
            <button 
              v-if="tasks.length > 0 && planApprovalState.isApproved"
              @click="handleExecuteOrSpur"
              class="relative px-4 py-2 rounded-lg shadow-sm flex items-center gap-2.5 transition-all duration-150 select-none cursor-pointer border font-cinzel tracking-wider text-xs font-bold"
              :class="[
                isRunActive
                  ? 'bg-cyan-950/90 hover:bg-cyan-900 border-cyan-400 text-cyan-300 ring-2 ring-cyan-400/40 shadow-[0_0_15px_rgba(41,242,236,0.3)] animate-pulse'
                  : 'bg-gear-gold hover:bg-amber-400 border-amber-500 text-ink shadow-[0_0_12px_rgba(222,170,59,0.25)] active:scale-95'
              ]"
              :title="isRunActive ? 'Automatons are running in worktrees. Click to ping status.' : 'Dispatch approved plan to automatons.'"
            >
              <span class="text-sm">{{ isRunActive ? '⚡' : '🚀' }}</span>
              <span>
                {{ isRunActive ? 'Automatons Running...' : 'Execute Plan' }}
              </span>
            </button>
            <button 
              v-if="isRunActive"
              @click="stopOrchestration"
              class="px-3 py-2 bg-red-950/80 hover:bg-red-900 border border-red-700/60 text-red-200 text-xs font-bold rounded transition"
              title="Gracefully stop orchestration"
            >
              ⏹ Stop
            </button>
          </div>
        </div>

        <!-- Live Orchestration Console Drawer (When Running or Logs exist) -->
        <div v-if="isRunActive || (runLogs && runLogs.length > 0)" class="mb-6 p-4 rounded-xl border border-steel/80 bg-slate-panel shadow-xl">
          <div class="flex items-center justify-between mb-3 border-b border-steel/60 pb-2">
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full" :class="isRunActive ? 'bg-cyan-400 animate-ping' : 'bg-zinc-500'"></span>
              <span class="text-xs font-bold font-cinzel tracking-wider text-gear-gold flex items-center gap-2">
                <span>{{ isRunActive ? 'Active Ringmaster Orchestration Stream' : 'Orchestration Log History' }}</span>
                <span v-if="isRunActive" class="text-[9px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-700/60 font-mono animate-pulse">STREAMING</span>
              </span>
            </div>
            <div class="flex items-center gap-3">
              <span class="text-[10px] text-zinc-400 font-mono">PID: {{ isRunActive ? 'Live' : 'Exited' }}</span>
              <button 
                v-if="!isRunActive && runLogs.length > 0"
                @click="clearRunLogs"
                class="text-[10px] text-zinc-400 hover:text-zinc-200 transition"
              >
                Clear Logs
              </button>
            </div>
          </div>
          <!-- Auto-scrolling terminal console -->
          <div ref="runConsoleRef" class="bg-slate-inner/90 p-3 rounded font-mono text-[11px] leading-relaxed max-h-56 overflow-y-auto space-y-1 border border-steel/40 text-zinc-300 select-text">
            <div v-for="(log, idx) in runLogs" :key="idx" class="break-all whitespace-pre-wrap font-mono flex items-start gap-1.5">
              <span class="text-zinc-600 select-none">›</span>
              <span :class="logLineClass(log)">
                {{ log }}
              </span>
            </div>
          </div>
        </div>

        <!-- Approval Gate Banner (Interactive Plan Review) -->
        <div v-if="tasks.length > 0" class="mb-6 p-4 rounded-xl border transition shadow-lg" :class="planApprovalState.isApproved ? 'bg-green-950/30 border-green-700/60' : 'bg-amber-950/30 border-amber-600/60'">
          <div class="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div>
              <div class="flex items-center gap-2 mb-1">
                <span class="text-xs font-black uppercase tracking-wider px-2 py-0.5 rounded" :class="planApprovalState.isApproved ? 'bg-green-500 text-ink' : 'bg-amber-500 text-ink'">
                  {{ planApprovalState.isApproved ? '★ PLAN APPROVED' : '⚠ APPROVAL REQUIRED' }}
                </span>
                <span class="text-xs font-mono text-zinc-300 font-bold">{{ tasks.length }} Tasks Synthesized</span>
                <span v-if="planApprovalState.approvedHash" class="text-[10px] font-mono text-zinc-500">SHA-256: {{ planApprovalState.approvedHash.slice(0, 16) }}...</span>
              </div>
              <p class="text-xs text-zinc-300">
                <span v-if="planApprovalState.isApproved">The execution DAG is cryptographically locked and verified. Automatons are ready to build in isolated worktrees.</span>
                <span v-else>Review the task assignments below. You can change automaton models per task before signing and locking the plan.</span>
              </p>
            </div>

            <!-- Action Buttons -->
            <div class="flex items-center gap-2.5">
              <button 
                v-if="!planApprovalState.isApproved"
                @click="approvePlanAction"
                :disabled="isApproving || unansweredQuestions.length > 0"
                :title="unansweredQuestions.length ? 'Answer the open questions first' : ''"
                class="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-ink font-bold text-xs rounded shadow flex items-center gap-1.5 transition cursor-pointer"
              >
                <span>✓</span>
                <span>{{ isApproving ? 'Hashing & Locking...' : 'Approve & Lock Plan' }}</span>
              </button>
              <button 
                @click="replanInChat"
                :disabled="isGeneratingPlan"
                class="px-3.5 py-2 bg-slate-panel hover:bg-slate-dark text-gear-gold border border-gear-dark rounded text-xs font-bold transition flex items-center gap-1.5"
                title="Regenerate plan with Master AI"
              >
                <span>↺</span>
                <span>Re-Plan</span>
              </button>
              <button 
                @click="discardPlanAction"
                class="px-3 py-2 bg-slate-dark hover:bg-rose-950/60 text-zinc-400 hover:text-rose-400 border border-steel rounded text-xs transition"
                title="Discard current plan and start over"
              >
                <span>✗ Discard</span>
              </button>
            </div>
          </div>

          <!-- Master's open questions: all must be answered before approval -->
          <div v-if="openQuestions.length || assumptions.length" class="mt-3 pt-2 border-t border-amber-700/40 text-xs divide-y divide-steel/40">
            <!-- Questions accordion: open while anything is unanswered, collapses to a one-line summary once done -->
            <div v-if="openQuestions.length" class="py-1.5">
              <button @click="questionsOpen = !questionsOpen" class="w-full flex items-center gap-2 py-1 text-left">
                <span class="text-zinc-500 text-[10px] w-3">{{ questionsOpen ? '▾' : '▸' }}</span>
                <span class="font-bold text-amber-200">❓ Questions ({{ openQuestions.length }})</span>
                <span v-if="unansweredQuestions.length" class="text-rose-300 font-bold">· {{ unansweredQuestions.length }} unanswered</span>
                <span v-else class="text-emerald-400 font-bold">· all answered</span>
                <span v-if="!questionsOpen" class="flex-1 min-w-0 truncate text-[10px] text-zinc-500">
                  {{ openQuestions.map(q => `${q.id}: ${q.answer || '—'}`).join(' · ') }}
                </span>
              </button>
              <div v-if="questionsOpen" class="mt-2 space-y-2">
                <div v-for="q in openQuestions" :key="q.id" class="bg-slate-inner/70 border border-steel/60 rounded p-2.5 space-y-2">
                  <div class="text-zinc-200"><span class="text-zinc-500 font-mono">{{ q.id }}</span> {{ q.question }}</div>
                  <div class="flex flex-wrap gap-2">
                    <button v-for="opt in q.options || []" :key="opt" @click="answerPlanQuestion(q.id, opt)"
                      class="px-2.5 py-1 rounded border text-[10px] font-bold"
                      :class="q.answer === opt ? 'bg-cyan-950 border-cyan-500 text-cyan-200' : 'border-steel text-zinc-400 hover:text-zinc-200'">
                      {{ opt }}<span v-if="opt === q.recommended" class="text-emerald-400"> ★ recommended</span>
                    </button>
                  </div>
                  <input :value="q.answer && !(q.options || []).includes(q.answer) ? q.answer : ''"
                    @change="answerPlanQuestion(q.id, $event.target.value)"
                    placeholder="…or type another answer and press Enter"
                    class="w-full bg-slate-dark border border-steel rounded px-2 py-1 text-[11px] text-zinc-200" />
                </div>
              </div>
            </div>

            <!-- Assumptions accordion: collapsed by default -->
            <div v-if="assumptions.length" class="py-1.5">
              <button @click="assumptionsOpen = !assumptionsOpen" class="w-full flex items-center gap-2 py-1 text-left">
                <span class="text-zinc-500 text-[10px] w-3">{{ assumptionsOpen ? '▾' : '▸' }}</span>
                <span class="font-bold text-zinc-300">Assumptions ({{ assumptions.length }})</span>
                <span class="text-[10px] text-zinc-500">· defaults the master picked</span>
              </button>
              <div v-if="assumptionsOpen" class="mt-1.5 pl-5 text-zinc-400 space-y-0.5">
                <div v-for="(a, i) in assumptions" :key="i">• {{ a }}</div>
                <div class="pt-1 text-[10px] text-zinc-500">Disagree? Tell the master in Chat and Re-Plan.</div>
              </div>
            </div>
          </div>
        </div>

        <!-- If Blank Project -->
        <div v-if="tasks.length === 0" class="flex-1 flex flex-col items-center justify-center p-12 bg-slate-panel/30 border border-dashed border-steel rounded-lg text-center">
          <img src="/icons/ringmaster-animated.svg" alt="Ringmaster Kinematics" class="w-36 h-36 object-contain drop-shadow-[0_0_20px_rgba(61,245,240,0.35)] mb-4" />
          <h3 class="text-sm font-bold text-gear-gold mb-1">No Tasks Registered Yet</h3>
          <p class="text-xs text-zinc-400 max-w-md mb-4">
            This project has just been initialized. Head over to Chat to collaborate with Master Planner or run <code class="text-gear-gold">ringmaster plan</code> in your terminal to create the DAG task workflow.
          </p>
          <button @click="activeView = 'chat'" class="px-4 py-2 bg-gear-gold text-ink font-bold text-xs rounded">
            Start Collaboration in Chat ▸
          </button>
        </div>

        <!-- Task Cards Grid with Interactive Worker Dropdown -->
        <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
          <div 
            v-for="task in tasks" 
            :key="task.id"
            @click="openTaskDetail(task.id)"
            title="Click for task details, transcript and report"
            class="bg-slate-panel border p-4 rounded-lg flex flex-col justify-between transition-all duration-300 relative overflow-hidden cursor-pointer"
            :class="{
              'border-cyan-energy/80 shadow-[0_0_20px_rgba(41,242,236,0.35)] ring-1 ring-cyan-energy/50': task.status === 'RUNNING',
              'border-steel hover:border-gear-gold': task.status !== 'RUNNING'
            }"
          >
            <!-- Animated Scanline when task is active/running -->
            <div 
              v-if="task.status === 'RUNNING'"
              class="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-cyan-energy to-transparent animate-[pulse_1.5s_infinite]"
            ></div>

            <div>
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-gear-gold font-mono flex items-center gap-1.5">
                  <span>{{ task.id }}</span>
                  <span 
                    v-if="task.status === 'RUNNING'"
                    class="w-2 h-2 rounded-full bg-cyan-energy animate-ping"
                    title="Worker Sprinting in worktree"
                  ></span>
                </span>
                <div class="flex items-center gap-2">
                  <span class="text-[9px] uppercase px-1.5 py-0.2 rounded bg-slate-dark text-zinc-400 border border-steel">
                    {{ task.type || 'backend' }}
                  </span>
                  <span 
                    class="text-[9px] px-2 py-0.5 rounded font-bold uppercase flex items-center gap-1"
                    :class="{
                      'bg-green-950 text-green-400 border border-green-800': task.status === 'DONE' || task.status === 'COMPLETED',
                      'bg-cyan-950 text-cyan-300 border border-cyan-500 animate-pulse shadow-sm shadow-cyan-500/50': task.status === 'RUNNING',
                      'bg-red-950 text-red-400 border border-red-800': task.status === 'BLOCKED',
                      'bg-zinc-900 text-zinc-500 border border-zinc-700': task.status === 'FAILED',
                      'bg-amber-950 text-amber-300 border border-amber-700': task.status === 'NEEDS_REVIEW',
                      'bg-indigo-950 text-indigo-300 border border-indigo-700': task.status === 'PAUSED',
                      'bg-rose-950 text-rose-300 border border-rose-700 animate-pulse': task.status === 'AWAITING_INPUT',
                      'bg-slate-inner text-zinc-400 border border-steel': task.status === 'PENDING'
                    }"
                  >
                    <span v-if="task.status === 'RUNNING'">🤖</span>
                    <span v-else-if="task.status === 'DONE' || task.status === 'COMPLETED'">✓</span>
                    <span>{{ task.status }}</span>
                  </span>
                </div>
              </div>
              <h3 class="text-xs font-semibold text-zinc-200 mb-1 leading-snug">{{ task.title }}</h3>
              <p class="text-[10px] text-zinc-400 mb-3 line-clamp-3">{{ task.desc }}</p>

              <!-- Blocker or Error Reason Alert Banner -->
              <div v-if="task.error" class="mb-3 p-2 bg-red-950/70 border border-red-800/80 rounded text-[10px] text-red-200 font-mono leading-tight">
                <span class="font-bold text-red-300">⚠ Reason:</span>
                <p class="mt-0.5 text-red-300/90 line-clamp-2">{{ task.error }}</p>
                <p class="mt-1 text-[9px] text-red-300/60">Click for full details ▸</p>
              </div>
              <div v-if="['AWAITING_INPUT', 'BLOCKED', 'NEEDS_REVIEW', 'PAUSED'].includes(task.status)" class="mb-3 px-2 py-1.5 bg-rose-950/50 border border-rose-700/60 rounded text-[10px] text-rose-200 font-bold">
                🙋 Needs you<span v-if="task.requests.length"> · {{ task.requests.length }} question(s)</span> · click to answer ▸
              </div>
            </div>

            <div class="border-t border-steel/70 pt-2.5 mt-2 space-y-2">
              <!-- Live Worktree & Working Feedback Indicator -->
              <div 
                v-if="task.status === 'RUNNING'"
                class="flex flex-col gap-2 bg-cyan-950/40 px-3 py-2.5 rounded border border-cyan-500/60 text-cyan-300"
              >
                <div class="flex items-center justify-between gap-2 text-[10px]">
                  <span class="flex items-center gap-1.5 font-bold whitespace-nowrap">
                    <span class="w-2 h-2 bg-cyan-400 rounded-full animate-ping shrink-0"></span>
                    Automaton Working
                  </span>
                  <span class="text-[9px] font-mono text-zinc-400 animate-pulse whitespace-nowrap">Running code…</span>
                </div>
                <div class="flex items-center justify-between gap-2 text-[9px] min-w-0">
                  <code class="text-cyan-energy truncate" :title="`branch rm/${task.id}`">⎇ rm/{{ task.id }}</code>
                  <span class="text-zinc-400 truncate" :title="task.worker">{{ task.worker }}</span>
                </div>
                <!-- Shimmering Activity Progress Bar -->
                <div class="w-full h-1 bg-slate-900 rounded overflow-hidden relative">
                  <div class="h-full bg-gradient-to-r from-cyan-500 via-teal-300 to-amber-400 animate-[pulse_1s_infinite] w-full"></div>
                </div>
              </div>

              <!-- Dependencies list -->
              <div class="flex items-center justify-between text-[9px] text-zinc-500">
                <span>Depends on: <code class="text-zinc-400">{{ task.depends_on && task.depends_on.length ? task.depends_on.join(', ') : 'None (Root)' }}</code></span>
                <span>Retries: {{ task.retries }}</span>
              </div>

              <!-- The approved plan lets this task rewrite tests that already exist -->
              <div
                v-if="task.editsTests"
                class="text-[9px] text-amber-300 bg-amber-950/40 border border-amber-800/60 rounded px-2 py-1"
                title="Normally a worker's changes to existing test files are undone before acceptance runs. The plan exempts this task (edits_tests)."
              >
                ⚠ May change existing tests
              </div>

              <!-- Interactive Automaton Selection Dropdown -->
              <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 text-[10px] bg-slate-inner p-2 rounded border border-steel/50">
                <span class="text-zinc-400 font-semibold whitespace-nowrap">Assigned Automaton:</span>
                <div class="relative w-full sm:w-auto flex-1 sm:max-w-[210px]" @click.stop>
                  <select 
                    :value="task.worker"
                    @change="updateTaskWorker(task.id, $event.target.value)"
                    class="w-full bg-slate-dark border border-steel rounded text-[10px] text-cyan-energy font-bold px-2 py-1 focus:outline-none focus:border-gear-gold truncate"
                  >
                    <option
                      v-if="task.worker && !availableWorkers.some(a => a.id === task.worker)"
                      :value="task.worker"
                      disabled
                    >
                      ⚠ {{ task.worker }} (unavailable)
                    </option>
                    <option
                      v-for="a in availableWorkers"
                      :key="a.id" 
                      :value="a.id"
                    >
                      🤖 {{ a.id }} ({{ a.model }})
                    </option>
                  </select>
                </div>
              </div>
            </div>
          </div>
        </div>
        <TaskDetail v-if="openTask" :key="openTask.id" :task-id="openTask.id" :title="openTask.title" @close="openTaskId = null" />
      </section>
</template>

<script setup>
const {
  activeView,
  project,
  tasks,
  planApprovalState,
  isApproving,
  availableWorkers,
  approvePlanAction,
  updateTaskWorker,
  discardPlanAction,
  isGeneratingPlan,
  triggerGeneratePlan,
  isRunActive,
  runLogs,
  runConsoleRef,
  logLineClass,
  handleExecuteOrSpur,
  clearRunLogs,
  stopOrchestration,
  setView,
  unansweredQuestions,
  answerPlanQuestion,
  openTaskId,
  openTaskDetail
} = useRingmaster()

const openQuestions = computed(() => project.value.plan?.open_questions || [])
const assumptions = computed(() => project.value.plan?.assumptions || [])

// Keep the questions open only while they block approval, so the task grid stays in focus
const questionsOpen = ref(unansweredQuestions.value.length > 0)
const assumptionsOpen = ref(false)
watch(() => unansweredQuestions.value.length, (n, prev) => {
  if (n > 0 && !prev) questionsOpen.value = true
  else if (n === 0 && prev > 0) questionsOpen.value = false
})

const openTask = computed(() => tasks.value.find(t => t.id === openTaskId.value) || null)

// Re-planning happens in the chat so its progress messages are visible there
function replanInChat() {
  setView('chat')
  triggerGeneratePlan()
}
</script>
