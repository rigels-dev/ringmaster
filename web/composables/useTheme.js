import { ref, onMounted } from 'vue'

// Light/dark theme. The boot script in nuxt.config.ts applies the saved
// choice before first paint; this keeps the toggle in sync with it.
const STORAGE_KEY = 'rm-theme'
const theme = ref('dark')

export function useTheme() {
  onMounted(() => {
    theme.value = document.documentElement.classList.contains('light') ? 'light' : 'dark'
  })

  const toggleTheme = () => {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    document.documentElement.classList.toggle('light', theme.value === 'light')
    try {
      localStorage.setItem(STORAGE_KEY, theme.value)
    } catch {}
  }

  return { theme, toggleTheme }
}
