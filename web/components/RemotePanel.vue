<template>
  <div class="p-4 rounded-lg border border-steel bg-slate-panel space-y-3">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
      <h3 class="text-body font-bold text-zinc-200">Remote</h3>

      <template v-if="status.url && !editing">
        <code class="text-xs text-cyan-energy break-all min-w-0">{{ status.url }}</code>
        <button @click="startEditing" class="text-xs text-zinc-400 hover:text-zinc-200 underline">Change</button>

        <div class="ml-auto flex items-center gap-3">
          <span class="text-xs" :class="pending > 0 ? 'text-amber-300' : 'text-zinc-400'">
            {{ pending > 0 ? `${pending} branch${pending === 1 ? '' : 'es'} not pushed` : 'Everything in main is pushed' }}
          </span>
          <button
            v-if="merged.length && !pruneOpen"
            @click="openPrune"
            :disabled="pushing || pruning"
            class="px-3 py-1.5 rounded text-body border border-steel text-zinc-300 bg-slate-panel hover:bg-slate-dark disabled:opacity-40"
            title="Delete the task branches whose work is already in main"
          >
            Delete merged branches…
          </button>
          <!-- Something to push: push it -->
          <button
            v-if="pending > 0 || pushing"
            @click="confirmPush"
            :disabled="pushing"
            class="px-3.5 py-1.5 rounded text-body font-bold border transition bg-gear-gold hover:bg-gear-glow border-gear-gold text-ink disabled:opacity-40 disabled:cursor-not-allowed"
            title="Push main and the task branches to the remote. Nothing is forced."
          >
            {{ pushing ? 'Pushing…' : '↑ Push' }}
          </button>
          <!-- Everything is pushed and the plan is finished: the next step is a new plan -->
          <button
            v-else-if="planFinished"
            @click="setView('chat')"
            class="px-3.5 py-1.5 rounded text-body font-bold border transition bg-gear-gold hover:bg-gear-glow border-gear-gold text-ink"
            title="Everything is pushed. Plan the next change with the Master in Chat."
          >
            Go to next plan ▸
          </button>
        </div>
      </template>

      <p v-else-if="!editing && !loaded" class="text-xs text-zinc-500">Loading…</p>
    </div>

    <!-- Set or change the remote -->
    <form v-if="editing || (loaded && !status.url)" @submit.prevent="saveUrl" class="space-y-2">
      <p v-if="!status.url" class="text-xs text-zinc-400">
        This project has no remote yet. Paste a repository URL to publish main and the task branches there.
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="urlInput"
          type="text"
          spellcheck="false"
          autocomplete="off"
          placeholder="https://github.com/you/project.git  or  git@github.com:you/project.git"
          class="flex-1 min-w-[260px] bg-slate-inner border border-steel text-parchment text-body font-mono px-3 py-2 rounded focus:outline-none focus:border-gear-gold"
        />
        <button
          type="submit"
          :disabled="saving || !urlInput.trim()"
          class="px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {{ saving ? 'Saving…' : 'Set remote' }}
        </button>
        <button v-if="editing" type="button" @click="editing = false" class="px-3 py-2 rounded text-body text-zinc-400 hover:text-zinc-200 border border-steel">
          Cancel
        </button>
      </div>
      <p class="text-2xs text-zinc-500">
        Create the repository on your git host first. Pushing uses your own git login (an SSH key or a credential helper); Ringmaster never stores a password.
      </p>
    </form>

    <p v-if="error" class="p-2.5 rounded border border-rose-700 bg-rose-950 text-rose-300 text-xs break-words">
      {{ error }}
      <span v-if="errorHint" class="block mt-1 text-rose-300/80">{{ errorHint }}</span>
    </p>

    <!-- Delete merged task branches -->
    <div v-if="pruneOpen" class="p-3 rounded border border-steel bg-slate-inner/40 space-y-2">
      <p class="text-xs text-zinc-300">
        Delete {{ merged.length }} task branch{{ merged.length === 1 ? '' : 'es' }} merged into main. The work stays in main's
        history; only the branch names go, and they leave the branch map below. Unfinished task branches and your own branches are kept.
      </p>
      <div class="flex flex-wrap gap-1.5">
        <code v-for="b in merged" :key="b.name" class="px-1.5 py-0.5 rounded border border-steel text-2xs text-zinc-400">{{ b.name }}</code>
      </div>
      <label v-if="status.url && mergedOnRemote" class="flex items-center gap-2 text-xs text-zinc-300">
        <input type="checkbox" v-model="pruneRemote" class="accent-amber-500" />
        Also delete them on the remote ({{ mergedOnRemote }} there). A branch someone moved on the remote is kept.
      </label>
      <div class="flex items-center gap-2">
        <button
          @click="prune"
          :disabled="pruning"
          class="px-3.5 py-1.5 rounded text-body font-bold bg-rose-700 hover:bg-rose-600 text-white disabled:opacity-40"
        >
          {{ pruning ? 'Deleting…' : pruneRemote ? 'Delete here and on the remote' : 'Delete here' }}
        </button>
        <button @click="pruneOpen = false" :disabled="pruning" class="text-xs text-zinc-400 hover:text-zinc-200">Cancel</button>
      </div>
    </div>

    <GitIdentityNotice :author="status.author" />

    <!-- Push state of each branch -->
    <div v-if="status.url && status.branches.length" class="flex flex-wrap gap-1.5">
      <span
        v-for="b in status.branches"
        :key="b.name"
        class="px-2 py-0.5 rounded border text-2xs font-mono flex items-center gap-1.5"
        :class="stateStyle(b).chip"
        :title="stateStyle(b).title"
      >
        <span class="w-1.5 h-1.5 rounded-full" :class="stateStyle(b).dot"></span>
        {{ b.name }}
        <span v-if="b.merged && b.state !== 'synced'" class="font-sans text-zinc-500">· in main</span>
        <span v-if="lastResult[b.name]" class="font-sans" :class="lastResult[b.name] === 'rejected' ? 'text-rose-300' : 'text-zinc-500'">· {{ resultLabel(lastResult[b.name]) }}</span>
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  // Bumped by the parent to ask for a reload
  refreshKey: { type: Number, default: 0 }
})
// Branches were deleted, so the branch map should reload
const emit = defineEmits(['changed'])

const { showToast, confirmDialog, setView, tasks, isRunActive } = useRingmaster()

// The current plan has run to the end: every task done, and no run going
const planFinished = computed(() =>
  tasks.value.length > 0 &&
  !isRunActive.value &&
  tasks.value.every(t => t.status === 'COMPLETED' || t.status === 'DONE')
)

const status = ref({ url: '', branches: [], author: null })
const loaded = ref(false)
const error = ref('')
const errorHint = ref('')
const editing = ref(false)
const urlInput = ref('')
const saving = ref(false)
const pushing = ref(false)
const lastResult = ref({}) // branch -> result of the most recent push

// A merged task branch's work goes with main, so only main and unfinished
// task branches wait on a push
const pending = computed(() => status.value.branches.filter(b => b.state !== 'synced' && !b.merged).length)
const merged = computed(() => status.value.branches.filter(b => b.merged))
const mergedOnRemote = computed(() => merged.value.filter(b => b.state !== 'unpushed').length)

const pruneOpen = ref(false)
const pruneRemote = ref(false)
const pruning = ref(false)

const STATE = {
  synced: { chip: 'border-green-800 bg-green-950 text-green-400', dot: 'bg-green-400', title: 'The remote has this branch as it is now' },
  ahead: { chip: 'border-amber-700 bg-amber-950 text-amber-300', dot: 'bg-amber-400', title: 'Has commits that are not on the remote yet' },
  unpushed: { chip: 'border-steel bg-slate-inner text-zinc-400', dot: 'bg-zinc-500', title: 'Not on the remote yet' },
  merged: { chip: 'border-steel/60 bg-slate-inner/50 text-zinc-500', dot: 'bg-zinc-600', title: 'Merged into main: its work is published with main, so the branch itself need not be pushed' }
}
const stateStyle = (b) => (b.merged && b.state !== 'synced' ? STATE.merged : STATE[b.state] || STATE.unpushed)

const RESULT = { new: 'new', updated: 'updated', 'up-to-date': 'no change', rejected: 'rejected' }
const resultLabel = (r) => RESULT[r] || r

async function load() {
  try {
    const res = await fetch('/api/remote')
    const data = await res.json()
    if (!data.ok) throw new Error(data.error || 'Failed to read the remote')
    status.value = { url: data.url, branches: data.branches, author: data.author }
  } catch (err) {
    error.value = err.message
    errorHint.value = ''
  } finally {
    loaded.value = true
  }
}

function startEditing() {
  urlInput.value = status.value.url
  editing.value = true
}

async function saveUrl() {
  saving.value = true
  error.value = ''
  errorHint.value = ''
  try {
    const res = await fetch('/api/remote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'set', url: urlInput.value.trim() })
    })
    const data = await res.json()
    if (!data.ok) throw new Error(data.error || 'Failed to set the remote')
    editing.value = false
    lastResult.value = {}
    showToast('Remote Set', data.url, 'success')
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

// Pushing publishes code, so name the destination and ask first
function confirmPush() {
  const taskBranches = status.value.branches.filter(b => b.name !== 'main').length
  confirmDialog.value = {
    show: true,
    title: 'Push to remote',
    message: `Push main and ${taskBranches} task branch${taskBranches === 1 ? '' : 'es'} to ${status.value.url}? Everyone with access to that repository will see them. Nothing is overwritten.`,
    confirmLabel: 'Push',
    danger: false,
    action: push
  }
}

async function push() {
  pushing.value = true
  error.value = ''
  errorHint.value = ''
  try {
    const res = await fetch('/api/remote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'push' })
    })
    const data = await res.json()
    lastResult.value = Object.fromEntries((data.results || []).map(r => [r.branch, r.result]))
    if (data.ok) {
      const sent = (data.results || []).filter(r => r.result === 'new' || r.result === 'updated').length
      showToast('Pushed', sent ? `${sent} branch${sent === 1 ? '' : 'es'} sent to the remote.` : 'The remote already had everything.', 'success')
    } else {
      error.value = data.error || 'The push failed'
      const rejected = (data.results || []).some(r => r.result === 'rejected')
      errorHint.value = rejected
        ? ''
        : 'Pushing uses your own git login. Check that "git push" works from a terminal in this project.'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    pushing.value = false
    await load()
  }
}

function openPrune() {
  pruneRemote.value = false
  pruneOpen.value = true
}

const PRUNE = { deleted: 'deleted', 'remote-deleted': 'deleted on the remote', kept: 'kept' }

async function prune() {
  pruning.value = true
  error.value = ''
  errorHint.value = ''
  try {
    const res = await fetch('/api/remote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'prune', remote: pruneRemote.value })
    })
    const data = await res.json()
    const results = data.results || []
    const here = results.filter(r => r.result === 'deleted').length
    const there = results.filter(r => r.result === 'remote-deleted').length
    const kept = results.filter(r => r.result === 'kept')
    if (data.ok || here || there) {
      const parts = [`${here} deleted here`]
      if (pruneRemote.value) parts.push(`${there} on the remote`)
      showToast('Branches Deleted', parts.join(', ') + '.', 'success')
      pruneOpen.value = false
    }
    if (!data.ok || kept.length) {
      error.value = data.error || 'Some branches were kept.'
      errorHint.value = kept.map(r => `${r.branch}: ${PRUNE[r.result]}${r.detail ? ` (${r.detail})` : ''}`).join(' · ')
    }
    emit('changed')
  } catch (err) {
    error.value = err.message
  } finally {
    pruning.value = false
    await load()
  }
}

watch(() => props.refreshKey, load)
onMounted(load)
</script>
