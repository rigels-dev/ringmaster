<template>
  <div v-if="stack.length" class="mt-3 pt-2 border-t border-steel/40 text-body space-y-1.5">
    <div class="flex flex-wrap items-center gap-2">
      <span class="font-bold text-zinc-300">Commands</span>
      <span class="text-xs text-zinc-500">
        {{ approved ? 'allowed by this plan:' : 'approving this plan allows these presets:' }}
      </span>
    </div>
    <div v-for="name in stack" :key="name" class="text-xs pl-1">
      <template v-if="presetOf(name)">
        <span class="font-mono font-bold text-gear-gold">{{ name }}</span>
        <span class="text-zinc-300"> · {{ presetOf(name).allow.join(', ') }}</span>
        <span v-if="presetOf(name).always_ask.length" class="block pl-4 text-zinc-500">
          still asks before: {{ presetOf(name).always_ask.join(', ') }}
        </span>
      </template>
      <template v-else-if="loaded">
        <span class="font-mono font-bold text-amber-300">{{ name }}</span>
        <span class="text-amber-300"> · not a preset; its commands will be asked about one by one</span>
      </template>
    </div>
    <p class="text-2xs text-zinc-500">
      Anything else, and anything that pushes, publishes or deletes, still stops the task to ask you.
      See them all with <code>ringmaster presets</code>.
    </p>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  stack: { type: Array, default: () => [] },
  approved: { type: Boolean, default: false }
})

const presets = ref([])
const loaded = ref(false)
const byName = computed(() => Object.fromEntries(presets.value.map(p => [p.name, p])))
const presetOf = (name) => byName.value[String(name).toLowerCase()] || null

onMounted(async () => {
  try {
    const data = await (await fetch('/api/presets')).json()
    if (data.ok) presets.value = data.presets || []
  } catch {}
  loaded.value = true
})
</script>
