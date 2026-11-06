import fs from 'node:fs'
import path from 'node:path'
import { resolveProjectDir, runCommand } from '../utils/ringmaster'

// Task branch history for the Branches view: every rm/Txx branch, the commits
// each automaton made on it (from Ringmaster-* trailers), where it forked and
// when it merged into main. ?commit=<sha> returns one commit's message and files.

const MAX_COMMITS = 400
const FS = '\x1f'
const RS = '\x1e'

const git = (cwd: string, args: string[]) =>
  runCommand('git', ['-C', cwd, ...args], { timeout: 15000 }).then((r) => r.stdout)

export default defineEventHandler(async (event) => {
  const projectDir = resolveProjectDir()
  const query = getQuery(event)

  if (query.commit) {
    const sha = String(query.commit)
    if (!/^[0-9a-f]{7,40}$/.test(sha)) return { ok: false, error: 'Invalid commit' }
    try {
      const [message, numstat] = await Promise.all([
        git(projectDir, ['show', '-s', '--format=%B', sha]),
        // Merge commits are diffed against main's previous state
        git(projectDir, ['show', '-m', '--first-parent', '--numstat', '--format=', sha])
      ])
      const files = numstat.split('\n').filter(Boolean).map((l) => {
        const [add, del, ...p] = l.split('\t')
        return { path: p.join('\t'), add: add === '-' ? null : Number(add), del: del === '-' ? null : Number(del) }
      })
      return { ok: true, message: message.trim(), files }
    } catch (err: any) {
      return { ok: false, error: err.message }
    }
  }

  // Branch refs
  let refsOut = ''
  try {
    refsOut = await git(projectDir, ['for-each-ref', '--format=%(refname:short)' + FS + '%(objectname)', 'refs/heads/rm/'])
  } catch (err: any) {
    return { ok: false, error: `Not a git repository: ${err.message}` }
  }
  const tips: Record<string, string> = {}
  for (const line of refsOut.split('\n').filter(Boolean)) {
    const [name, sha] = line.split(FS)
    tips[name] = sha
  }

  let mainRef = 'main'
  try {
    await git(projectDir, ['rev-parse', '--verify', '--quiet', 'refs/heads/main'])
  } catch {
    mainRef = 'HEAD'
  }

  const trailer = (key: string) => `%(trailers:key=${key},valueonly,separator=%x2C)`
  const format = ['%H', '%P', '%aI', '%s', trailer('Ringmaster-Task'), trailer('Ringmaster-Automaton'), trailer('Ringmaster-Attempt')].join('%x1f') + '%x1e'
  let logOut = ''
  try {
    logOut = await git(projectDir, ['log', '--date-order', `-n${MAX_COMMITS + 1}`, `--format=${format}`, mainRef, ...Object.keys(tips)])
  } catch (err: any) {
    return { ok: false, error: err.message }
  }

  const commits = logOut.split(RS).map((r) => r.replace(/^\n/, '')).filter(Boolean).map((rec) => {
    const [sha, parents, time, subject, task, automaton, attempt] = rec.split(FS)
    return {
      sha,
      parents: parents ? parents.split(' ') : [],
      time,
      subject,
      task: task.trim() || null,
      automaton: automaton.trim() || null,
      attempt: attempt.trim() ? Number(attempt.trim()) : null,
      branch: null as string | null,
      mergeOf: null as string | null
    }
  })
  const truncated = commits.length > MAX_COMMITS
  if (truncated) commits.length = MAX_COMMITS
  const bySha = new Map(commits.map((c) => [c.sha, c]))

  // main's own line: its first-parent chain
  const mainLine = new Set<string>()
  try {
    const tip = (await git(projectDir, ['rev-parse', mainRef])).trim()
    for (let c = bySha.get(tip); c; c = bySha.get(c.parents[0])) mainLine.add(c.sha)
  } catch {}

  // A branch's chain is its first-parent walk down to main's line. A branch
  // forked from another task branch also walks through that branch's commits,
  // so each commit belongs to the shortest chain that contains it.
  const chains: Record<string, string[]> = {}
  for (const [name, tip] of Object.entries(tips)) {
    const chain: string[] = []
    for (let c = bySha.get(tip); c && !mainLine.has(c.sha); c = bySha.get(c.parents[0])) chain.push(c.sha)
    chains[name] = chain
  }
  const owner = new Map<string, string>()
  for (const [name, chain] of Object.entries(chains).sort((a, b) => b[1].length - a[1].length)) {
    for (const sha of chain) owner.set(sha, name)
  }
  for (const c of commits) {
    if (mainLine.has(c.sha)) {
      c.branch = 'main'
      if (c.parents.length > 1) c.mergeOf = owner.get(c.parents[1]) || null
    } else {
      c.branch = owner.get(c.sha) || null
    }
  }

  // Task titles, workers and statuses from the plan and state
  const readJson = (file: string) => {
    try {
      return JSON.parse(fs.readFileSync(file, 'utf-8'))
    } catch {
      return null
    }
  }
  const plan = readJson(path.join(projectDir, '.ringmaster', 'plan.json'))
  const state = readJson(path.join(projectDir, '.ringmaster', 'state.json'))

  const branches = Object.keys(tips).map((name) => {
    const own = commits.filter((c) => c.branch === name) // newest first
    const oldest = own[own.length - 1]
    const merge = commits.find((c) => c.mergeOf === name) || null
    const taskId = name.replace(/^rm\//, '')
    const planTask = plan?.tasks?.find((t: any) => t.id === taskId)
    const ts = state?.task_states?.[taskId]
    const automatons = [...new Set(own.slice().reverse().map((c) => c.automaton).filter(Boolean))]
    if (!automatons.length && ts?.worker) automatons.push(ts.worker)
    return {
      name,
      task: taskId,
      title: planTask?.title || '',
      tip: tips[name],
      fork: oldest?.parents[0] || null,
      merge: merge ? { sha: merge.sha, time: merge.time } : null,
      firstCommit: oldest?.time || null,
      commits: own.length,
      attempts: Math.max(0, ...own.map((c) => c.attempt || 0)) || ts?.attempt || 0,
      automatons,
      status: merge ? 'MERGED' : (ts?.status || 'OPEN')
    }
  })

  return { ok: true, main: mainRef, truncated, commits, branches }
})
