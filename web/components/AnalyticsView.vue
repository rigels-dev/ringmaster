<template>
      <section class="flex-1 flex flex-col p-6 overflow-y-auto">
        <div class="mb-6">
          <h2 class="text-lg font-bold text-gear-gold font-cinzel">Execution Analytics & Token Usage</h2>
          <p class="text-xs text-zinc-400">Accurate token breakdown per agent, role, and task vs estimates.</p>
        </div>

        <div class="grid grid-cols-3 gap-4 mb-6">
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Total Tokens Consumed</span>
            <p class="text-xl font-bold text-gear-gold mt-1">{{ totalTokensConsumed }}</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Project Status</span>
            <p class="text-xl font-bold text-cyan-energy mt-1">{{ project.isBlank ? 'PENDING INIT' : (project.state?.status || 'READY') }}</p>
          </div>
          <div class="bg-slate-panel border border-steel p-4 rounded-lg">
            <span class="text-[10px] text-zinc-400 uppercase">Tasks Completed</span>
            <p class="text-xl font-bold text-zinc-300 mt-1">{{ completedTasksCount }} / {{ tasks.length }}</p>
          </div>
        </div>
      </section>
</template>

<script setup>
const {
  project,
  tasks,
  totalTokensConsumed,
  completedTasksCount
} = useRingmaster()
</script>
