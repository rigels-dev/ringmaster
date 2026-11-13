<template>
  <div v-if="missing" class="p-3 rounded-lg border border-amber-700/60 bg-amber-950/30 text-xs text-zinc-300 space-y-2">
    <p>
      <strong class="text-amber-300">Git has no user.name / user.email set.</strong>
      Commits will be authored by "Ringmaster" and will not be linked to your account on a git host.
      To make them yours, run these once in a terminal, using an email your git host knows:
    </p>
    <pre class="p-2 rounded bg-slate-inner border border-steel text-zinc-200 overflow-x-auto select-all">git config --global user.name  "Your Name"
git config --global user.email "you@example.com"</pre>
    <p class="text-zinc-500">Commits that are already made keep their author.</p>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  // The commit author as /api/remote reports it. Left out, the notice asks for it itself.
  author: { type: Object, default: undefined }
})

const fetched = ref(null)
const current = computed(() => (props.author !== undefined ? props.author : fetched.value))
// Shown only when the answer is known: a failed or pending lookup shows nothing
const missing = computed(() => !!current.value && current.value.is_user === false)

onMounted(async () => {
  if (props.author !== undefined) return
  try {
    const data = await (await fetch('/api/remote')).json()
    if (data.ok) fetched.value = data.author
  } catch {}
})
</script>
