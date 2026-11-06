<template>
  <!-- Launcher bubble, chat-widget style: toggles the floating window; filled while open -->
  <button
    @click="open ? minimize() : openPanel()"
    class="fixed bottom-5 right-5 z-40 w-12 h-12 rounded-full border shadow-xl flex items-center justify-center hover:scale-105 transition"
    :class="open ? 'bg-gear-gold text-ink border-gear-gold' : 'bg-slate-dark text-gear-gold border-gear-dark hover:border-gear-gold'"
    :title="open ? 'Hide terminal (shells keep running) · Ctrl+`' : 'Open terminal · Ctrl+`'"
    :aria-label="open ? 'Hide terminal' : 'Open terminal'"
  >
    <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round">
      <path d="M5 7l5 5-5 5" />
      <path d="M12 17h7" />
    </svg>
    <span v-if="hasUnread && !open" class="absolute top-0.5 right-0.5 w-2.5 h-2.5 rounded-full bg-cyan-400 ring-2 ring-slate-dark"></span>
  </button>

  <!-- Floating window above the bubble; kept mounted while hidden so shells and scrollback survive -->
  <div
    v-show="open"
    class="fixed z-40 flex flex-col rounded-xl overflow-hidden border border-steel bg-slate-panel shadow-2xl"
    :style="windowStyle"
  >
    <!-- Corner grip: anchored bottom-right, so dragging the top-left corner resizes -->
    <div
      v-if="!maximized"
      class="absolute top-0 left-0 w-4 h-4 z-10 cursor-nwse-resize touch-none"
      title="Drag to resize · double-click to reset"
      @pointerdown="startResize"
      @dblclick="setSize(DEFAULT_SIZE.w, DEFAULT_SIZE.h)"
    >
      <svg class="w-3 h-3 m-0.5 transition-colors" :class="isResizing ? 'text-gear-gold' : 'text-zinc-500'" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M1 7V1h6M1 11 11 1" stroke-linecap="round" />
      </svg>
    </div>

    <!-- Tab bar -->
    <div class="h-9 shrink-0 flex items-center gap-1 pl-5 pr-2 border-b border-steel bg-slate-dark text-[11px] font-mono">
      <span class="text-gear-gold font-bold px-1 select-none">&gt;_</span>
      <div class="flex-1 min-w-0 flex items-center gap-1 overflow-x-auto">
        <div
          v-for="tab in tabs"
          :key="tab.key"
          @click="activate(tab.key)"
          class="group/tab flex items-center gap-1.5 pl-2.5 pr-1 py-1 rounded cursor-pointer border shrink-0"
          :class="tab.key === activeKey ? 'bg-slate-inner border-steel text-zinc-200' : 'border-transparent text-zinc-400 hover:text-zinc-200'"
        >
          <span v-if="tab.unread && tab.key !== activeKey" class="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
          <span :class="tab.exited ? 'line-through text-zinc-500' : ''">{{ tab.title || 'shell' }}</span>
          <button
            @click.stop="closeTab(tab.key)"
            class="w-4 h-4 flex items-center justify-center rounded text-zinc-500 hover:text-zinc-200 hover:bg-slate-panel"
            title="Close terminal (ends the shell)"
          >×</button>
        </div>
        <button @click="newTab()" class="w-6 h-6 shrink-0 flex items-center justify-center rounded text-zinc-400 hover:text-gear-gold hover:bg-slate-inner" title="New terminal">+</button>
      </div>
      <button @click="toggleMaximize" class="w-6 h-6 flex items-center justify-center rounded text-zinc-400 hover:text-zinc-200 hover:bg-slate-inner" :title="maximized ? 'Restore size' : 'Maximize'">{{ maximized ? '❐' : '□' }}</button>
      <button @click="minimize" class="w-6 h-6 flex items-center justify-center rounded text-zinc-400 hover:text-zinc-200 hover:bg-slate-inner" title="Minimize (shells keep running)">▁</button>
    </div>

    <!-- One xterm per tab; only the active one is visible -->
    <div class="relative flex-1 min-h-0 bg-slate-inner">
      <div
        v-for="tab in tabs"
        :key="tab.key"
        v-show="tab.key === activeKey"
        :ref="(el) => mountTerm(tab, el)"
        class="absolute inset-0 py-1 pl-2"
      ></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, markRaw } from 'vue'

const SIZE_KEY = 'rm-terminal-size'
const DEFAULT_SIZE = { w: 640, h: 380 }
const MIN_SIZE = { w: 360, h: 200 }
// Gap kept to the viewport edges, plus room for the bubble under the window
const EDGE = 20
const BUBBLE_SPACE = 80

const open = ref(false)
const tabs = ref([])
const activeKey = ref(null)
const size = ref({ ...DEFAULT_SIZE })
const maximized = ref(false)
const isResizing = ref(false)
const { theme } = useTheme()

const windowStyle = computed(() =>
  maximized.value
    ? { top: `${EDGE}px`, left: `${EDGE}px`, right: `${EDGE}px`, bottom: `${BUBBLE_SPACE}px` }
    : { right: `${EDGE}px`, bottom: `${BUBBLE_SPACE}px`, width: `${size.value.w}px`, height: `${size.value.h}px` }
)

const hasUnread = computed(() => tabs.value.some((t) => t.unread))

let xtermModules = null
async function loadXterm() {
  if (!xtermModules) {
    const [{ Terminal }, { FitAddon }] = await Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')])
    xtermModules = { Terminal, FitAddon }
  }
  return xtermModules
}

// Terminal colors follow the app's light/dark tokens
function termTheme() {
  const css = getComputedStyle(document.documentElement)
  const rgb = (name) => `rgb(${css.getPropertyValue(`--c-${name}`).trim().split(/\s+/).join(',')})`
  return {
    background: rgb('slate-inner'),
    foreground: rgb('parchment'),
    cursor: rgb('gear-gold'),
    cursorAccent: rgb('slate-inner'),
    selectionBackground: rgb('steel')
  }
}

let keySeq = 0
function newTab(sessionId = null, title = '') {
  const tab = {
    key: ++keySeq,
    id: sessionId,
    title,
    exited: false,
    unread: false,
    term: null,
    fit: null,
    ws: null,
    el: null,
    observer: null
  }
  tabs.value.push(tab)
  activeKey.value = tab.key
  open.value = true
  return tab
}

// Called by the template ref once the tab's container exists
async function mountTerm(tab, el) {
  if (!el || tab.el) return
  tab.el = el
  const { Terminal, FitAddon } = await loadXterm()

  const term = markRaw(new Terminal({
    fontFamily: '"IBM Plex Mono", monospace',
    fontSize: 12,
    cursorBlink: true,
    scrollback: 5000,
    theme: termTheme()
  }))
  const fit = markRaw(new FitAddon())
  term.loadAddon(fit)
  term.open(el)
  tab.term = term
  tab.fit = fit
  safeFit(tab)

  term.onData((d) => send(tab, { t: 'i', d }))
  term.onResize(({ cols, rows }) => send(tab, { t: 'r', c: cols, r: rows }))

  tab.observer = new ResizeObserver(() => safeFit(tab))
  tab.observer.observe(el)

  connect(tab)
  if (tab.key === activeKey.value) term.focus()
}

function safeFit(tab) {
  // A hidden (v-show) container has no size; fitting it would collapse the grid
  if (tab.fit && tab.el && tab.el.offsetWidth > 0 && tab.el.offsetHeight > 0) {
    try { tab.fit.fit() } catch {}
  }
}

function connect(tab) {
  const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/_terminal`)
  tab.ws = markRaw(ws)
  ws.onopen = () => {
    ws.send(JSON.stringify({ t: 'attach', id: tab.id, cols: tab.term.cols, rows: tab.term.rows }))
  }
  ws.onmessage = (ev) => {
    let msg
    try { msg = JSON.parse(ev.data) } catch { return }
    if (msg.t === 'ready') {
      tab.id = msg.id
      tab.title = msg.title
      if (msg.replay) tab.term.write(msg.replay)
    } else if (msg.t === 'o') {
      tab.term.write(msg.d)
      if (!open.value || tab.key !== activeKey.value) tab.unread = true
    } else if (msg.t === 'x') {
      tab.exited = true
      tab.term.write(`\r\n\x1b[2m[process exited${msg.code != null ? ` with code ${msg.code}` : ''}]\x1b[0m\r\n`)
    } else if (msg.t === 'error') {
      tab.term.write(`\r\n\x1b[31m${msg.message}\x1b[0m\r\n`)
    }
  }
  ws.onclose = () => {
    if (!tab.exited && tabs.value.includes(tab)) {
      tab.term?.write('\r\n\x1b[2m[disconnected from terminal server]\x1b[0m\r\n')
    }
  }
}

function send(tab, msg) {
  if (tab.ws && tab.ws.readyState === WebSocket.OPEN) tab.ws.send(JSON.stringify(msg))
}

function activate(key) {
  activeKey.value = key
  const tab = tabs.value.find((t) => t.key === key)
  if (!tab) return
  tab.unread = false
  nextTick(() => {
    safeFit(tab)
    tab.term?.focus()
  })
}

function closeTab(key) {
  const i = tabs.value.findIndex((t) => t.key === key)
  if (i < 0) return
  const [tab] = tabs.value.splice(i, 1)
  send(tab, { t: 'kill' })
  tab.observer?.disconnect()
  tab.ws?.close()
  tab.term?.dispose()
  if (activeKey.value === key) {
    const next = tabs.value[Math.min(i, tabs.value.length - 1)]
    if (next) activate(next.key)
    else {
      activeKey.value = null
      open.value = false
    }
  }
}

function openPanel() {
  open.value = true
  if (tabs.value.length === 0) newTab()
  else activate(activeKey.value ?? tabs.value[0].key)
}

function minimize() {
  open.value = false
}

// Window size, remembered per browser and kept inside the viewport
function clampSize(w, h) {
  return {
    w: Math.round(Math.min(window.innerWidth - 2 * EDGE, Math.max(MIN_SIZE.w, w))),
    h: Math.round(Math.min(window.innerHeight - BUBBLE_SPACE - EDGE, Math.max(MIN_SIZE.h, h)))
  }
}
function setSize(w, h) {
  size.value = clampSize(w, h)
  try { localStorage.setItem(SIZE_KEY, JSON.stringify(size.value)) } catch {}
}
function toggleMaximize() {
  maximized.value = !maximized.value
  const tab = tabs.value.find((t) => t.key === activeKey.value)
  nextTick(() => tab?.term?.focus())
}
function startResize(e) {
  e.preventDefault()
  const startX = e.clientX
  const startY = e.clientY
  const start = { ...size.value }
  isResizing.value = true
  document.body.style.cursor = 'nwse-resize'
  document.body.style.userSelect = 'none'
  const onMove = (ev) => { size.value = clampSize(start.w + (startX - ev.clientX), start.h + (startY - ev.clientY)) }
  const onUp = () => {
    isResizing.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    setSize(size.value.w, size.value.h)
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}
const onWindowResize = () => { size.value = clampSize(size.value.w, size.value.h) }

watch(theme, () => nextTick(() => {
  const t = termTheme()
  for (const tab of tabs.value) if (tab.term) tab.term.options.theme = t
}))

// Ctrl+` toggles the panel, like an editor's integrated terminal
function onKey(e) {
  if (e.ctrlKey && e.key === '`') {
    e.preventDefault()
    open.value ? minimize() : openPanel()
  }
}

onMounted(async () => {
  try {
    const saved = JSON.parse(localStorage.getItem(SIZE_KEY) || 'null')
    if (saved?.w && saved?.h) size.value = clampSize(saved.w, saved.h)
  } catch {}
  window.addEventListener('keydown', onKey)
  window.addEventListener('resize', onWindowResize)

  // Reattach to shells that survived a page refresh, without popping the panel open
  try {
    const res = await fetch('/api/terminal')
    const data = await res.json()
    for (const s of data.sessions || []) {
      if (!s.exited) newTab(s.id, s.title)
    }
    open.value = false
  } catch {}
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onWindowResize)
  for (const tab of tabs.value) {
    tab.observer?.disconnect()
    tab.ws?.close()
    tab.term?.dispose()
  }
})
</script>
