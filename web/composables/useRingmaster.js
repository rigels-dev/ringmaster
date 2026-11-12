import { statusLabel } from '~/utils/status'

import { ref, computed, watch } from 'vue'

// Shared Ringmaster UI state. One instance per Nuxt app, so every view
// component sees the same refs; app.vue calls start()/stop() on mount/unmount.
export function useRingmaster() {
  const nuxtApp = useNuxtApp()
  if (!nuxtApp._ringmaster) nuxtApp._ringmaster = createRingmaster()
  return nuxtApp._ringmaster
}

function createRingmaster() {
  const activeView = ref('chat')
  const selectedTarget = ref('master')
  const inputMsg = ref('')

  // Synchronize active view with URL hash (/tasks, /providers, /analytics, /chat)
  function syncViewFromHash() {
    if (typeof window === 'undefined') return
    const hash = window.location.hash.replace(/^#\/?/, '').toLowerCase()
    if (['chat', 'tasks', 'branches', 'providers', 'skills', 'logs', 'analytics'].includes(hash)) {
      activeView.value = hash
    }
  }

  function setView(viewId) {
    activeView.value = viewId
    if (typeof window !== 'undefined') {
      window.location.hash = `#/${viewId}`
    }
  }

  watch(activeView, (newVal) => {
    if (typeof window !== 'undefined' && newVal) {
      if (window.location.hash.replace(/^#\/?/, '').toLowerCase() !== newVal) {
        window.location.hash = `#/${newVal}`
      }
    }
  })

  const project = ref({
    name: '',
    path: '',
    isBlank: true,
    brief: '',
    plan: null,
    state: null,
    master: '',
    workers: []
  })

  const providersData = ref({
    configPath: '',
    providers: [],
    agents: []
  })

  const skillsData = ref({
    skills: [],
    globalDir: '',
    projectDir: '',
    error: ''
  })
  const skillBusy = ref('')

  const newProv = ref({
    id: '',
    type: 'openai',
    base_url: '',
    api_key_env: ''
  })

  const isProviderFormValid = computed(() => {
    if (!newProv.value.id.trim()) return false
    if (newProv.value.type === 'claude_subscription') return true
    return Boolean(newProv.value.base_url.trim() && newProv.value.api_key_env.trim())
  })

  const pingStatus = ref(null)
  const isPinging = ref(false)
  const showApiKey = ref(false)
  const showAddAgentModal = ref(false)
  const isLoadingModels = ref(false)
  const fetchedModelList = ref([])
  const agentPingStates = ref({})
  const isPingingNewAgent = ref(false)
  const toastList = ref([])

  const confirmDialog = ref({
    show: false,
    title: '',
    message: '',
    confirmLabel: '',
    action: null
  })

  function executeConfirmDialog() {
    if (confirmDialog.value.action) {
      confirmDialog.value.action()
    }
    confirmDialog.value.show = false
  }

  function showToast(title, message, type = 'success', meta = null) {
    const id = Date.now() + Math.random()
    toastList.value.push({ id, title, message, type, meta })
    setTimeout(() => {
      dismissToast(id)
    }, 5000)
  }

  function dismissToast(id) {
    toastList.value = toastList.value.filter(t => t.id !== id)
  }

  const newAgent = ref({
    id: '',
    provider: '',
    model: '',
    role: 'worker'
  })

  async function fetchProviderModels() {
    if (!newAgent.value.provider) return
    isLoadingModels.value = true
    fetchedModelList.value = []
    try {
      const res = await fetch(`/api/models?provider=${encodeURIComponent(newAgent.value.provider)}`)
      const data = await res.json()
      if (data.ok && Array.isArray(data.models)) {
        fetchedModelList.value = data.models
        if (data.models.length > 0) {
          newAgent.value.model = data.models[0].id
        }
      }
    } catch (err) {
      console.error('Failed to fetch provider models:', err)
    } finally {
      isLoadingModels.value = false
    }
  }

  function maskApiKey(val) {
    if (!val) return '—'
    if (val.startsWith('sk-') || val.length > 15) {
      return '••••••••••••••••••••'
    }
    return val
  }

  const navItems = computed(() => [
    { 
      id: 'chat', 
      label: 'Chat', 
      icon: '💬', 
      badge: unreviewedAlertsCount.value > 0 ? `${unreviewedAlertsCount.value} Alert` : null, 
      badgeClass: 'bg-amber-950 text-amber-400 border border-amber-800' 
    },
    { 
      id: 'tasks', 
      label: 'Tasks', 
      icon: '📋',
      badge: needsYouCount.value > 0
        ? `🙋 Needs you (${needsYouCount.value})`
        : tasks.value.length > 0 && !planApprovalState.value.isApproved ? 'Needs Approval' : null,
      badgeClass: needsYouCount.value > 0
        ? 'bg-rose-950 text-rose-300 border border-rose-700 animate-pulse'
        : 'bg-amber-950 text-amber-300 border border-amber-700'
    },
    { id: 'branches', label: 'Branches', icon: '🌿' },
    { id: 'providers', label: 'Providers', icon: '🤖' },
    { id: 'skills', label: 'Skills', icon: '🧩' },
    { id: 'logs', label: 'Logs', icon: '📜', badge: isRunActive.value ? 'Live' : null, badgeClass: 'bg-cyan-950 text-cyan-300 border border-cyan-800' },
    { id: 'analytics', label: 'Analytics', icon: '📈' }
  ])

  const unreviewedAlertsCount = computed(() => {
    return messages.value.filter(m => m.actionable).length
  })

  const hasMasterPlanner = computed(() => {
    return project.value.master && project.value.master !== 'none' && project.value.master !== ''
  })

  const messages = ref([])
  // Set once saved chat history is loaded; alerts must not be saved over it before then
  const chatLoaded = ref(false)
  const isThinking = ref(false)
  const chatContainerRef = ref(null)
  const canGeneratePlan = ref(false)

  const currentKinematicStatus = computed(() => {
    if (isRunActive.value) {
      return 'overdrive' // Rapid gear & ring spin when automatons are active in worktrees!
    }
    if (isExecutingPlan.value) {
      return 'overdrive'
    }
    if (isGeneratingPlan.value || isThinking.value) {
      return 'dag' // Fast deliberate planning spin
    }
    if (activeWorkersList.value && activeWorkersList.value.length > 0) {
      return 'execution' // Active puppet control & execution
    }
    return 'idle' // Calm gentle rotation
  })

  function scrollToBottom() {
    nextTick(() => {
      setTimeout(() => {
        if (chatContainerRef.value) {
          chatContainerRef.value.scrollTop = chatContainerRef.value.scrollHeight + 1000
        }
      }, 50)
    })
  }

  async function loadChatHistory() {
    try {
      const res = await fetch('/api/chat')
      const data = await res.json()
      if (data.ok && Array.isArray(data.messages) && data.messages.length > 0) {
        messages.value = data.messages
        // Check if previous discussion already reached sufficient readiness
        const hasPlanRecommendation = data.messages.some(m => 
          m.role === 'master' && (
            m.text.includes('Generate Plan') || 
            m.text.includes('Rencana Proyek') || 
            m.text.includes('Pembagian Task (DAG)') ||
            m.text.includes('Rencana Pembagian Task') ||
            m.text.includes('Rencana Eksekusi') ||
            m.text.includes('Task Breakdown') ||
            m.text.includes('Rencana ini sudah actionable')
          )
        )
        if (hasPlanRecommendation) {
          canGeneratePlan.value = true
        }
        return true
      }
    } catch {}
    return false
  }

  async function saveChatHistory(updateBrief = false, briefContent = '') {
    try {
      await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          messages: messages.value,
          updateBrief,
          briefContent
        })
      })
    } catch {}
  }

  async function refreshProject() {
    try {
      const res = await fetch('/api/project')
      project.value = await res.json()
    } catch {}
  }

  // Initial load only: polling must not reload the chat, or it would overwrite
  // messages that are not yet saved to the server.
  async function loadProjectData() {
    try {
      await refreshProject()
      const data = project.value

      // Check if there is saved chat history first
      const hasHistory = await loadChatHistory()
      chatLoaded.value = true

      // If blank project and no prior chat history, initialize welcoming chat
      if (!hasHistory && data.isBlank) {
        messages.value = [
          {
            role: 'master',
            targetLabel: 'You',
            text: `Project '${data.name}' is connected and has no tasks yet. Tell me what you want to build. I will ask what I need, then split the work into tasks.`,
            time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          }
        ]
        saveChatHistory()
      }
    } catch {}
  }

  async function loadProvidersData() {
    try {
      const res = await fetch('/api/providers')
      const data = await res.json()
      if (data.ok) {
        providersData.value = data
        if (data.providers && data.providers.length > 0 && !newAgent.value.provider) {
          newAgent.value.provider = data.providers[0].id
          fetchProviderModels()
        }
      }
    } catch {}
  }

  async function loadSkills() {
    try {
      const res = await fetch('/api/skills')
      const data = await res.json()
      skillsData.value = {
        skills: data.skills || [],
        globalDir: data.globalDir || '',
        projectDir: data.projectDir || '',
        error: data.ok ? '' : (data.error || 'Unknown error')
      }
    } catch (err) {
      skillsData.value = { ...skillsData.value, error: err.message }
    }
  }

  async function toggleSkill(skill) {
    skillBusy.value = skill.name
    try {
      const res = await fetch('/api/skills', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: skill.name, enabled: !skill.enabled })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Skill Updated', data.message, 'info')
      } else {
        showToast('Skill Not Changed', data.error, 'error')
      }
    } catch (err) {
      showToast('Skill Not Changed', err.message, 'error')
    } finally {
      skillBusy.value = ''
      await loadSkills()
    }
  }

  const tasks = computed(() => {
    if (!project.value.plan || !project.value.plan.tasks) return []
    return project.value.plan.tasks.map(t => {
      const ts = project.value.state?.task_states?.[t.id] || {}
      return {
        id: t.id,
        title: t.title,
        type: t.type || 'backend',
        desc: t.description || 'No description',
        worker: t.worker || 'worker',
        depends_on: t.depends_on || [],
        editsTests: Boolean(t.edits_tests),
        status: ts.status || 'PENDING',
        error: ts.error || '',
        requests: ts.requests || [],
        retries: `${ts.attempt || 0}/${t.max_retries || 2}`
      }
    })
  })

  // Statuses where a task can only move on after the user acts
  const NEEDS_YOU_STATUSES = ['AWAITING_INPUT', 'BLOCKED', 'NEEDS_REVIEW', 'PAUSED']

  const unansweredQuestions = computed(() =>
    (project.value.plan?.open_questions || []).filter(q => !(q.answer || '').trim())
  )

  const needsYouCount = computed(() =>
    tasks.value.filter(t => NEEDS_YOU_STATUSES.includes(t.status)).length + unansweredQuestions.value.length
  )

  // Task whose detail modal is open (shared so Chat can open it too)
  const openTaskId = ref(null)
  function openTaskDetail(id) {
    openTaskId.value = id
    setView('tasks')
  }

  // Post a chat alert when a task starts waiting on the user; clear it once it moves on
  watch([tasks, chatLoaded], ([list, loaded]) => {
    if (!loaded) return
    let changed = false
    for (const t of list) {
      const waiting = NEEDS_YOU_STATUSES.includes(t.status)
      const alertKey = `${t.id}:${t.status}:${t.error}`
      for (const m of messages.value) {
        if (m.alertTaskId === t.id && m.actionable && (!waiting || m.alertKey !== alertKey)) {
          m.actionable = false
          changed = true
        }
      }
      if (!waiting || messages.value.some(m => m.alertKey === alertKey)) continue
      const lines = t.requests.map(r => r.type === 'approval'
        ? `• [${r.id}] Approve command: ${r.command}${r.reason ? ` (${r.reason})` : ''}`
        : `• [${r.id}] ${r.question}${r.options?.length ? ` (${r.options.join(' / ')})` : ''}`)
      const reason = t.error.length > 400 ? `${t.error.slice(0, 400)}…` : t.error
      messages.value.push({
        role: 'worker',
        workerId: t.worker,
        taskId: t.id,
        taskBranch: `rm/${t.id}`,
        statusTitle: `🙋 ${statusLabel(t.status)}`,
        text: lines.length ? `${reason}\n\n${lines.join('\n')}` : reason,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        actionable: true,
        alertTaskId: t.id,
        alertKey
      })
      changed = true
    }
    if (changed) saveChatHistory()
  })

  async function answerTask(id, payload) {
    try {
      const res = await fetch('/api/task', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id, ...payload })
      })
      const data = await res.json()
      if (!data.ok) {
        showToast('Answer Failed', data.error, 'error')
        return false
      }
      if (data.resumeError) {
        showToast('Answer Saved', `Could not resume automatically: ${data.resumeError}`, 'error')
      } else {
        showToast('Answer Sent', `${id} is re-queued; the automaton continues with your answer.`, 'success')
      }
      await refreshProject()
      return true
    } catch (err) {
      showToast('Answer Failed', err.message, 'error')
      return false
    }
  }

  async function answerPlanQuestion(questionId, answer) {
    try {
      const res = await fetch('/api/plan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'answer_question', questionId, answer })
      })
      const data = await res.json()
      if (!data.ok) {
        showToast('Answer Failed', data.error, 'error')
        return
      }
      planApprovalState.value.isApproved = false
      await refreshProject()
    } catch (err) {
      showToast('Answer Failed', err.message, 'error')
    }
  }

  const activeWorkersList = computed(() => {
    // Only return active if run process is actually alive OR status is truly RUNNING
    if (!isRunActive.value || !project.value.state || !project.value.state.task_states) return []
    const planTasks = project.value.plan?.tasks || []
    const list = []
    for (const [id, ts] of Object.entries(project.value.state.task_states)) {
      if (ts.status === 'RUNNING') {
        list.push({
          id: ts.worker || 'worker',
          name: ts.worker || 'worker',
          taskId: id,
          taskTitle: planTasks.find(t => t.id === id)?.title || 'Active Task',
          status: ts.status,
          attempt: ts.attempt || 1,
          startAt: ts.start_at || null
        })
      }
    }
    return list
  })

  const totalTokensConsumed = computed(() => {
    if (!project.value.state) return '0'
    let total = 0
    const planning = project.value.state.planning_usage
    if (planning) {
      total += (planning.tokens_in || 0) + (planning.tokens_out || 0)
    }
    if (project.value.state.task_states) {
      for (const ts of Object.values(project.value.state.task_states)) {
        if (ts.usage) {
          total += (ts.usage.tokens_in || 0) + (ts.usage.tokens_out || 0)
        }
      }
    }
    return total.toLocaleString()
  })

  const completedTasksCount = computed(() => {
    return tasks.value.filter(t => t.status === 'DONE' || t.status === 'COMPLETED').length
  })

  async function saveNewAgent() {
    if (!newAgent.value.id.trim() || !newAgent.value.model.trim()) {
      showToast('Validation Error', 'Agent alias and model name are required', 'error')
      return
    }
    try {
      const res = await fetch('/api/providers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'add_agent',
          ...newAgent.value,
          tool_calling: newAgent.value.role === 'worker'
        })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Agent Added', `Agent '${newAgent.value.id}' (${newAgent.value.model}) is ready to use.`, 'success')
        showAddAgentModal.value = false
        newAgent.value.id = ''
        newAgent.value.model = ''
        loadProvidersData()
        refreshProject()
      } else {
        showToast('Registration Error', data.error, 'error')
      }
    } catch (err) {
      showToast('Failed to register agent', err.message, 'error')
    }
  }

  async function toggleAgentStatus(id) {
    try {
      const res = await fetch('/api/providers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'toggle_agent', id })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Agent Status Toggled', data.message, 'info')
        loadProvidersData()
        refreshProject()
      } else {
        showToast('Toggle Failed', data.error, 'error')
      }
    } catch (err) {
      showToast('Failed to toggle status', err.message, 'error')
    }
  }

  async function setAgentRole(id, role) {
    try {
      const res = await fetch('/api/providers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'set_role', id, role })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Role Updated', `Agent '${id}' is now ${role === 'master' ? 'the Master' : 'an automaton'}.`, 'info')
        loadProvidersData()
        refreshProject()
      } else {
        showToast('Update Failed', data.error, 'error')
      }
    } catch (err) {
      showToast('Role Switch Error', err.message, 'error')
    }
  }

  async function deleteAgent(id) {
    confirmDialog.value = {
      show: true,
      title: 'Delete agent',
      message: `Delete agent '${id}'?`,
      confirmLabel: 'Delete agent',
      action: async () => {
        try {
          const res = await fetch('/api/providers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'delete_agent', id })
          })
          const data = await res.json()
          if (data.ok) {
            showToast('Agent Removed', `Agent '${id}' was deleted.`, 'info')
            loadProvidersData()
            refreshProject()
          } else {
            showToast('Delete Failed', data.error, 'error')
          }
        } catch (err) {
          showToast('Failed to delete agent', err.message, 'error')
        }
      }
    }
  }

  function handleTypeChange() {
    if (newProv.value.type === 'claude_subscription') {
      newProv.value.base_url = ''
      newProv.value.api_key_env = ''
    }
  }

  async function testPing() {
    isPinging.value = true
    pingStatus.value = null
    try {
      const res = await fetch('/api/ping', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newProv.value)
      })
      const data = await res.json()
      if (data.ok) {
        pingStatus.value = { ok: true, message: `✓ Ping Succeeded (${data.latency_ms}ms) — ${data.message}` }
      } else {
        pingStatus.value = { ok: false, message: `✗ Ping Failed (${data.latency_ms || 0}ms): ${data.error}` }
      }
    } catch (err) {
      pingStatus.value = { ok: false, message: '✗ Network error: ' + err.message }
    } finally {
      isPinging.value = false
    }
  }

  async function pingAgent(a) {
    const p = providersData.value.providers?.find(prov => prov.id === a.provider)
    if (!p) {
      showToast('Provider Not Found', `Provider '${a.provider}' is not configured in ringmaster.toml`, 'error')
      return
    }

    agentPingStates.value[a.id] = { loading: true }
    try {
      const res = await fetch('/api/ping', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: p.type,
          base_url: p.base_url,
          api_key_env: p.api_key_env,
          model: a.model
        })
      })
      const data = await res.json()
      if (data.ok) {
        showToast(
          `Agent '${a.id}' Connected`,
          `Model ${a.model} successfully responded to chat request.`,
          'success',
          { latency: data.latency_ms, tokens: 1 }
        )
      } else {
        showToast(
          `Agent '${a.id}' Ping Failed`,
          data.error || 'Unknown error occurred while contacting model.',
          'error',
          { latency: data.latency_ms || 0 }
        )
      }
    } catch (err) {
      showToast(`Connection Error`, err.message, 'error')
    } finally {
      agentPingStates.value[a.id] = { loading: false }
    }
  }

  async function pingNewAgentModel() {
    if (!newAgent.value.provider || !newAgent.value.model) {
      showToast('Missing Fields', 'Please select a provider and model name first', 'error')
      return
    }

    const p = providersData.value.providers?.find(prov => prov.id === newAgent.value.provider)
    if (!p) {
      showToast('Provider Not Found', `Provider '${newAgent.value.provider}' is not configured`, 'error')
      return
    }

    isPingingNewAgent.value = true
    try {
      const res = await fetch('/api/ping', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: p.type,
          base_url: p.base_url,
          api_key_env: p.api_key_env,
          model: newAgent.value.model
        })
      })
      const data = await res.json()
      if (data.ok) {
        showToast(
          `Model Reachable (${data.latency_ms}ms)`,
          `Model '${newAgent.value.model}' responded successfully to live chat probe!`,
          'success',
          { latency: data.latency_ms, tokens: 1 }
        )
      } else {
        showToast(
          `Model Ping Failed (${data.latency_ms || 0}ms)`,
          data.error || 'Failed to contact model endpoint',
          'error',
          { latency: data.latency_ms || 0 }
        )
      }
    } catch (err) {
      showToast('Connection Error', err.message, 'error')
    } finally {
      isPingingNewAgent.value = false
    }
  }

  async function pingExisting(p) {
    try {
      const res = await fetch('/api/ping', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(p)
      })
      const data = await res.json()
      if (data.ok) {
        showToast(
          `Provider '${p.id}' Reachable`,
          data.message,
          'success',
          { latency: data.latency_ms }
        )
      } else {
        showToast(
          `Provider '${p.id}' Offline`,
          data.error,
          'error',
          { latency: data.latency_ms || 0 }
        )
      }
    } catch (err) {
      showToast(`Ping Error`, err.message, 'error')
    }
  }

  async function saveProvider() {
    if (!newProv.value.id.trim()) {
      showToast('Validation Error', 'Provider ID is required', 'error')
      return
    }
    try {
      const res = await fetch('/api/providers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'add_provider',
          ...newProv.value
        })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Provider Saved', data.message, 'success')
        newProv.value = { id: '', type: 'openai', base_url: '', api_key_env: '' }
        pingStatus.value = null
        loadProvidersData()
      } else {
        showToast('Save Failed', data.error, 'error')
      }
    } catch (err) {
      showToast('Failed to save provider', err.message, 'error')
    }
  }

  async function deleteProvider(id) {
    confirmDialog.value = {
      show: true,
      title: 'Delete provider',
      message: `Delete provider '${id}'? Agents that use it will stop working.`,
      confirmLabel: 'Delete provider',
      action: async () => {
        try {
          const res = await fetch('/api/providers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'delete_provider', id })
          })
          const data = await res.json()
          if (data.ok) {
            showToast('Provider Deleted', `Provider '${id}' was deleted.`, 'info')
            loadProvidersData()
          } else {
            showToast('Delete Failed', data.error, 'error')
          }
        } catch (err) {
          showToast('Delete Failed', err.message, 'error')
        }
      }
    }
  }

  const planApprovalState = ref({
    isApproved: false,
    approvedHash: ''
  })
  const isApproving = ref(false)

  const availableWorkers = computed(() => {
    if (!providersData.value?.agents) return []
    return providersData.value.agents.filter(a => !a.disabled && (a.role === 'worker' || a.id !== project.value.master))
  })

  async function checkPlanApproval() {
    try {
      const res = await fetch('/api/plan')
      const data = await res.json()
      if (data.ok) {
        planApprovalState.value = {
          isApproved: Boolean(data.isApproved),
          approvedHash: data.approvedHash || ''
        }
      }
    } catch {}
  }

  async function approvePlanAction() {
    if (isApproving.value) return
    isApproving.value = true
    try {
      const res = await fetch('/api/plan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'approve' })
      })
      const data = await res.json()
      if (data.ok) {
        planApprovalState.value = {
          isApproved: true,
          approvedHash: data.approvedHash || ''
        }
        showToast('Plan Approved', `You can run it now. Hash ${data.approvedHash?.slice(0, 16)}…`, 'success')
        await refreshProject()
      } else {
        showToast('Approval Failed', data.error, 'error')
      }
    } catch (err) {
      showToast('Approval Failed', err.message, 'error')
    } finally {
      isApproving.value = false
    }
  }

  async function updateTaskWorker(taskId, worker) {
    try {
      const res = await fetch('/api/plan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'update_worker', taskId, worker })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Automaton Changed', `Task ${taskId} now uses '${worker}'. The plan needs approval again.`, 'info')
        planApprovalState.value.isApproved = false
        await refreshProject()
      } else {
        showToast('Update Failed', data.error, 'error')
      }
    } catch (err) {
      showToast('Could Not Change Automaton', err.message, 'error')
    }
  }

  async function discardPlanAction() {
    confirmDialog.value = {
      show: true,
      title: 'Discard plan',
      message: 'Discard this plan? Its tasks are cleared, and you can plan again with the Master.',
      confirmLabel: 'Discard plan',
      action: async () => {
        try {
          const res = await fetch('/api/plan', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'reject' })
          })
          const data = await res.json()
          if (data.ok) {
            showToast('Plan Discarded', 'The plan was cleared.', 'info')
            planApprovalState.value.isApproved = false
            await refreshProject()
          }
        } catch (err) {
          showToast('Discard Failed', err.message, 'error')
        }
      }
    }
  }

  const isGeneratingPlan = ref(false)

  async function triggerGeneratePlan() {
    if (isGeneratingPlan.value) return
    isGeneratingPlan.value = true
    isThinking.value = true

    messages.value.push({
      role: 'user',
      targetLabel: 'Master',
      text: 'Generate the plan from this conversation.',
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    })
    scrollToBottom()

    showToast('Generating Plan', 'The Master is splitting the brief into tasks…', 'info')

    try {
      const res = await fetch('/api/plan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ force: true })
      })
      const data = await res.json()
      if (data.ok) {
        showToast('Plan Ready', 'Review it in Tasks.', 'success')
        messages.value.push({
          role: 'master',
          targetLabel: 'You',
          planReady: true,
          text: `The plan is ready: ${data.plan?.tasks?.length || 0} tasks.\n\nReview them in Tasks, change the automaton for any task, then approve.`,
          time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        })
        scrollToBottom()
        await saveChatHistory()
        await checkPlanApproval()
        await refreshProject()
      } else {
        showToast('Plan Generation Failed', data.error || 'Unknown error', 'error')
        messages.value.push({
          role: 'master',
          targetLabel: 'You',
          text: `The plan could not be generated: ${data.error}\n\n${data.output || ''}`,
          time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
          type: 'error'
        })
        scrollToBottom()
        await saveChatHistory()
      }
    } catch (err) {
      showToast('Network Error', err.message, 'error')
    } finally {
      isGeneratingPlan.value = false
      isThinking.value = false
      scrollToBottom()
    }
  }

  async function sendMessage() {
    if (!inputMsg.value.trim() || isThinking.value) return
    const text = inputMsg.value
    const target = selectedTarget.value

    messages.value.push({
      role: 'user',
      targetLabel: target === 'master' ? 'Master' : target,
      text: text,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    })
    inputMsg.value = ''
    isThinking.value = true
    saveChatHistory()

    if (target === 'master') {
      try {
        const res = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            action: 'discuss',
            prompt: text,
            history: messages.value
          })
        })
        const data = await res.json()

        // Update CAN_GENERATE readiness state from Master AI evaluation
        if (typeof data.canGenerate === 'boolean') {
          canGeneratePlan.value = data.canGenerate
        }

        if (data && data.reply) {
          messages.value.push({
            role: 'master',
            targetLabel: 'You',
            text: data.reply,
            time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
          })
          scrollToBottom()
          await saveChatHistory()
          await refreshProject()
        } else {
          messages.value.push({
            role: 'master',
            targetLabel: 'You',
            text: data.error ? `Error: ${data.error}` : `The Master received your message.`,
            time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
            type: 'error'
          })
          scrollToBottom()
          saveChatHistory()
        }
      } catch (err) {
        messages.value.push({
          role: 'master',
          targetLabel: 'You',
          text: `Could not reach the Master: ${err.message}`,
          time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
          type: 'error'
        })
        scrollToBottom()
        saveChatHistory()
      } finally {
        isThinking.value = false
        scrollToBottom()
      }
    } else {
      // Nothing carries a chat message to a running automaton yet, so say so
      // instead of showing an acknowledgement that never happened.
      isThinking.value = false
      messages.value.push({
        role: 'worker',
        workerId: target,
        taskId: 'not delivered',
        statusTitle: 'Message not delivered',
        taskBranch: `rm/${target}`,
        text: `A running automaton cannot receive chat messages yet. To steer ${target}, answer its question when it asks one, or stop the run and re-plan.`,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        type: 'error'
      })
      saveChatHistory()
    }
  }

  function instructWorker(msg, action) {
    if (action === 'answer') {
      openTaskDetail(msg.taskId)
    } else if (action === 'via_master') {
      selectedTarget.value = 'master'
      inputMsg.value = `Master, tolong bantu arahkan ${msg.workerId} di task ${msg.taskId}.`
    }
  }

  const isExecutingPlan = ref(false)
  const isRunActive = ref(false)
  const isRunExternal = ref(false)
  const runPid = ref(null)
  const runLogs = ref([])
  const runLogTimes = ref([])
  let runPollTimer = null

  const runConsoleRef = ref(null)
  const logsPageRef = ref(null)

  function logLineClass(log) {
    if (log.includes('error') || log.includes('stderr') || log.includes('❌')) return 'text-rose-400 font-semibold'
    if (log.includes('✓') || log.includes('★') || log.includes('passed')) return 'text-green-400 font-semibold'
    if (log.includes('⚡')) return 'text-cyan-300 font-semibold'
    return 'text-zinc-300'
  }

  async function pollRunStatus() {
    try {
      const res = await fetch('/api/run')
      const data = await res.json()
      if (data.ok) {
        isRunActive.value = data.isRunning
        isRunExternal.value = Boolean(data.external)
        runPid.value = data.pid || null
        const logs = data.logs || []
        if (logs.length !== runLogs.value.length || logs[logs.length - 1] !== runLogs.value[runLogs.value.length - 1]) {
          runLogs.value = logs
          runLogTimes.value = data.logTimes || []
          // The Logs page manages its own scrolling (it pauses when you scroll up)
          nextTick(() => {
            if (runConsoleRef.value) runConsoleRef.value.scrollTop = runConsoleRef.value.scrollHeight + 100
          })
        }
        if (data.projectState) {
          project.value.state = data.projectState
        }
      }
    } catch {}
  }

  async function handleExecuteOrSpur() {
    if (isRunActive.value) {
      showToast(
        'Run Active',
        'Tasks are running. See Logs.',
        'info'
      )
      return
    }

    isExecutingPlan.value = true
    try {
      const res = await fetch('/api/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'start', parallel: 4 })
      })
      const data = await res.json()
      if (data.ok) {
        isRunActive.value = true
        showToast(
          'Run Started',
          `Automatons are starting on the plan (PID ${data.pid || 'live'}).`,
          'success'
        )
        refreshProject()
        pollRunStatus()
      } else {
        showToast('Run Did Not Start', data.error || 'Unknown error', 'error')
      }
    } catch (err) {
      showToast('Run Did Not Start', err.message, 'error')
    } finally {
      isExecutingPlan.value = false
    }
  }

  async function clearRunLogs() {
    try {
      await fetch('/api/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'clear_logs' })
      })
      runLogs.value = []
      runLogTimes.value = []
    } catch (err) {
      showToast('Failed to clear logs', err.message, 'error')
    }
  }

  async function stopOrchestration() {
    try {
      const res = await fetch('/api/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'stop' })
      })
      const data = await res.json()
      if (data.ok) {
        isRunActive.value = false
        showToast('Stopping', data.message || 'Halted gracefully', 'info')
        refreshProject()
      }
    } catch (err) {
      showToast('Failed to stop', err.message, 'error')
    }
  }

  function start() {
    syncViewFromHash()
    if (typeof window !== 'undefined') {
      window.addEventListener('hashchange', syncViewFromHash)
    }
    loadProjectData()
    loadProvidersData()
    checkPlanApproval()
    pollRunStatus()

    // Background auto-refresh for live DAG updates
    runPollTimer = setInterval(() => {
      refreshProject()
      pollRunStatus()
    }, 2500)
  }

  function stop() {
    if (typeof window !== 'undefined') {
      window.removeEventListener('hashchange', syncViewFromHash)
    }
    clearInterval(runPollTimer)
  }

  return {
    activeView,
    selectedTarget,
    inputMsg,
    syncViewFromHash,
    setView,
    unansweredQuestions,
    openTaskId,
    openTaskDetail,
    needsYouCount,
    answerTask,
    answerPlanQuestion,
    project,
    providersData,
    skillsData,
    skillBusy,
    loadSkills,
    toggleSkill,
    newProv,
    isProviderFormValid,
    pingStatus,
    isPinging,
    showApiKey,
    showAddAgentModal,
    isLoadingModels,
    fetchedModelList,
    agentPingStates,
    isPingingNewAgent,
    toastList,
    confirmDialog,
    executeConfirmDialog,
    showToast,
    dismissToast,
    newAgent,
    fetchProviderModels,
    maskApiKey,
    navItems,
    unreviewedAlertsCount,
    hasMasterPlanner,
    messages,
    isThinking,
    chatContainerRef,
    canGeneratePlan,
    currentKinematicStatus,
    scrollToBottom,
    loadChatHistory,
    saveChatHistory,
    refreshProject,
    loadProjectData,
    loadProvidersData,
    tasks,
    activeWorkersList,
    totalTokensConsumed,
    completedTasksCount,
    saveNewAgent,
    toggleAgentStatus,
    setAgentRole,
    deleteAgent,
    handleTypeChange,
    testPing,
    pingAgent,
    pingNewAgentModel,
    pingExisting,
    saveProvider,
    deleteProvider,
    planApprovalState,
    isApproving,
    availableWorkers,
    checkPlanApproval,
    approvePlanAction,
    updateTaskWorker,
    discardPlanAction,
    isGeneratingPlan,
    triggerGeneratePlan,
    sendMessage,
    instructWorker,
    isExecutingPlan,
    isRunActive,
    isRunExternal,
    runPid,
    runLogs,
    runLogTimes,
    runConsoleRef,
    logsPageRef,
    logLineClass,
    pollRunStatus,
    handleExecuteOrSpur,
    clearRunLogs,
    stopOrchestration,
    start,
    stop
  }
}
