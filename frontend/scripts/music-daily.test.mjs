import assert from 'node:assert/strict'
import test from 'node:test'
import { dailySelection, shanghaiDate } from '../src/musicDaily.ts'

test('Shanghai day rolls over at UTC 16:00', () => {
  assert.equal(shanghaiDate(new Date('2026-09-21T15:59:59Z')), '2026-09-21')
  assert.equal(shanghaiDate(new Date('2026-09-21T16:00:00Z')), '2026-09-22')
})
test('selection is stable across order, unique, and does not mutate the source', () => {
  const items = Array.from({ length: 53 }, (_, i) => ({ id: `track-${i}` }))
  const before = items.map(item => item.id)
  const first = dailySelection(items, '2026-09-21')
  assert.deepEqual(first, dailySelection([...items].reverse(), '2026-09-21'))
  assert.deepEqual(first, dailySelection([...items, ...items], '2026-09-21'))
  assert.equal(first.length, 3)
  assert.deepEqual(items.map(item => item.id), before)
  assert.notDeepEqual(first, dailySelection(items, '2026-09-22'))
})
test('empty and small libraries have no fabricated or duplicate tracks', () => {
  assert.deepEqual(dailySelection([], '2026-09-21'), [])
  assert.deepEqual(dailySelection([{ id: 'one' }, { id: 'one' }], '2026-09-21'), [{ id: 'one' }])
})
