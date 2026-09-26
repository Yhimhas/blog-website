/** Shanghai calendar helper. Legacy local selection is retained but no longer used by the music page. */
export function shanghaiDate(now = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  }).formatToParts(now)
  const get = (type: string) => parts.find(part => part.type === type)?.value
  return `${get('year')}-${get('month')}-${get('day')}`
}

function hash(value: string): number {
  let result = 2166136261
  for (const char of value) result = Math.imul(result ^ char.charCodeAt(0), 16777619)
  return result >>> 0
}

export function dailySelection<T extends { id: string }>(items: readonly T[], date: string, count = 3): T[] {
  const unique = [...new Map(items.map(item => [item.id, item])).values()]
  return unique.sort((a, b) => hash(`${date}:${a.id}`) - hash(`${date}:${b.id}`) || (a.id < b.id ? -1 : 1))
    .slice(0, Math.max(0, Math.floor(count)))
}
