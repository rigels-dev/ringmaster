<template>
  <div 
    class="relative inline-flex items-center justify-center select-none"
    :class="containerClass"
    :title="statusTooltip"
  >
    <svg 
      xmlns="http://www.w3.org/2000/svg" 
      viewBox="0 0 800 800" 
      class="w-full h-full"
      :class="status === 'overdrive' ? 'rm-glow-running' : 'drop-shadow-[0_0_12px_rgba(222,170,59,0.35)]'"
    >
      <defs>
        <component :is="'style'">
          @keyframes rmGearSpin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
          @keyframes rmRingCounterSpin { from { transform: rotate(360deg); } to { transform: rotate(0deg); } }
          @keyframes rmLeftHandControl { 0%, 100% { transform: translateY(-4px) rotate(-5.5deg); } 50% { transform: translateY(5px) rotate(5.5deg); } }
          @keyframes rmRightHandControl { 0%, 100% { transform: translateY(5px) rotate(-5.5deg); } 50% { transform: translateY(-4px) rotate(5.5deg); } }
          @keyframes rmPuppetBobA { 0%, 100% { transform: translateY(-7px) rotate(-2.5deg); } 50% { transform: translateY(6px) rotate(2.5deg); } }
          @keyframes rmPuppetBobB { 0%, 100% { transform: translateY(6px) rotate(2.5deg); } 50% { transform: translateY(-7px) rotate(-2.5deg); } }
          @keyframes rmHeadFloat { 0%, 100% { transform: translateY(-2.5px) rotate(-0.8deg); } 50% { transform: translateY(3px) rotate(0.8deg); } }
          @keyframes energyPulse { to { stroke-dashoffset: -28; } }

          .rm-gear { 
            transform-origin: 400px 400px; 
            animation: rmGearSpin var(--rm-gear-dur, 20s) linear infinite; 
          }
          .rm-ring { 
            transform-origin: 400px 400px; 
            animation: rmRingCounterSpin var(--rm-ring-dur, 44s) linear infinite; 
          }
          .rm-hand-l { 
            transform-origin: 255px 412px; 
            animation: rmLeftHandControl var(--rm-hand-dur, 2.4s) ease-in-out infinite; 
          }
          .rm-hand-r { 
            transform-origin: 545px 412px; 
            animation: rmRightHandControl var(--rm-hand-dur, 2.4s) ease-in-out infinite; 
          }
          .rm-pup-a { 
            transform-origin: 336px 585px; 
            animation: rmPuppetBobA var(--rm-hand-dur, 2.4s) ease-in-out infinite; 
          }
          .rm-pup-b { 
            transform-origin: 464px 585px; 
            animation: rmPuppetBobB var(--rm-hand-dur, 2.4s) ease-in-out infinite; 
          }
          .rm-head { 
            transform-origin: 400px 320px; 
            animation: rmHeadFloat 3.2s ease-in-out infinite; 
          }
          .rm-strings { 
            stroke-dasharray: 6 10; 
            animation: energyPulse var(--rm-pulse-dur, 0.6s) linear infinite; 
          }
        </component>

        <filter id="rm-cyan-glow" x="-40%" y="-40%" width="180%" height="180%">
          <feGaussianBlur stdDeviation="6" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>

        <radialGradient id="rm-lens-grad" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#99FFFF" />
          <stop offset="60%" stop-color="#29F2EC" />
          <stop offset="100%" stop-color="#14C2BD" />
        </radialGradient>

        <clipPath id="rm-ring-outer-clip">
          <circle cx="400" cy="400" r="245" />
        </clipPath>

        <clipPath id="rm-face-clip">
          <path d="M 332,276 L 468,276 L 463,372 L 424,442 L 376,442 L 337,372 Z" />
        </clipPath>

        <g id="rm-gear-tooth">
          <polygon points="362,66 438,66 452,130 348,130" fill="#DEAA3B" />
        </g>

        <g id="rm-ring-divider">
          <rect x="394" y="148" width="12" height="42" fill="#1F272E" />
        </g>
      </defs>

      <!-- Outer Golden Gear (Kinematic Rotation) -->
      <g 
        class="rm-gear"
        :style="{
          '--rm-gear-dur': animDurations.gearDur,
          '--rm-ring-dur': animDurations.ringDur,
          '--rm-hand-dur': animDurations.handDur,
          '--rm-pulse-dur': animDurations.pulseDur
        }"
      >
        <use href="#rm-gear-tooth" transform="rotate(0 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(30 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(60 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(90 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(120 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(150 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(180 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(210 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(240 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(270 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(300 400 400)" />
        <use href="#rm-gear-tooth" transform="rotate(330 400 400)" />
        <circle cx="400" cy="400" r="286" fill="#DEAA3B" />
      </g>

      <!-- Segmented Terracotta Ring (Counter-Rotation) -->
      <circle cx="400" cy="400" r="256" fill="#1F272E" />
      <g 
        class="rm-ring"
        :style="{
          '--rm-ring-dur': animDurations.ringDur
        }"
      >
        <circle cx="400" cy="400" r="245" fill="#B6432A" />
        <use href="#rm-ring-divider" transform="rotate(0 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(30 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(60 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(90 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(120 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(150 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(180 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(210 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(240 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(270 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(300 400 400)" />
        <use href="#rm-ring-divider" transform="rotate(330 400 400)" />
      </g>
      <circle cx="400" cy="400" r="222" fill="#1F272E" />

      <!-- Worker Puppets in Arena -->
      <g clip-path="url(#rm-ring-outer-clip)">
        <g class="rm-pup-a" :style="{ '--rm-hand-dur': animDurations.handDur }" fill="#B6432A">
          <circle cx="316" cy="562" r="13" />
          <path d="M 298,580 C 298,576 334,576 334,580 L 338,645 L 294,645 Z" />
          <path d="M 286,562 L 302,586" stroke="#B6432A" stroke-width="10" stroke-linecap="round" />
        </g>
        <g class="rm-pup-b" :style="{ '--rm-hand-dur': animDurations.handDur }" fill="#DEAA3B">
          <circle cx="356" cy="576" r="12" />
          <path d="M 340,592 C 340,588 372,588 372,592 L 376,655 L 336,655 Z" />
          <path d="M 368,602 L 386,584" stroke="#DEAA3B" stroke-width="9" stroke-linecap="round" />
        </g>
        <g class="rm-pup-a" :style="{ '--rm-hand-dur': animDurations.handDur }" fill="#B6432A">
          <circle cx="444" cy="576" r="12" />
          <path d="M 428,592 C 428,588 460,588 460,592 L 464,655 L 424,655 Z" />
          <path d="M 432,602 L 414,584" stroke="#B6432A" stroke-width="9" stroke-linecap="round" />
        </g>
        <g class="rm-pup-b" :style="{ '--rm-hand-dur': animDurations.handDur }" fill="#DEAA3B">
          <circle cx="484" cy="562" r="13" />
          <path d="M 466,580 C 466,576 502,576 502,580 L 506,645 L 462,645 Z" />
        </g>
      </g>

      <!-- Torso, Crimson Coat, Sleeves & Keyhole -->
      <g stroke="#1F272E" stroke-width="8" stroke-linejoin="round" stroke-linecap="round">
        <path d="M 282,425 L 332,395 L 400,455 L 468,395 L 518,425 L 496,510 Q 400,536 304,510 Z" fill="#B6432A" />
        <path d="M 218,435 C 216,492 284,496 296,455 L 280,425 Z" fill="#B6432A" />
        <path d="M 582,435 C 584,492 516,496 504,455 L 520,425 Z" fill="#B6432A" />
        <path d="M 336,385 L 302,412 L 368,504 L 400,472 L 432,504 L 498,412 L 464,385 Z" fill="#DEAA3B" />
        <polygon points="362,435 438,435 415,518 385,518" fill="#1F272E" />
        <polygon points="370,458 386,482 378,505" fill="#B6432A" stroke-width="5" />
        <polygon points="430,458 414,482 422,505" fill="#B6432A" stroke-width="5" />
        <path d="M 386,490 A 15,15 0 1 1 414,490 L 418,520 L 382,520 Z" fill="#DEAA3B" stroke-width="7" />
        <circle cx="400" cy="485" r="5" fill="#1F272E" stroke="none" />
        <polygon points="396,488 404,488 406,510 394,510" fill="#1F272E" stroke="none" />
      </g>

      <!-- Cyan Energy Strings Control Strings -->
      <g stroke="#3DF5F0" stroke-width="4.5" stroke-linecap="round" filter="url(#rm-cyan-glow)">
        <line x1="205" y1="412" x2="208" y2="518" />
        <line x1="306" y1="420" x2="316" y2="552" />
        <line x1="316" y1="422" x2="356" y2="566" />
        <line x1="484" y1="422" x2="444" y2="566" />
        <line x1="494" y1="420" x2="484" y2="552" />
        <line x1="595" y1="412" x2="592" y2="518" />
      </g>
      <g 
        v-if="status !== 'idle'" 
        class="rm-strings" 
        :style="{ '--rm-pulse-dur': animDurations.pulseDur }" 
        stroke="#E6FFFF" 
        stroke-width="2.5" 
        stroke-linecap="round" 
        filter="url(#rm-cyan-glow)"
      >
        <line x1="205" y1="412" x2="208" y2="518" />
        <line x1="306" y1="420" x2="316" y2="552" />
        <line x1="316" y1="422" x2="356" y2="566" />
        <line x1="484" y1="422" x2="444" y2="566" />
        <line x1="494" y1="420" x2="484" y2="552" />
        <line x1="595" y1="412" x2="592" y2="518" />
      </g>

      <!-- Head, Top Hat & Face (Stable, not spinning!) -->
      <g class="rm-head">
        <path d="M 288,276 L 512,276 L 478,288 L 322,288 Z" fill="#DEAA3B" stroke="#1F272E" stroke-width="8" stroke-linejoin="round" />
        <polygon points="326,276 474,276 458,162 342,162" fill="#1F272E" stroke="#1F272E" stroke-width="8" stroke-linejoin="round" />
        <polygon points="330,272 470,272 466,242 334,242" fill="#B6432A" />
        <circle cx="400" cy="242" r="16" fill="#DEAA3B" stroke="#1F272E" stroke-width="6" />

        <!-- Porcelain Mask Face -->
        <g stroke="#1F272E" stroke-width="8" stroke-linejoin="round">
          <path d="M 332,276 L 468,276 L 463,372 L 424,442 L 376,442 L 337,372 Z" fill="#EFE4CE" />
          <path d="M 346,310 C 374,310 382,336 400,336 C 418,336 426,310 454,310 L 454,354 C 426,354 418,378 400,378 C 382,378 374,354 346,354 Z" fill="#B6432A" />
          <polygon points="392,348 408,348 400,378" fill="#1F272E" />
          <path d="M 378,405 Q 400,422 422,405" fill="none" stroke="#1F272E" stroke-width="7" stroke-linecap="round" />

          <!-- Monocle & Cyan Ocular Eye -->
          <circle cx="430" cy="332" r="24" fill="url(#rm-lens-grad)" stroke="#DEAA3B" stroke-width="8" filter="url(#rm-cyan-glow)" />
          <circle cx="430" cy="332" r="7" fill="#FFFFFF" />
          <line x1="360" y1="332" x2="384" y2="332" stroke="#1F272E" stroke-width="6" stroke-linecap="round" />
        </g>
      </g>

      <!-- Marionette Control Hands (Tugging Puppets) -->
      <g class="rm-hand-l" :style="{ '--rm-hand-dur': animDurations.handDur }" stroke="#1F272E" stroke-width="8" stroke-linejoin="round">
        <polygon points="200,392 310,392 302,432 208,432" fill="#DEAA3B" />
        <circle cx="205" cy="412" r="14" fill="#EFE4CE" />
        <circle cx="306" cy="420" r="14" fill="#EFE4CE" />
        <circle cx="316" cy="422" r="14" fill="#EFE4CE" />
      </g>
      <g class="rm-hand-r" :style="{ '--rm-hand-dur': animDurations.handDur }" stroke="#1F272E" stroke-width="8" stroke-linejoin="round">
        <polygon points="490,392 600,392 592,432 498,432" fill="#DEAA3B" />
        <circle cx="484" cy="422" r="14" fill="#EFE4CE" />
        <circle cx="494" cy="420" r="14" fill="#EFE4CE" />
        <circle cx="595" cy="412" r="14" fill="#EFE4CE" />
      </g>
    </svg>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: {
    type: String,
    default: 'idle' // 'idle' | 'dag' | 'execution' | 'overdrive'
  },
  size: {
    type: String,
    default: 'md' // 'sm' | 'md' | 'lg'
  }
})

const containerClass = computed(() => {
  switch (props.size) {
    case 'sm': return 'w-8 h-8'
    case 'lg': return 'w-24 h-24'
    case 'xl': return 'w-44 h-44'
    default: return 'w-10 h-10'
  }
})

const animDurations = computed(() => {
  switch (props.status) {
    case 'overdrive':
      // Running tasks: the spin itself is the activity indicator, so make it obvious
      return { gearDur: '2.4s', ringDur: '5.5s', handDur: '0.8s', pulseDur: '0.2s' }
    case 'dag':
    case 'planning':
      return { gearDur: '14.0s', ringDur: '30.0s', handDur: '1.8s', pulseDur: '0.45s' }
    case 'execution':
    case 'running':
      return { gearDur: '20.0s', ringDur: '44.0s', handDur: '2.4s', pulseDur: '0.6s' }
    case 'idle':
    default:
      return { gearDur: '50.0s', ringDur: '110.0s', handDur: '4.2s', pulseDur: '1.2s' }
  }
})

const statusTooltip = computed(() => {
  switch (props.status) {
    case 'overdrive': return 'Running at full speed'
    case 'dag':
    case 'planning': return 'The Master is planning'
    case 'execution':
    case 'running': return 'Automatons are working'
    default: return 'Idle'
  }
})
</script>

<style scoped>
/* Running tasks: a pulsing blue glow replaces the idle gold one */
.rm-glow-running {
  animation: rmGlowRunning 1.6s ease-in-out infinite;
}
@keyframes rmGlowRunning {
  0%, 100% { filter: drop-shadow(0 0 6px rgb(var(--c-cyan-energy) / 0.55)); }
  50% { filter: drop-shadow(0 0 16px rgb(var(--c-cyan-energy) / 0.95)); }
}
@media (prefers-reduced-motion: reduce) {
  .rm-glow-running {
    animation: none;
    filter: drop-shadow(0 0 12px rgb(var(--c-cyan-energy) / 0.8));
  }
}
</style>
