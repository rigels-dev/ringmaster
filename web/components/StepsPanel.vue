<template>
  <div class="p-4 rounded-lg border border-steel bg-slate-panel space-y-3">
    <div>
      <h3 class="text-body font-bold text-zinc-200">Steps</h3>
      <p class="text-xs text-zinc-400">
        Every plan whose tasks are all done is a step. Release a step with a version tag, or finish it without one.
      </p>
    </div>

    <p v-if="!steps.length" class="text-xs text-zinc-500">
      No finished step yet. A plan becomes a step once all its tasks are done and merged.
    </p>

    <div
      v-for="s in ordered"
      :key="s.number"
      class="rounded border p-3 space-y-2"
      :class="s.number === openStep ? 'border-gear-dark bg-slate-inner/40' : 'border-steel/70'"
    >
      <!-- Title, state and what the step holds -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
        <template v-if="renaming === s.number">
          <form @submit.prevent="saveName(s)" class="flex items-center gap-2">
            <input
              v-model="nameInput"
              maxlength="80"
              :placeholder="`Step ${s.number}`"
              aria-label="Step name"
              class="w-64 bg-slate-inner border border-steel text-parchment text-body px-2 py-1 rounded focus:outline-none focus:border-gear-gold"
            />
            <button type="submit" class="text-xs text-gear-gold font-bold">Save</button>
            <button type="button" @click="renaming = 0" class="text-xs text-zinc-400">Cancel</button>
          </form>
        </template>
        <template v-else>
          <span class="text-body font-bold text-zinc-200">
            <span class="text-gear-gold">◆</span>
            {{ s.name ? `Step ${s.number}: ${s.name}` : `Step ${s.number}` }}
          </span>
          <button @click="startRename(s)" class="text-xs text-zinc-500 hover:text-zinc-300 underline">Rename</button>
        </template>

        <span class="ml-auto text-2xs px-2 py-0.5 rounded border font-bold whitespace-nowrap" :class="stateOf(s).cls" :title="stateOf(s).title">
          {{ stateOf(s).label }}
        </span>
      </div>

      <p class="text-xs text-zinc-400">
        {{ s.tasks.length }} task{{ s.tasks.length === 1 ? '' : 's' }}
        <span class="font-mono">{{ taskRange(s.tasks) }}</span>
        · finished {{ when(s.finished_at) }}
        · ends at <code class="text-zinc-300">{{ s.commit.slice(0, 7) }}</code>
      </p>

      <!-- Actions -->
      <div class="flex flex-wrap items-center gap-2">
        <a
          :href="archiveUrl(s)"
          class="px-2.5 py-1 rounded text-xs text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark"
          :title="`A zip of the committed files at ${refOf(s)}`"
        >↓ Download zip</a>

        <template v-if="!s.releases.length">
          <button
            v-if="openStep !== s.number"
            @click="openRelease(s)"
            class="px-2.5 py-1 rounded text-xs font-bold border border-gear-dark text-gear-gold hover:bg-slate-dark"
          >{{ s.closed ? 'Release it anyway' : 'Release…' }}</button>
          <button
            v-if="!s.closed"
            @click="setClosed(s, true)"
            :disabled="busy"
            class="px-2.5 py-1 rounded text-xs text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark disabled:opacity-40"
            title="Record that this step is finished without a version tag. Ringmaster stops offering a release for it."
          >Finish without release</button>
          <button
            v-else
            @click="setClosed(s, false)"
            :disabled="busy"
            class="px-2.5 py-1 rounded text-xs text-zinc-500 hover:text-zinc-300 underline"
          >Undo</button>
        </template>
        <template v-else>
          <a
            v-if="pageOf(s)"
            :href="pageOf(s)"
            target="_blank"
            rel="noopener"
            class="px-2.5 py-1 rounded text-xs text-zinc-300 border border-steel bg-slate-panel hover:bg-slate-dark"
          >Release page ↗</a>
          <button
            v-else-if="canMakePage(s) && pageStep !== s.number"
            @click="openPage(s)"
            class="px-2.5 py-1 rounded text-xs font-bold border border-gear-dark text-gear-gold hover:bg-slate-dark"
            :title="`Create the release page of ${s.releases[0]} on GitHub, with its notes`"
          >Create GitHub release page…</button>
        </template>
      </div>

      <!-- Release page for a release that is pushed but has none -->
      <div v-if="pageStep === s.number && s.releases.length" class="pt-2 border-t border-steel/60 space-y-2">
        <p class="text-xs text-zinc-300">
          <code class="text-zinc-200">{{ s.releases[0] }}</code> is on GitHub as a tag. Its release page shows these notes
          on <code class="text-zinc-200">{{ info.github.repo }}</code>.
        </p>
        <ReleaseNotesEditor v-model="notes" :step="s.number" :version="s.releases[0]" existing :disabled="busy" />
        <div class="flex flex-wrap items-center gap-2">
          <button
            @click="confirmPage(s)"
            :disabled="busy || !info.github.cli"
            class="px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink disabled:opacity-40 disabled:cursor-not-allowed"
          >{{ busy ? 'Working…' : 'Create release page' }}</button>
          <button type="button" @click="pageStep = 0" class="text-xs text-zinc-400 hover:text-zinc-200">Cancel</button>
          <span v-if="!info.github.cli" class="text-2xs text-zinc-500">Needs the GitHub CLI: install <code>gh</code>, then run <code>gh auth login</code>.</span>
        </div>
      </div>

      <!-- Release form for the open step -->
      <div v-if="openStep === s.number && !s.releases.length" class="pt-2 border-t border-steel/60 space-y-2">
        <p class="text-xs text-zinc-300">
          <span v-if="info.latest">The last release was <code class="text-zinc-200">{{ info.latest }}</code>. </span>
          The version is yours to choose<span v-if="info.suggestions.length">; these are suggestions</span>.
          The tag goes on <code class="text-zinc-200">{{ s.commit.slice(0, 7) }}</code>, where this step ended.
        </p>
        <div v-if="info.suggestions.length" class="flex flex-wrap gap-2">
          <button
            v-for="sg in info.suggestions"
            :key="sg.version"
            type="button"
            @click="version = sg.version"
            class="px-2.5 py-1 rounded border text-xs font-bold"
            :class="version === sg.version ? 'bg-cyan-950 border-cyan-500 text-cyan-200' : 'border-steel text-zinc-400 hover:text-zinc-200'"
          >
            <span class="font-mono">{{ sg.version }}</span>
            <span class="font-normal"> · {{ kindLabel(sg.kind) }}</span>
            <span v-if="sg.kind === info.recommended" class="text-emerald-400"> ★</span>
          </button>
        </div>
        <ReleaseNotesEditor v-model="notes" :step="s.number" :version="version" :disabled="busy" />
        <label v-if="remoteUrl && info.github.repo" class="flex flex-wrap items-center gap-2 text-xs text-zinc-300">
          <input type="checkbox" v-model="githubRelease" :disabled="!info.github.cli" class="accent-amber-500" />
          With Tag and push, also create a GitHub release page with these notes
          <span v-if="!info.github.cli" class="text-zinc-500">(needs the GitHub CLI: install <code>gh</code>, then run <code>gh auth login</code>)</span>
        </label>
        <form @submit.prevent="remoteUrl ? confirmReleaseAndPush(s) : release(s, false)" class="flex flex-wrap items-center gap-2">
          <input
            v-model="version"
            type="text"
            spellcheck="false"
            autocomplete="off"
            placeholder="Version, in your own scheme"
            aria-label="Release version"
            class="w-56 bg-slate-inner border border-steel text-parchment text-body font-mono px-3 py-2 rounded focus:outline-none focus:border-gear-gold"
          />
          <!-- With a remote, publishing is the usual aim: tagging only is the second choice -->
          <button
            v-if="remoteUrl"
            type="submit"
            :disabled="busy || !version.trim()"
            class="px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink disabled:opacity-40 disabled:cursor-not-allowed"
            title="Tag the release, then push main and the tag to the remote"
          >{{ busy ? 'Working…' : 'Tag and push' }}</button>
          <button
            :type="remoteUrl ? 'button' : 'submit'"
            @click="remoteUrl && release(s, false)"
            :disabled="busy || !version.trim()"
            :class="remoteUrl
              ? 'px-3.5 py-2 rounded text-body border border-steel text-zinc-300 bg-slate-panel hover:bg-slate-dark disabled:opacity-40 disabled:cursor-not-allowed'
              : 'px-3.5 py-2 rounded text-body font-bold bg-gear-gold hover:bg-gear-glow text-ink disabled:opacity-40 disabled:cursor-not-allowed'"
            :title="remoteUrl ? 'Tag the release on this machine only. Push sends it later.' : 'Tag the end of this step as a release. Nothing leaves your machine.'"
          >{{ busy && !remoteUrl ? 'Working…' : remoteUrl ? 'Tag only' : 'Tag release' }}</button>
          <button type="button" @click="openStep = 0" class="text-xs text-zinc-400 hover:text-zinc-200">Cancel</button>
        </form>
      </div>
    </div>

    <p v-if="error" class="p-2.5 rounded border border-rose-700 bg-rose-950 text-rose-300 text-xs break-words">{{ error }}</p>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  steps: { type: Array, default: () => [] }
})
const emit = defineEmits(['changed'])

const { showToast, confirmDialog } = useRingmaster()

const info = ref({ latest: '', suggestions: [], recommended: '', github: { repo: '', cli: false }, pages: {} })
const remoteUrl = ref('')
const unpushedTags = ref([])
const openStep = ref(0)
const version = ref('')
const busy = ref(false)
const error = ref('')
const renaming = ref(0)
const nameInput = ref('')
const notes = ref('')
const githubRelease = ref(true)
// The released step whose GitHub release page is being made
const pageStep = ref(0)

// Newest first
const ordered = computed(() => [...props.steps].sort((a, b) => b.number - a.number))

const KIND = { first: 'first release', patch: 'fixes only', minor: 'new features', major: 'breaking changes' }
const kindLabel = (k) => KIND[k] || k

function stateOf(s) {
  if (s.releases.length) {
    // A tag made without pushing exists only on this machine
    const local = remoteUrl.value ? s.releases.filter(t => unpushedTags.value.includes(t)) : []
    if (local.length) {
      return { label: `Tagged ${s.releases.join(', ')} · not pushed`, cls: 'bg-amber-950 text-amber-300 border-amber-700',
        title: 'The tag is only on this machine. Push it with Push in the Remote panel above.' }
    }
    return { label: `Released ${s.releases.join(', ')}`, cls: 'bg-green-950 text-green-400 border-green-800' }
  }
  if (s.closed) return { label: 'Finished without release', cls: 'bg-slate-inner text-zinc-400 border-steel' }
  return { label: 'Not released', cls: 'bg-amber-950 text-amber-300 border-amber-700' }
}

function taskRange(ids) {
  if (!ids.length) return ''
  return ids.length === 1 ? ids[0] : `${ids[0]}–${ids[ids.length - 1]}`
}

function when(iso) {
  const d = new Date(iso)
  return isNaN(d) ? '' : d.toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
}

// A released step is downloaded at its tag, otherwise at the commit it ended on
const refOf = (s) => s.releases[0] || s.commit.slice(0, 7)
const archiveUrl = (s) => `/api/archive?format=zip&ref=${encodeURIComponent(refOf(s))}`

async function loadInfo() {
  try {
    const data = await (await fetch('/api/release')).json()
    if (data.ok) {
      info.value = {
        latest: data.latest || '', suggestions: data.suggestions || [], recommended: data.recommended || '',
        github: data.github || { repo: '', cli: false }, pages: data.pages || {}
      }
    }
  } catch {}
  try {
    const data = await (await fetch('/api/remote')).json()
    remoteUrl.value = data.ok ? data.url : ''
    unpushedTags.value = data.ok ? (data.tags || []).filter(t => t.state !== 'synced').map(t => t.name) : []
  } catch {}
}

function pickSuggestion() {
  const pick = info.value.suggestions.find(sg => sg.kind === info.value.recommended) || info.value.suggestions[0]
  version.value = pick ? pick.version : ''
}

function openRelease(s) {
  error.value = ''
  notes.value = ''
  pageStep.value = 0
  openStep.value = s.number
  pickSuggestion()
}

// A release page is made on GitHub, for a tag that is there already
const pageOf = (s) => s.releases.map(t => info.value.pages[t]).find(Boolean) || ''
const canMakePage = (s) => Boolean(remoteUrl.value && info.value.github.repo) && !unpushedTags.value.includes(s.releases[0])

function openPage(s) {
  error.value = ''
  notes.value = ''
  openStep.value = 0
  pageStep.value = s.number
}

// Making the page publishes the notes, so say where first
function confirmPage(s) {
  const v = s.releases[0]
  confirmDialog.value = {
    show: true,
    title: 'Create release page',
    message: `Create the release page of ${v} on github.com/${info.value.github.repo}, with these notes? Everyone with access to that repository will see it.`,
    confirmLabel: 'Create release page',
    danger: false,
    action: () => makePage(s, v)
  }
}

async function makePage(s, v) {
  busy.value = true
  error.value = ''
  try {
    const res = await fetch('/api/release', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'page', version: v, notes: notes.value })
    })
    const data = await res.json()
    if (!data.ok) throw new Error(data.error || 'The release page could not be made')
    showToast('Release Page Created', `${v} has its release page on GitHub.`, 'success')
    pageStep.value = 0
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
    await loadInfo()
  }
}

// The newest step that is neither released nor finished without a release
// is ready to release, so its form starts open
function openNewestPending() {
  const pending = ordered.value.find(s => !s.releases.length && !s.closed)
  if (pending && !openStep.value) openRelease(pending)
}

function startRename(s) {
  renaming.value = s.number
  nameInput.value = s.name || ''
}

async function post(body) {
  const res = await fetch('/api/steps', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  })
  const data = await res.json()
  if (!data.ok) throw new Error(data.error || 'Failed to update the step')
}

async function saveName(s) {
  error.value = ''
  try {
    await post({ action: 'name', step: s.number, name: nameInput.value })
    renaming.value = 0
    emit('changed')
  } catch (err) {
    error.value = err.message
  }
}

async function setClosed(s, closed) {
  busy.value = true
  error.value = ''
  try {
    await post({ action: closed ? 'close' : 'reopen', step: s.number })
    if (closed && openStep.value === s.number) openStep.value = 0
    showToast(closed ? 'Step Finished' : 'Step Reopened',
      closed ? `Step ${s.number} is finished without a release.` : `Step ${s.number} can be released again.`, 'info')
    emit('changed')
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
  }
}

// Pushing publishes the release, so name the destination and ask first
function confirmReleaseAndPush(s) {
  const v = version.value.trim()
  confirmDialog.value = {
    show: true,
    title: 'Tag and push',
    message: `Tag step ${s.number} as ${v}, then push main and that tag to ${remoteUrl.value}${wantsPage() ? `, and create its release page on GitHub with these notes` : ''}? Everyone with access to that repository will see the release. A release tag cannot be changed afterwards.`,
    confirmLabel: 'Tag and push',
    danger: false,
    action: () => release(s, true)
  }
}

const wantsPage = () => Boolean(githubRelease.value && info.value.github.repo && info.value.github.cli)

async function release(s, push) {
  const v = version.value.trim()
  if (!v) return
  busy.value = true
  error.value = ''
  try {
    const res = await fetch('/api/release', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ version: v, push, step: s.number, notes: notes.value, githubRelease: push && wantsPage() })
    })
    const data = await res.json()
    if (data.ok) {
      showToast('Released', push ? `Step ${s.number} is tagged ${v} and pushed${data.page_url ? ', with its release page' : ''}.` : `Step ${s.number} is tagged ${v}.`, 'success')
      openStep.value = 0
      version.value = ''
      notes.value = ''
      // The tag and the push are made; only the page is missing, and it can be made from the step
      if (data.page_error) error.value = `${v} is tagged and pushed, but its release page was not made: ${data.page_error}`
    } else {
      // Keep what was typed, so it can be corrected
      error.value = data.error || 'The release failed'
    }
  } catch (err) {
    error.value = err.message
  } finally {
    busy.value = false
    await loadInfo()
    emit('changed')
  }
}

// The remote may have been set or pushed to since: "Tag and push" and the
// pushed state of each release depend on it
watch(() => props.steps, async () => {
  await loadInfo()
  openNewestPending()
})
onMounted(async () => {
  await loadInfo()
  openNewestPending()
})
</script>
