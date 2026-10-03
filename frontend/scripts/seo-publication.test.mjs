import assert from 'node:assert/strict'
import { test } from 'node:test'
import { effectScope, nextTick } from 'vue'
import { ApiError } from '../src/blogApi.ts'
import { useSeoPublication } from '../src/useSeoPublication.ts'

const receipt = { sourceId: '11111111-2222-3333-4444-555555555555', revision: '9007199254740993', appliedRevision: '9007199254740993', lastAttemptAt: '2026-10-03T00:00:00Z', publishedAt: '2026-10-03T00:00:00Z', lastError: '' }
const settle = async () => { await Promise.resolve(); await nextTick() }
function monitor(t, options) {
  const scope = effectScope()
  const model = scope.run(() => useSeoPublication(options))
  t.after(() => scope.stop())
  return { ...model, scope }
}

test('receipt transitions distinguish pending, last failure, completion and unavailable status', async t => {
  let result = receipt
  let fail = false
  const model = monitor(t, { request: async () => { if (fail) throw new Error('offline'); return result } })
  await model.start()
  assert.equal(model.state.value.status, 'current')
  result = { ...receipt, appliedRevision: '9007199254740992', lastAttemptAt: null }
  await model.refresh()
  assert.equal(model.state.value.status, 'pending')
  assert.equal(model.state.value.data.revision, receipt.revision)
  result = { ...result, lastError: 'build failed' }
  await model.refresh()
  assert.equal(model.state.value.status, 'failed')
  fail = true
  await model.refresh()
  assert.equal(model.state.value.status, 'unknown')
  assert.equal(model.state.value.data, undefined)
  fail = false
  result = receipt
  await model.refresh()
  assert.equal(model.state.value.status, 'current')
})

test('successful article changes invalidate a poll started before the mutation', async t => {
  const pending = []
  const model = monitor(t, { request: signal => new Promise(resolve => { pending.push({ resolve, signal }) }) })
  const initial = model.start()
  model.markPending()
  assert.equal(model.state.value.status, 'pending')
  assert.equal(pending[0].signal.aborted, true)
  const updated = model.refresh()
  pending[0].resolve(receipt)
  await initial
  assert.equal(model.state.value.status, 'pending', 'old success must not hide the post-write warning')
  assert.equal(model.loading.value, true, 'old finally must not clear the new request')
  pending[1].resolve({ ...receipt, revision: '9007199254740994' })
  await updated
  assert.equal(model.state.value.status, 'pending')
})

test('newer manual refresh wins even if an older failure arrives later', async t => {
  const pending = []
  const model = monitor(t, { request: signal => new Promise((resolve, reject) => { pending.push({ resolve, reject, signal }) }) })
  const old = model.start()
  const latest = model.refresh()
  pending[1].resolve(receipt)
  await latest
  pending[0].reject(new Error('late error'))
  await old
  assert.equal(model.state.value.status, 'current')
  assert.equal(pending[0].signal.aborted, true)
})

test('polling notices background changes and stops after logout', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let calls = 0
  const model = monitor(t, { request: async () => ++calls === 1 ? receipt : { ...receipt, revision: '9007199254740994' } })
  await model.start()
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(calls, 2)
  assert.equal(model.state.value.status, 'pending')
  model.stop()
  t.mock.timers.tick(15000)
  await settle()
  assert.equal(calls, 2)
  assert.equal(model.state.value.status, 'checking')
})

test('authorization loss is reported independently and can stop status polling', async t => {
  let calls = 0
  let denied
  let model
  model = monitor(t, { request: async () => { calls++; throw new ApiError(403, 'ADMIN_REQUIRED', '需要管理员权限') }, onUnauthorized: error => { denied = error; model.stop() } })
  await model.start()
  assert.equal(denied.status, 403)
  assert.equal(calls, 1)
  assert.equal(model.loading.value, false)
})

test('leaving the page aborts requests and ignores late results', async t => {
  let resolve
  let signal
  let calls = 0
  const model = monitor(t, { request: active => { calls++; signal = active; return new Promise(done => { resolve = done }) } })
  const active = model.start()
  model.scope.stop()
  assert.equal(signal.aborted, true)
  resolve(receipt)
  await active
  assert.equal(model.state.value.status, 'checking')
  await model.start()
  await model.refresh()
  assert.equal(calls, 1)
})
