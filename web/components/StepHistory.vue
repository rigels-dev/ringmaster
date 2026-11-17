<template>
  <div v-if="entries.length" class="pt-2 border-t border-steel space-y-2">
    <div class="flex items-center justify-between">
      <span class="text-body font-bold text-zinc-300">Steps</span>
      <button @click="setView('branches')" class="text-2xs text-zinc-500 hover:text-zinc-300 underline">Releases in Branches</button>
    </div>

    <div v-for="e in entries" :key="e.number" class="rounded border border-steel/70 bg-slate-inner/50">
      <button
        @click="toggle(e.number)"
        class="w-full flex items-center gap-2 px-2.5 py-2 text-left"
        :aria-expanded="isOpen(e)"
      >
        <span class="text-zinc-500 text-2xs w-3">{{ isOpen(e) ? '▾' : '▸' }}</span>
        <span class="text-xs font-bold text-gear-gold whitespace-nowrap">◆ Step {{ e.number }}</span>
        <span v-if="e.name" class="text-xs text-zinc-300 truncate min-w-0">{{ e.name }}</span>
        <span class="ml-auto text-2xs px-1.5 py-0.5 rounded border whitespace-nowrap" :class="e.badge.cls">{{ e.badge.label }}</span>
      </button>

      <div v-if="isOpen(e)" class="px-2.5 pb-2.5 space-y-2 text-xs">
        <div>
          <div class="text-2xs uppercase text-zinc-500">Asked</div>
          <p v-if="e.asked" class="text-zinc-300 line-clamp-3" :title="e.asked">“{{ e.asked }}”</p>
          <p v-else class="text-zinc-500">Nothing asked yet.</p>
          <p v-if="e.moreRequests" class="text-2xs text-zinc-500">and {{ e.moreRequests }} more message{{ e.moreRequests === 1 ? '' : 's' }}</p>
        </div>

        <div v-if="e.tasks.length">
          <div class="text-2xs uppercase text-zinc-500">{{ e.finished ? 'Built' : 'Planned' }}</div>
          <ul class="space-y-1">
            <li v-for="t in e.tasks" :key="t.id" class="min-w-0">
              <div class="flex gap-1.5 min-w-0">
                <span class="font-mono text-zinc-500 shrink-0">{{ t.id }}</span>
                <span class="text-zinc-200 truncate" :title="t.title">{{ t.title || '—' }}</span>
              </div>
              <code v-if="t.branch" class="block text-2xs text-zinc-500 truncate pl-8">⎇ {{ t.branch }}</code>
            </li>
          </ul>
        </div>

        <p v-if="e.finishedAt" class="text-2xs text-zinc-500">Finished {{ e.finishedAt }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { requestsByStep } from '~/utils/chatSteps'

const { steps, currentStep, stepStatus, planFinished, tasks, messages, project, setView } = useRingmaster()

// Which entries the user opened or closed; the rest follow the default:
// the step in progress open, finished ones closed
const toggled = ref({})
const isOpen = (e) => (e.number in toggled.value ? toggled.value[e.number] : !e.finished)
const toggle = (n) => {
  const e = entries.value.find(x => x.number === n)
  toggled.value = { ...toggled.value, [n]: !isOpen(e) }
}

const requests = computed(() => {
  const byStep = {}
  for (const g of requestsByStep(messages.value, steps.value.length, planFinished.value)) byStep[g.step] = g.requests
  return byStep
})

function taskInfo(id) {
  const ts = project.value.state?.task_states?.[id] || {}
  const planned = tasks.value.find(t => t.id === id)
  return { id, title: ts.title || planned?.title || '', branch: ts.branch || planned?.branch || '' }
}

function when(iso) {
  const d = new Date(iso)
  return isNaN(d) ? '' : d.toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
}

const entries = computed(() => {
  const list = []
  const asked = (n) => {
    const reqs = requests.value[n] || []
    return { asked: reqs[0] || '', moreRequests: Math.max(0, reqs.length - 1) }
  }

  // The step being worked on: the current plan if it is not finished yet
  const current = currentStep.value
  const inProgress = tasks.value.length > 0 && !planFinished.value
  if (inProgress || (requests.value[current] || []).length) {
    list.push({
      number: current,
      name: '',
      finished: false,
      badge: { label: stepStatus.value, cls: 'bg-cyan-950 text-cyan-300 border-cyan-800' },
      ...asked(current),
      tasks: inProgress ? tasks.value.map(t => taskInfo(t.id)) : [],
      finishedAt: ''
    })
  }

  // Finished steps, newest first
  for (const st of [...steps.value].sort((a, b) => b.number - a.number)) {
    let badge = { label: 'not released', cls: 'bg-amber-950 text-amber-300 border-amber-700' }
    if (st.releases?.length) badge = { label: st.releases.join(', '), cls: 'bg-green-950 text-green-400 border-green-800' }
    else if (st.closed) badge = { label: 'no release', cls: 'bg-slate-inner text-zinc-400 border-steel' }
    list.push({
      number: st.number,
      name: st.name || '',
      finished: true,
      badge,
      ...asked(st.number),
      tasks: (st.tasks || []).map(taskInfo),
      finishedAt: when(st.finished_at)
    })
  }
  return list
})
</script>
