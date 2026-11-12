<template>
  <div class="md" v-html="html"></div>
</template>

<script setup>
import { computed } from 'vue'
import { renderMarkdown } from '~/utils/markdown'

const props = defineProps({
  source: { type: String, default: '' }
})

// renderMarkdown escapes raw HTML, so its output is safe for v-html
const html = computed(() => renderMarkdown(props.source))
</script>
