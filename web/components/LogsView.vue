<template>
      <section class="flex-1 flex flex-col p-6 overflow-hidden">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-gear-gold font-cinzel">Orchestration Logs</h2>
            <p class="text-xs text-zinc-400">Live output of the Ringmaster engine dispatching tasks to automatons.</p>
          </div>
          <div class="flex items-center gap-3">
            <button
              v-if="isRunActive"
              @click="stopOrchestration"
              class="px-3 py-2 bg-red-950/80 hover:bg-red-900 border border-red-700/60 text-red-200 text-xs font-bold rounded transition"
              title="Gracefully stop orchestration"
            >
              ⏹ Stop
            </button>
            <button
              :disabled="isRunActive || runLogs.length === 0"
              @click="clearRunLogs"
              class="px-3.5 py-2 bg-slate-panel hover:bg-slate-panel/70 text-zinc-300 font-bold text-xs rounded border border-steel transition disabled:opacity-40 disabled:cursor-not-allowed"
              :title="isRunActive ? 'Logs can be cleared once the run has exited.' : 'Clear orchestration log history'"
            >
              Clear Logs
            </button>
          </div>
        </div>

        <!-- Run Status Strip -->
        <div class="mb-4 p-3 rounded-lg border border-steel/80 bg-slate-panel flex flex-wrap items-center gap-x-6 gap-y-2 text-[11px]">
          <div class="flex items-center gap-2">
            <span class="w-2.5 h-2.5 rounded-full" :class="isRunActive ? 'bg-cyan-400 animate-ping' : 'bg-zinc-500'"></span>
            <span class="font-bold" :class="isRunActive ? 'text-cyan-300' : 'text-zinc-400'">{{ isRunActive ? 'RUNNING' : 'IDLE' }}</span>
            <span v-if="isRunActive" class="text-[9px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-300 border border-cyan-700/60 animate-pulse">STREAMING</span>
          </div>
          <span class="text-zinc-400">PID: <strong class="text-zinc-200">{{ isRunActive ? (runPid || 'Live') : 'Exited' }}</strong></span>
          <span class="text-zinc-400">Project: <strong class="text-gear-gold">{{ project.state?.status || 'READY' }}</strong></span>
          <span class="text-zinc-400">Working Automatons: <strong :class="activeWorkersList.length > 0 ? 'text-green-400' : 'text-zinc-500'">{{ activeWorkersList.length }}</strong></span>
          <span class="text-zinc-400">Tasks Completed: <strong class="text-zinc-200">{{ completedTasksCount }} / {{ tasks.length }}</strong></span>
        </div>

        <p v-if="isRunExternal" class="mb-3 text-[11px] text-amber-300/90">
          ℹ This run was started outside the current Web UI session (CLI or before a server restart). Its console output is not captured here; task progress is still tracked from state.json.
        </p>

        <!-- Full-height terminal console -->
        <div ref="logsPageRef" class="flex-1 min-h-0 bg-slate-inner/90 p-4 rounded-lg font-mono text-[11px] leading-relaxed overflow-y-auto space-y-1 border border-steel/40 text-zinc-300 select-text">
          <div v-if="runLogs.length === 0" class="h-full flex items-center justify-center text-zinc-500 text-xs">
            No orchestration output yet. Execute an approved plan from the Tasks view.
          </div>
          <div v-for="(log, idx) in runLogs" :key="idx" class="break-all whitespace-pre-wrap font-mono flex items-start gap-1.5">
            <span class="text-zinc-600 select-none">›</span>
            <span :class="logLineClass(log)">{{ log }}</span>
          </div>
        </div>
      </section>
</template>

<script setup>
const {
  project,
  tasks,
  activeWorkersList,
  completedTasksCount,
  isRunActive,
  isRunExternal,
  runPid,
  runLogs,
  logsPageRef,
  logLineClass,
  clearRunLogs,
  stopOrchestration
} = useRingmaster()
</script>
