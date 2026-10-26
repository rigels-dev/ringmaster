// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-10-26',
  devtools: { enabled: false },
  modules: [
    '@nuxtjs/tailwindcss',
    '@pinia/nuxt'
  ],
  app: {
    head: {
      title: 'Ringmaster — Multi-Agent Control Plane',
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
            slate: {
              deep: '#192026',
              dark: '#232B33',
              panel: '#1b2229',
              card: '#151a20',
              inner: '#0f1418',
            },
            gear: {
              gold: '#DEAA3B',
              glow: '#ffc857',
              dark: '#8c6418',
            },
            coat: {
              crimson: '#B6432A',
              dim: '#7a2b1a',
              bright: '#e05436',
            },
            cyan: {
              energy: '#29F2EC',
              glow: '#88FFFF',
              dim: '#14C2BD',
              dark: '#0a4f4e',
            },
            parchment: '#EFE4CE',
            steel: '#2e3842'
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
