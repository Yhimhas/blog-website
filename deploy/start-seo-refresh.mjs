import { spawn } from 'node:child_process'

// Run one existing Worker at a time. Docker owns process startup/restart;
// PostgreSQL still owns publication state, leases and retry correctness.
let stopping = false
let child
let wake
for (const signal of ['SIGTERM', 'SIGINT']) {
  process.on(signal, () => {
    stopping = true
    wake?.()
    child?.kill('SIGTERM')
  })
}

while (!stopping) {
  await new Promise(resolve => {
    child = spawn('/opt/blog/bin/seo-refresh', [], { stdio: 'inherit' })
    child.once('error', error => {
      console.error(JSON.stringify({ event: 'seo-worker-start-failed', message: error.message }))
    })
    child.once('close', code => {
      child = undefined
      if (code !== 0 && !stopping) console.error(JSON.stringify({ event: 'seo-worker-exit', code }))
      resolve()
    })
  })
  if (stopping) break
  await new Promise(resolve => {
    const timer = setTimeout(resolve, 15_000)
    wake = () => { clearTimeout(timer); resolve() }
    if (stopping) wake()
  })
  wake = undefined
}
