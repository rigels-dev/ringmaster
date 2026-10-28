<template>
      <section class="flex-1 flex h-full overflow-hidden">
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
                {{ project.isBlank ? 'Discuss project scope with Master Planner to draft your BRIEF.md.' : 'Live mission control and multi-agent fleet communications.' }}
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
          <div ref="chatContainerRef" class="flex-1 p-6 overflow-y-auto space-y-4">
            <!-- Welcome Hero Animated SVG when chat is clean/blank -->
            <div v-if="messages.length <= 1" class="flex flex-col items-center justify-center py-6 text-center select-none">
              <img src="/icons/ringmaster-animated.svg" alt="Ringmaster Kinematics" class="w-44 h-44 object-contain drop-shadow-[0_0_25px_rgba(222,170,59,0.35)] mb-3" />
              <h3 class="font-cinzel text-base font-bold text-gear-gold tracking-wide">The Stage is Ready</h3>
              <p class="text-xs text-zinc-400 max-w-md mt-1">Discuss project requirements with Master Planner to architect tasks and orchestrate automaton agents.</p>
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
                <span v-else-if="msg.role === 'worker'" class="text-cyan-energy font-bold">🤖 Automaton: {{ msg.workerId }} · Task {{ msg.taskId }}</span>
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
                <!-- Automaton Report Card Header -->
                <div v-if="msg.role === 'worker'" class="flex items-center justify-between pb-2 mb-2 border-b border-steel/50">
                  <span class="text-[10px] font-bold tracking-wider" :class="msg.type === 'error' ? 'text-coat-bright' : 'text-cyan-energy'">
                    {{ msg.statusTitle }}
                  </span>
                  <span class="text-[9px] px-1.5 py-0.5 rounded bg-slate-panel text-zinc-400">
                    {{ msg.taskBranch }}
                  </span>
                </div>

                <p class="whitespace-pre-wrap">{{ msg.text }}</p>

                <!-- Shortcut button to navigate directly to Tasks tab if plan was generated -->
                <div v-if="msg.role === 'master' && msg.text && msg.text.includes('DAG Plan successfully generated')" class="mt-3 pt-2.5 border-t border-gear-dark/40 flex items-center gap-2">
                  <button 
                    @click="setView('tasks')"
                    class="px-3.5 py-1.5 bg-gradient-to-r from-gear-gold to-amber-500 hover:from-amber-400 hover:to-gear-gold text-slate-deep font-bold text-xs rounded transition flex items-center gap-1.5 shadow"
                  >
                    <span>📋 View & Review Tasks Pipeline</span>
                    <span>▸</span>
                  </button>
                </div>

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

            <!-- Thinking / Typing Indicator with Animated Kinematics Emblem -->
            <div v-if="isThinking" class="flex flex-col mr-auto items-start max-w-xl animate-fade-in">
              <div class="flex items-center gap-1.5 mb-1 text-[9px] uppercase tracking-wider">
                <span class="text-gear-gold font-bold">🎪 Master Planner</span>
                <span class="text-zinc-500">· orchestrating</span>
              </div>
              <div class="p-3.5 rounded-lg text-xs leading-relaxed bg-slate-dark border border-gear-gold/40 shadow-md flex items-center gap-3">
                <!-- Kinematic Emblem with moving hands, rotating gears & strings -->
                <RingmasterEmblem :status="isGeneratingPlan ? 'dag' : 'execution'" size="sm" />
                <div class="flex flex-col">
                  <span class="text-gear-gold font-semibold text-[11px] tracking-wide">
                    {{ isGeneratingPlan ? 'Synthesizing DAG execution graph...' : 'Discussing with Master Planner...' }}
                  </span>
                  <span class="text-zinc-400 text-[10px]">
                    {{ isGeneratingPlan ? 'Allocating worker worktrees, estimates & acceptance tests' : 'Evaluating architecture, trade-offs, and project scope' }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Input Bar with Dynamic Target Selector -->
          <div class="p-4 border-t border-steel bg-slate-panel">
            <!-- Warning if no Master Planner is configured -->
            <div v-if="!hasMasterPlanner" class="mb-3 p-3 bg-amber-950/60 border border-amber-800 rounded flex items-center justify-between text-xs">
              <div class="flex items-center gap-2 text-amber-300">
                <span>⚠</span>
                <span>No Master Planner configured. Project briefing chat is disabled.</span>
              </div>
              <button @click="activeView = 'providers'" class="px-2.5 py-1 bg-gear-gold text-slate-deep font-bold rounded text-[10px]">
                Assign Master in Providers ▸
              </button>
            </div>

            <div class="flex items-center gap-2 mb-2 overflow-x-auto pb-1 justify-between">
              <div class="flex items-center gap-2">
                <span class="text-[10px] text-zinc-400 font-semibold whitespace-nowrap">Target Agent:</span>
                
                <!-- Master Option -->
                <button 
                  @click="selectedTarget = 'master'"
                  :disabled="!hasMasterPlanner"
                  class="px-2.5 py-1 rounded text-[10px] font-bold transition flex items-center gap-1 whitespace-nowrap disabled:opacity-40 disabled:cursor-not-allowed"
                  :class="selectedTarget === 'master' ? 'bg-gear-gold text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
                >
                  <span>🎪 Master Planner</span>
                  <span v-if="!hasMasterPlanner" class="text-[8px] text-amber-300">(None)</span>
                </button>

                <!-- Dynamic Active Automaton Options -->
                <button 
                  v-for="worker in activeWorkersList"
                  :key="worker.taskId"
                  @click="selectedTarget = worker.id"
                  class="px-2.5 py-1 rounded text-[10px] font-bold transition flex items-center gap-1.5 whitespace-nowrap"
                  :class="selectedTarget === worker.id ? 'bg-cyan-energy text-slate-deep' : 'bg-slate-inner text-zinc-400 border border-steel hover:text-zinc-200'"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-green-400 animate-ping"></span>
                  <span>🤖 {{ worker.name }} (Task {{ worker.taskId }})</span>
                </button>
              </div>

              <!-- Dedicated Generate DAG Plan Button (Only active when Master signals CAN_GENERATE) -->
              <div class="flex items-center gap-2">
                <span 
                  v-if="!canGeneratePlan && !isGeneratingPlan" 
                  class="text-[10px] text-zinc-500 italic hidden sm:inline"
                >
                  Discuss specs with Master to unlock plan
                </span>
                <button
                  @click="triggerGeneratePlan"
                  :disabled="isGeneratingPlan || !hasMasterPlanner || !canGeneratePlan"
                  class="px-3.5 py-1.5 rounded flex items-center gap-1.5 shadow transition font-bold text-[11px] whitespace-nowrap"
                  :class="canGeneratePlan 
                    ? 'bg-gradient-to-r from-gear-gold to-amber-500 hover:from-amber-400 hover:to-gear-gold text-slate-deep animate-pulse shadow-[0_0_15px_rgba(222,170,59,0.5)] cursor-pointer' 
                    : 'bg-slate-dark text-zinc-500 border border-steel/60 opacity-60 cursor-not-allowed'"
                  :title="canGeneratePlan ? 'Master has sufficient info! Click to decompose into DAG tasks.' : 'Master is still evaluating requirements with you. Keep discussing to unlock.'"
                >
                  <svg v-if="isGeneratingPlan" class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" stroke-opacity="0.25"></circle>
                    <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"></path>
                  </svg>
                  <span v-else>{{ canGeneratePlan ? '⚡' : '🔒' }}</span>
                  <span>{{ isGeneratingPlan ? 'Generating DAG...' : (canGeneratePlan ? '⚡ Ready: Generate Plan' : 'Generate Plan (Locked)') }}</span>
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
                  @keydown.enter.exact.prevent="sendMessage"
                  @keydown.enter.shift.exact="inputMsg += '\n'"
                  :placeholder="selectedTarget === 'master' 
                    ? (hasMasterPlanner ? 'Message Master Planner (e.g. features to implement in ' + (project.name || 'project') + ')...\nPress Enter to send, Shift+Enter for newline' : 'Chat disabled: Set an agent as Master Planner in Providers menu first.')
                    : `Direct instruction to automaton '${selectedTarget}'...\nPress Enter to send, Shift+Enter for newline`"
                  class="w-full bg-slate-inner border border-steel text-parchment text-xs px-4 py-2.5 rounded focus:outline-none focus:border-gear-gold disabled:opacity-40 disabled:cursor-not-allowed resize-y min-h-[52px] max-h-48 leading-relaxed font-mono"
                ></textarea>
              </div>
              <button 
                type="submit" 
                :disabled="(selectedTarget === 'master' && !hasMasterPlanner) || !inputMsg.trim()"
                class="px-5 py-3 bg-cyan-dim hover:bg-cyan-energy text-slate-deep font-bold text-xs rounded transition flex items-center gap-2 disabled:opacity-40 disabled:cursor-not-allowed h-[52px] shadow-sm mb-1"
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
                :key="worker.taskId"
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
                  <span>Attempt: {{ worker.attempt }}</span>
                </div>
              </div>
            </div>

            <div v-else class="p-3 bg-slate-dark/60 border border-steel/60 rounded text-center">
              <p class="text-[11px] text-zinc-400">No active running tasks yet.</p>
              <p class="text-[9px] text-zinc-500 mt-1">Discuss requirements in Chat to generate the task pipeline.</p>
            </div>

            <!-- BRIEF.md Preview -->
            <div class="pt-2 border-t border-steel">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-zinc-300">BRIEF.md</span>
                <span class="text-[9px] text-zinc-500">{{ project.brief ? 'Found' : 'Empty' }}</span>
              </div>
              <div class="p-3 bg-slate-inner border border-steel rounded font-mono text-[10px] leading-relaxed text-zinc-400 whitespace-pre-wrap max-h-56 overflow-y-auto">
{{ project.brief || '(BRIEF.md is empty. Send a message to Master Planner to generate it automatically.)' }}
              </div>
            </div>
          </div>
        </div>
      </section>
</template>

<script setup>
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
  isGeneratingPlan,
  triggerGeneratePlan,
  sendMessage,
  instructWorker
} = useRingmaster()
</script>
