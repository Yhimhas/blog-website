import { seoHead, siteOrigin, type SeoPage } from './seo'

export function applySeo(page: SeoPage) {
  const origin = siteOrigin(import.meta.env.VITE_SITE_URL || window.location.origin)
  const template = document.createElement('template')
  template.innerHTML = seoHead({ ...page, noindex: page.noindex || import.meta.env.MODE !== 'production' }, origin)
  document.head.querySelectorAll('[data-seo]').forEach(node => node.remove())
  document.head.append(template.content)
}
