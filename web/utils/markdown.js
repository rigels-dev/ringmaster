import MarkdownIt from 'markdown-it'

// Text from a model or a worker's report is untrusted: raw HTML is escaped
// (html: false), unsafe link schemes are dropped by markdown-it, and images are
// off so a reply cannot make the browser fetch a remote URL.
const md = new MarkdownIt({ html: false, linkify: false, breaks: true })
md.disable('image')

const defaultLinkOpen = md.renderer.rules.link_open ||
  ((tokens, idx, options, env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('target', '_blank')
  tokens[idx].attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpen(tokens, idx, options, env, self)
}

// renderMarkdown turns markdown into HTML that is safe to place with v-html.
export function renderMarkdown(source) {
  return md.render(String(source ?? ''))
}
