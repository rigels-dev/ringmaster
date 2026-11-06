// https://nuxt.com/docs/api/configuration/nuxt-config

// Colors resolve to CSS variables from assets/css/theme.css so the light/dark
// toggle can swap the whole palette without touching component classes.
const themed = (name: string) => `rgb(var(--c-${name}) / <alpha-value>)`
const SHADES = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]
const scale = (family: string) => Object.fromEntries(SHADES.map((s) => [s, themed(`${family}-${s}`)]))

// Applied before first paint so a saved light theme never flashes dark.
const themeBootScript = `try{var t=localStorage.getItem('rm-theme');if(t==='light'||(!t&&matchMedia('(prefers-color-scheme: light)').matches))document.documentElement.classList.add('light')}catch(e){}`

export default defineNuxtConfig({
  compatibilityDate: '2026-10-26',
  devtools: { enabled: false },
  css: ['~/assets/css/theme.css', '@xterm/xterm/css/xterm.css'],
  // WebSocket support for the web UI terminal (server/routes/_terminal.ts)
  nitro: {
    experimental: { websocket: true }
  },
  modules: [
    '@nuxtjs/tailwindcss',
    '@pinia/nuxt'
  ],
  app: {
    head: {
      title: 'Ringmaster — Multi-Agent Control Plane',
      script: [{ innerHTML: themeBootScript }],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/icons/ringmaster-icon-transparent.svg' },
        { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Cinzel:wght@500;700;900&family=IBM+Plex+Mono:ital,wght@0,300;0,400;0,600;1,400&family=Outfit:wght@300;400;600;800&display=swap' }
      ]
    }
  },
  tailwindcss: {
    config: {
      theme: {
        extend: {
          colors: {
            zinc: scale('zinc'),
            red: scale('red'),
            green: scale('green'),
            emerald: scale('emerald'),
            teal: scale('teal'),
            amber: scale('amber'),
            rose: scale('rose'),
            indigo: scale('indigo'),
            slate: {
              ...scale('slate'),
              deep: themed('slate-deep'),
              dark: themed('slate-dark'),
              panel: themed('slate-panel'),
              card: themed('slate-card'),
              inner: themed('slate-inner'),
            },
            gear: {
              gold: themed('gear-gold'),
              glow: themed('gear-glow'),
              dark: themed('gear-dark'),
            },
            coat: {
              crimson: '#B6432A',
              dim: '#7a2b1a',
              bright: '#e05436',
            },
            cyan: {
              ...scale('cyan'),
              energy: themed('cyan-energy'),
              glow: themed('cyan-glow'),
              dim: themed('cyan-dim'),
              dark: themed('cyan-dark'),
            },
            // Text on gold/cyan buttons: dark ink in dark mode, light in light mode
            ink: themed('ink'),
            parchment: themed('parchment'),
            steel: themed('steel')
          },
          fontFamily: {
            mono: ['"IBM Plex Mono"', 'monospace'],
            cinzel: ['Cinzel', 'serif'],
            outfit: ['Outfit', 'sans-serif'],
          }
        }
      }
    }
  }
})
