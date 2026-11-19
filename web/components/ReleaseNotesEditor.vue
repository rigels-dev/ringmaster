<template>
  <div class="space-y-1.5">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <label :for="id" class="text-xs font-bold text-zinc-300">Release notes</label>
      <span class="text-2xs text-zinc-500">{{ kept ? 'The notes this release was made with.' : 'Drafted from the step\'s tasks.' }} Markdown; edit freely.</span>
      <span class="ml-auto flex items-center gap-3">
        <button
          type="button"
          @click="writeSummary"
          :disabled="disabled || loading || aiBusy"
          class="px-2.5 py-1 rounded text-xs font-bold border border-cyan-700 text-cyan-300 hover:bg-cyan-950 disabled:opacity-40 disabled:cursor-not-allowed"
          title="The planner model reads this step's tasks and the files they changed, and writes the summary. One model call, on your provider's quota."
        >{{ aiBusy ? 'Writing…' : '✦ Write summary with AI' }}</button>
        <button
          type="button"
          @click="load"
          :disabled="disabled || loading || aiBusy"
          class="text-xs text-zinc-500 hover:text-zinc-300 underline disabled:opacity-40"
          title="Throw away the changes and start from the draft again"
        >Reset</button>
      </span>
    </div>
    <textarea
      :id="id"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
      :disabled="disabled || loading"
      :maxlength="MAX"
      rows="9"
      spellcheck="true"
      :placeholder="loading ? 'Drafting…' : 'What this release brings'"
      class="w-full bg-slate-inner border border-steel text-parchment text-xs font-mono leading-relaxed px-3 py-2 rounded focus:outline-none focus:border-gear-gold disabled:opacity-60"
    ></textarea>
    <p v-if="hint" class="text-2xs" :class="hintError ? 'text-rose-300' : 'text-zinc-500'">{{ hint }}</p>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { insertSummary, needsSummary } from '~/utils/releaseNotes'

const props = defineProps({
  modelValue: { type: String, default: '' },
  step: { type: Number, required: true },
  // The release's version: the notes of one that exists are loaded as they were made
  version: { type: String, default: '' },
  // Only the version of a release that exists is sent, so a version still being typed changes nothing
  existing: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue'])

const MAX = 20000
const id = `release-notes-${Math.random().toString(36).slice(2, 8)}`
const loading = ref(false)
const aiBusy = ref(false)
const kept = ref(false)
const hint = ref('')
const hintError = ref(false)

function query() {
  const q = new URLSearchParams({ step: String(props.step) })
  if (props.existing && props.version) q.set('version', props.version)
  return q
}

async function load() {
  loading.value = true
  hint.value = ''
  try {
    const data = await (await fetch(`/api/release-notes?${query()}`)).json()
    if (!data.ok) throw new Error(data.error)
    kept.value = Boolean(data.kept)
    emit('update:modelValue', data.notes || '')
  } catch (err) {
    hintError.value = true
    hint.value = `The draft could not be made: ${err.message || err}. Write the notes yourself.`
  } finally {
    loading.value = false
  }
}

async function writeSummary() {
  aiBusy.value = true
  hint.value = ''
  hintError.value = false
  try {
    const res = await fetch('/api/release-notes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(Object.fromEntries(query()))
    })
    const data = await res.json()
    if (!data.ok) throw new Error(data.error)
    // Only the summary is taken: whatever the user changed in the notes stays
    emit('update:modelValue', insertSummary(props.modelValue, data.summary))
    const u = data.usage || {}
    const tokens = (u.tokens_in || 0) + (u.tokens_out || 0)
    hint.value = `Summary written by AI${tokens ? ` (${tokens.toLocaleString()} tokens)` : ''}. Read it before you release.`
  } catch (err) {
    hintError.value = true
    hint.value = err.message || String(err)
  } finally {
    aiBusy.value = false
  }
}

defineExpose({ needsSummary: () => needsSummary(props.modelValue) })
onMounted(load)
</script>
