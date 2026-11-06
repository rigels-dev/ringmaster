<template>
  <div class="flex h-screen w-screen overflow-hidden bg-slate-deep text-parchment font-mono">
    <!-- Left Navigation Sidemenu -->
    <aside class="w-64 flex flex-col bg-slate-dark border-r border-steel z-20">
      <!-- Brand Header with Kinematic Rotating Cog Icon -->
      <div class="h-14 flex items-center gap-3 px-4 border-b border-steel bg-slate-deep">
        <RingmasterEmblem :status="currentKinematicStatus" size="sm" class="shrink-0" />
        <div>
          <h1 class="font-cinzel font-black tracking-wider text-gear-gold text-sm flex items-center gap-1.5">
            <span>RINGMASTER</span>
          </h1>
          <p class="text-[9px] text-zinc-400 tracking-tight font-outfit uppercase">Agent Control Plane</p>
        </div>
        <button
          @click="toggleTheme"
          class="ml-auto w-7 h-7 flex items-center justify-center rounded border border-steel text-zinc-400 hover:text-gear-gold hover:border-gear-dark transition"
          :title="theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme'"
          :aria-label="theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme'"
        >
          <svg v-if="theme === 'light'" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
          </svg>
          <svg v-else class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
          </svg>
        </button>
      </div>

      <!-- Navigation Links -->
      <nav class="flex-1 p-3 space-y-1.5 overflow-y-auto">
        <button
          v-for="item in navItems"
          :key="item.id"
          @click="setView(item.id)"
          class="w-full flex items-center justify-between px-3 py-2.5 rounded text-xs font-semibold transition-all"
          :class="activeView === item.id 
            ? 'bg-slate-panel text-gear-gold border border-gear-dark/40 shadow-sm shadow-gear-dark/20' 
            : 'text-zinc-400 hover:text-zinc-200 hover:bg-slate-panel/50'"
        >
          <div class="flex items-center gap-3">
            <span class="text-base">{{ item.icon }}</span>
            <span>{{ item.label }}</span>
          </div>
          <span v-if="item.badge && !project.isBlank" class="px-1.5 py-0.5 text-[9px] rounded font-bold" :class="item.badgeClass">
            {{ item.badge }}
          </span>
        </button>
      </nav>

      <!-- Working Automatons (visible from every view while a run is live) -->
      <div v-if="isRunActive" class="px-3 py-2.5 border-t border-steel bg-slate-dark">
        <div class="flex items-center justify-between mb-2">
          <span class="text-[10px] uppercase font-bold text-zinc-400">Working Automatons</span>
          <button @click="setView('logs')" class="text-[9px] text-cyan-energy hover:text-cyan-300 transition">View Logs →</button>
        </div>
        <div v-if="activeWorkersList.length > 0" class="space-y-1.5 max-h-48 overflow-y-auto">
          <div
            v-for="worker in activeWorkersList"
            :key="worker.taskId"
            class="p-2 rounded bg-slate-panel border border-steel/80"
            :title="`${worker.taskId}: ${worker.taskTitle}`"
          >
            <div class="flex items-center gap-1.5">
              <span class="w-1.5 h-1.5 rounded-full bg-green-400 animate-ping shrink-0"></span>
              <span class="text-[10px] font-bold text-zinc-200 truncate">{{ worker.name }}</span>
              <span class="ml-auto text-[9px] text-cyan-energy font-mono shrink-0">{{ worker.taskId }}</span>
            </div>
            <p class="text-[9px] text-zinc-400 truncate mt-0.5">{{ worker.taskTitle }} · attempt {{ worker.attempt }}</p>
          </div>
        </div>
        <p v-else class="text-[9px] text-zinc-500">Engine is running — waiting for the next task dispatch…</p>
      </div>

      <!-- Active Project Info & Path (Dynamic!) -->
      <div class="p-3 border-t border-steel bg-slate-inner/80">
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Project Name</span>
          <span class="text-gear-gold font-bold truncate max-w-[120px]" :title="project.name">{{ project.name || 'Loading...' }}</span>
        </div>
        <div class="text-[9px] text-zinc-500 mb-2 truncate" :title="project.path">
          <span class="text-zinc-400">Path:</span> <code class="text-zinc-400">{{ project.path || '-' }}</code>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400 mb-1">
          <span>Master Planner</span>
          <span class="text-cyan-energy font-semibold">{{ project.master || 'default' }}</span>
        </div>
        <div class="flex items-center justify-between text-[10px] text-zinc-400">
          <span>Active Automatons</span>
          <span :class="activeWorkersList.length > 0 ? 'text-green-400 font-bold' : 'text-zinc-500'">
            {{ activeWorkersList.length > 0 ? activeWorkersList.length + ' Running' : '0 Idle' }}
          </span>
        </div>
        <div class="mt-2 pt-2 border-t border-steel/60 text-[9px] text-zinc-500 font-mono" :title="`Ringmaster ${project.version || 'dev'}`">
          Ringmaster {{ project.version || 'dev' }}
        </div>
      </div>
    </aside>

    <!-- Global Floating Toast Notification System (Replaces browser alert) -->
    <div class="fixed top-5 right-5 z-50 flex flex-col gap-2 pointer-events-none max-w-sm">
      <div 
        v-for="toast in toastList" 
        :key="toast.id"
        class="p-4 rounded-xl shadow-2xl border backdrop-blur-md font-mono text-xs pointer-events-auto transition-all duration-300"
        :class="{
          'bg-slate-dark/95 border-green-500/50 text-parchment shadow-green-500/10': toast.type === 'success',
          'bg-slate-dark/95 border-red-500/50 text-parchment shadow-red-500/10': toast.type === 'error',
          'bg-slate-dark/95 border-gear-gold/50 text-parchment shadow-gear-gold/10': toast.type === 'info'
        }"
      >
        <div class="flex items-start gap-3">
          <span class="text-base" v-if="toast.type === 'success'">✓</span>
          <span class="text-base" v-else-if="toast.type === 'error'">✗</span>
          <span class="text-base" v-else>ℹ</span>
          <div class="flex-1">
            <h4 class="font-bold text-xs mb-1" :class="toast.type === 'success' ? 'text-green-400' : (toast.type === 'error' ? 'text-red-400' : 'text-gear-gold')">
              {{ toast.title }}
            </h4>
            <p class="text-[11px] text-zinc-300 leading-relaxed whitespace-pre-wrap">{{ toast.message }}</p>
            <div v-if="toast.meta" class="mt-2 pt-2 border-t border-steel/60 flex items-center justify-between text-[10px] text-zinc-500">
              <span>Latency: <strong class="text-cyan-energy">{{ toast.meta.latency }}ms</strong></span>
              <span v-if="toast.meta.tokens">Tokens: <strong class="text-zinc-300">{{ toast.meta.tokens }}</strong></span>
            </div>
          </div>
          <button @click="dismissToast(toast.id)" class="text-zinc-500 hover:text-zinc-300 text-xs">✕</button>
        </div>
      </div>
    </div>

    <!-- Steampunk Custom Confirmation Dialog (Replaces browser confirm) -->
    <div v-if="confirmDialog.show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-dark border border-gear-gold/60 p-5 rounded-xl max-w-sm w-full shadow-2xl flex flex-col gap-3 font-mono">
        <div class="flex items-center gap-2 text-gear-gold">
          <span class="text-lg">⚠</span>
          <h3 class="font-bold text-sm">{{ confirmDialog.title }}</h3>
        </div>
        <p class="text-xs text-zinc-300 leading-relaxed">{{ confirmDialog.message }}</p>
        <div class="flex items-center justify-end gap-2.5 pt-2 border-t border-steel/60">
          <button 
            @click="confirmDialog.show = false" 
            class="px-3.5 py-1.5 rounded text-xs text-zinc-400 hover:text-zinc-200 border border-steel bg-slate-panel"
          >
            Cancel
          </button>
          <button 
            @click="executeConfirmDialog" 
            class="px-4 py-1.5 rounded text-xs font-bold bg-red-950 text-red-400 hover:bg-red-900 border border-red-800 shadow"
          >
            Confirm Delete
          </button>
        </div>
      </div>
    </div>

    <!-- Main Content Workspace Area -->
    <main class="flex-1 flex flex-col h-full bg-slate-deep overflow-hidden">
      <ChatView v-if="activeView === 'chat'" />
      <TasksView v-else-if="activeView === 'tasks'" />
      <ProvidersView v-else-if="activeView === 'providers'" />
      <LogsView v-else-if="activeView === 'logs'" />
      <BranchesView v-else-if="activeView === 'branches'" />
      <AnalyticsView v-else-if="activeView === 'analytics'" />
    </main>

    <!-- Integrated terminal (floating window behind a bottom-right bubble) -->
    <ClientOnly>
      <TerminalDock />
    </ClientOnly>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'

const {
  activeView,
  setView,
  project,
  toastList,
  confirmDialog,
  executeConfirmDialog,
  dismissToast,
  navItems,
  isThinking,
  currentKinematicStatus,
  tasks,
  activeWorkersList,
  isGeneratingPlan,
  isRunActive,
  start,
  stop
} = useRingmaster()

const { theme, toggleTheme } = useTheme()

onMounted(start)
onUnmounted(stop)
</script>
