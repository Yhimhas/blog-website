import MarkdownIt from 'markdown-it'

// API content and editor previews share the same renderer. Raw HTML is never enabled.
const markdown = new MarkdownIt({ html: false })
export const renderMarkdown = (source: string) => markdown.render(source)
