// Out-in removes the old DOM before inserting the destination. Defer router scroll.
let readyPath = ''
let pending: (() => void)[] = []

export function pageReady(path: string) {
  readyPath = path
  const callbacks = pending
  pending = []
  callbacks.forEach(resolve => resolve())
}

export function pageLeaving() {
  readyPath = ''
}

export function waitForPage(path: string) {
  if (readyPath === path) return Promise.resolve()
  return new Promise<void>(resolve => pending.push(resolve))
}
