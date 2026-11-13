<template>
  <section class="flex-1 flex flex-col p-6 pb-24 overflow-y-auto">
    <div class="flex items-center justify-between mb-6 gap-4">
      <div>
        <h2 class="text-lg font-bold text-gear-gold font-cinzel">Skills</h2>
        <p class="text-body text-zinc-400">Instructions your automatons read when a task calls for them. An installed skill is on until you switch it off.</p>
      </div>
      <button
        @click="loadSkills"
        class="px-3 py-1.5 rounded text-xs text-zinc-300 bg-slate-panel border border-steel hover:border-gear-dark hover:text-gear-gold transition whitespace-nowrap"
      >
        ↻ Refresh
      </button>
    </div>

    <p v-if="skillsData.error" class="mb-4 p-3 rounded border border-red-800 bg-red-950/40 text-body text-red-300">
      Could not list skills: {{ skillsData.error }}
    </p>

    <div class="bg-slate-panel border border-steel rounded-lg overflow-hidden mb-6">
      <table class="w-full text-left text-body">
        <thead class="bg-slate-inner text-xs uppercase text-zinc-400">
          <tr>
            <th class="px-4 py-2.5 font-semibold">Skill</th>
            <th class="px-4 py-2.5 font-semibold">Description</th>
            <th class="px-4 py-2.5 font-semibold">Source</th>
            <th class="px-4 py-2.5 font-semibold text-right">Automatons get it</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in skillsData.skills" :key="s.name" class="border-t border-steel/60">
            <td class="px-4 py-3 font-bold whitespace-nowrap" :class="s.enabled ? 'text-parchment' : 'text-zinc-500'">{{ s.name }}</td>
            <td class="px-4 py-3 text-zinc-400 leading-relaxed">{{ s.description || '—' }}</td>
            <td class="px-4 py-3 whitespace-nowrap">
              <span class="px-1.5 py-0.5 rounded text-xs border" :class="s.source === 'project' ? 'text-cyan-energy border-cyan-800 bg-cyan-950/40' : 'text-zinc-400 border-steel bg-slate-inner'">
                {{ s.source === 'project' ? 'this project' : 'all projects' }}
              </span>
            </td>
            <td class="px-4 py-3 text-right whitespace-nowrap">
              <span v-if="s.disabled_by === 'project'" class="text-xs text-amber-300" title="Listed under [skills] disabled in this project's ringmaster.toml; edit that file to change it">
                Off in this project's ringmaster.toml
              </span>
              <button
                v-else
                @click="toggleSkill(s)"
                :disabled="skillBusy === s.name"
                role="switch"
                :aria-checked="s.enabled"
                :aria-label="`${s.enabled ? 'Turn off' : 'Turn on'} skill ${s.name}`"
                class="px-3 py-1 rounded text-xs font-bold border transition disabled:opacity-50"
                :class="s.enabled
                  ? 'bg-green-950/60 text-green-400 border-green-800 hover:bg-green-900/60'
                  : 'bg-slate-inner text-zinc-400 border-steel hover:text-zinc-200'"
              >
                {{ s.enabled ? '● On' : '○ Off' }}
              </button>
            </td>
          </tr>
          <tr v-if="!skillsData.skills.length && !skillsData.error">
            <td colspan="4" class="px-4 py-8 text-center text-zinc-500 italic">No skills installed yet.</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="bg-slate-panel border border-steel rounded-lg p-5 text-body text-zinc-400 leading-relaxed space-y-2">
      <h3 class="text-body font-bold text-gear-gold uppercase tracking-wider">Installing a skill</h3>
      <p>A skill is a folder holding a <code class="text-zinc-300">SKILL.md</code>. Copy or symlink the folder into one of these, then refresh:</p>
      <ul class="space-y-1">
        <li><code class="text-zinc-300 break-all">{{ skillsData.globalDir || '~/.config/ringmaster/skills' }}</code> — every project</li>
        <li><code class="text-zinc-300 break-all">{{ skillsData.projectDir || '<project>/.ringmaster/skills' }}</code> — this project only</li>
      </ul>
      <p>The switches here apply to every project. Scripts inside a skill still need a command your allowlist permits.</p>
    </div>
  </section>
</template>

<script setup>
import { onMounted } from 'vue'

const { skillsData, skillBusy, loadSkills, toggleSkill } = useRingmaster()

onMounted(loadSkills)
</script>
