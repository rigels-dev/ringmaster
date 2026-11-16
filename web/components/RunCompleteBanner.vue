<template>
  <div class="mb-6 p-4 rounded-xl border border-green-700/60 bg-green-950/30 space-y-3">
    <!-- What the run did -->
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <span class="text-xs font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-green-500 text-ink">
        {{ step ? `Step ${step.number} done` : 'All tasks done' }}
      </span>
      <span class="text-body text-zinc-300">
        {{ tasks.length }} task{{ tasks.length === 1 ? '' : 's' }} merged into main<span v-if="attempts"> · {{ attempts }} attempt{{ attempts === 1 ? '' : 's' }}</span>
      </span>
      <span v-if="step?.name" class="text-body text-zinc-400">· {{ step.name }}</span>
    </div>

    <p class="text-body text-zinc-300">
      <template v-if="step?.releases?.length">Released as <code class="text-gear-gold font-bold">{{ step.releases.join(', ') }}</code>.</template>
      <template v-else-if="step?.closed">Finished without a release.</template>
      <template v-else>Release it, push it or download it from Branches, or finish it without a release.</template>
    </p>

    <div class="flex flex-wrap items-center gap-2">
      <button
        @click="setView('branches')"
        class="px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink"
      >
        {{ step?.releases?.length || step?.closed ? 'Open Branches ▸' : 'Release and push in Branches ▸' }}
      </button>
      <button
        @click="setView('chat')"
        class="px-3.5 py-2 rounded text-body text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark"
      >
        Plan the next change ▸
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const { tasks, setView, project } = useRingmaster()

const steps = ref([])

const attempts = computed(() =>
  tasks.value.reduce((n, t) => n + (project.value.state?.task_states?.[t.id]?.attempt || 0), 0)
)
// The step this plan became: the one holding its tasks
const step = computed(() => {
  const ids = new Set(tasks.value.map(t => t.id))
  return [...steps.value].reverse().find(s => s.tasks.some(id => ids.has(id))) || null
})

onMounted(async () => {
  try {
    const data = await (await fetch('/api/steps')).json()
    if (data.ok) steps.value = data.steps
  } catch {}
})
</script>
