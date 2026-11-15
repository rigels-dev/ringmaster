<template>
  <div class="mb-6 p-4 rounded-xl border border-green-700/60 bg-green-950/30 space-y-4">
    <!-- What the run did -->
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <span class="text-xs font-bold uppercase tracking-wider px-2 py-0.5 rounded bg-green-500 text-ink">All tasks done</span>
      <span class="text-body text-zinc-300">
        {{ tasks.length }} task{{ tasks.length === 1 ? '' : 's' }} merged into main<span v-if="attempts"> · {{ attempts }} attempt{{ attempts === 1 ? '' : 's' }}</span>
      </span>
    </div>

    <!-- Release -->
    <div class="space-y-2">
      <template v-if="info.current">
        <p class="text-body text-zinc-200">
          Released as <code class="text-gear-gold font-bold">{{ info.current }}</code>.
          <span class="text-zinc-400">New work on main can be released as a new version.</span>
        </p>
      </template>

      <template v-else-if="loaded">
        <p class="text-body text-zinc-300">
          <strong class="text-zinc-200">Release it.</strong>
          <span v-if="info.latest"> The last release was <code class="text-zinc-200">{{ info.latest }}</code>.</span>
          The version is yours to choose<span v-if="info.suggestions.length">; these are suggestions</span>.
        </p>

        <div v-if="info.suggestions.length" class="flex flex-wrap gap-2">
          <button
            v-for="s in info.suggestions"
            :key="s.version"
            type="button"
            @click="version = s.version"
            class="px-2.5 py-1 rounded border text-xs font-bold"
            :class="version === s.version ? 'bg-cyan-950 border-cyan-500 text-cyan-200' : 'border-steel text-zinc-400 hover:text-zinc-200'"
          >
            <span class="font-mono">{{ s.version }}</span>
            <span class="font-normal"> · {{ kindLabel(s.kind) }}</span>
            <span v-if="s.kind === info.recommended" class="text-emerald-400"> ★</span>
          </button>
        </div>

        <form @submit.prevent="release(false)" class="flex flex-wrap items-center gap-2">
          <input
            v-model="version"
            type="text"
            spellcheck="false"
            autocomplete="off"
            placeholder="Version, in your own scheme"
            aria-label="Release version"
            class="w-56 bg-slate-inner border border-steel text-parchment text-body font-mono px-3 py-2 rounded focus:outline-none focus:border-gear-gold"
          />
          <button
            type="submit"
            :disabled="busy || !version.trim()"
            class="px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink disabled:opacity-40 disabled:cursor-not-allowed"
            title="Mark main as this release with a git tag. Nothing leaves your machine."
          >
            {{ busy ? 'Working…' : 'Tag release' }}
          </button>
          <button
            v-if="hasRemote"
            type="button"
            @click="confirmReleaseAndPush"
            :disabled="busy || !version.trim()"
            class="px-3.5 py-2 rounded text-body font-bold border border-gear-dark text-gear-gold hover:bg-slate-panel disabled:opacity-40 disabled:cursor-not-allowed"
            title="Tag the release, then push main and the tag to the remote"
          >
            Tag and push
          </button>
        </form>
      </template>

      <p v-if="error" class="p-2.5 rounded border border-rose-700 bg-rose-950 text-rose-300 text-xs break-words">{{ error }}</p>
    </div>

    <!-- Other next steps -->
    <div class="flex flex-wrap items-center gap-2 pt-3 border-t border-green-800/40">
      <a
        :href="archiveUrl"
        class="px-3 py-1.5 rounded text-body text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark"
        :title="`A zip of the committed files at ${archiveRef}`"
      >
        ↓ Download {{ archiveRef }} as zip
      </a>
      <button @click="setView('branches')" class="px-3 py-1.5 rounded text-body text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark">
        {{ hasRemote ? 'Push branches ▸' : 'Set a remote and push ▸' }}
      </button>
      <button @click="setView('chat')" class="px-3 py-1.5 rounded text-body text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark">
        Plan the next change ▸
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const { tasks, setView, showToast, confirmDialog, project } = useRingmaster()

const info = ref({ latest: '', current: '', suggestions: [], recommended: '' })
const loaded = ref(false)
const version = ref('')
const busy = ref(false)
const error = ref('')
const remoteUrl = ref('')

const hasRemote = computed(() => !!remoteUrl.value)
const attempts = computed(() => {
  const states = project.value.state?.task_states || {}
  return Object.values(states).reduce((n, ts) => n + (ts.attempt || 0), 0)
})
// A released project is downloaded at its tag; otherwise main as it stands
const archiveRef = computed(() => info.value.current || 'main')
const archiveUrl = computed(() => `/api/archive?format=zip&ref=${encodeURIComponent(archiveRef.value)}`)

const KIND = { first: 'first release', patch: 'fixes only', minor: 'new features', major: 'breaking changes' }
const kindLabel = (k) => KIND[k] || k

async function load() {
  try {
    const data = await (await fetch('/api/release')).json()
    if (!data.ok) throw new Error(data.error || 'Failed to read the release information')
    info.value = {
      latest: data.latest || '',
      current: data.current || '',
      suggestions: data.suggestions || [],
      recommended: data.recommended || ''
    }
    if (!version.value) {
      const pick = info.value.suggestions.find(s => s.kind === info.value.recommended) || info.value.suggestions[0]
      version.value = pick ? pick.version : ''
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loaded.value = true
  }
  try {
    const data = await (await fetch('/api/remote')).json()
    remoteUrl.value = data.ok ? data.url : ''
  } catch {}
}

// Pushing publishes the release, so name the destination and ask first
function confirmReleaseAndPush() {
  const v = version.value.trim()
  confirmDialog.value = {
    show: true,
    title: 'Tag and push',
    message: `Tag main as ${v}, then push main and that tag to ${remoteUrl.value}? Everyone with access to that repository will see the release. A release tag cannot be changed afterwards.`,
    confirmLabel: 'Tag and push',
    danger: false,
    action: () => release(true)
  }
}

async function release(push) {
  const v = version.value.trim()
  if (!v) return
  busy.value = true
  error.value = ''
  try {
    const res = await fetch('/api/release', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version: v, push })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Released', push ? `${v} is tagged and pushed.` : `main is tagged as ${v}.`, 'success')
      version.value = ''
    } else {
      // Keep what was typed, so it can be corrected
      error.value = data.error || 'The release failed'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
    await load()
  }
}

onMounted(load)
</script>
