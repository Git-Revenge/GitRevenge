import { describe, expect, it } from 'vitest'
import { groupTrailers, asPerson, valueParts } from './trailers'

describe('groupTrailers', () => {
  const line = (key: string, value: string) => ({ key, value })
  const kernel = [
    line('Link', 'https://lore.kernel.org/a'),
    line('Fixes', '5e31275cc997 ("mm: add per-VMA lock")'),
    line('Signed-off-by', 'Lorenzo <l@kernel.org>'),
    line('Signed-off-by', 'Andrew <a@linux.org>'),
    line('Reported-by', 'Daehyeon <d@gmail.com>'),
    line('Closes', 'https://lore.kernel.org/b'),
    line('Reviewed-by', 'Jose <j@gmail.com>'),
    line('Cc', 'Suren <s@google.com>'),
    line('Cc', 'Liam <l@infradead.org>'),
    line('Cc', '<stable@vger.kernel.org> # 7.3.x'),
  ]

  it('puts what the commit points at first, in a fixed order', () => {
    expect(groupTrailers(kernel).related.map((r) => r.label)).toEqual(['Fixes', 'Closes', 'Link', 'Backport'])
  })

  it('lists credits before sign-offs', () => {
    expect(groupTrailers(kernel).people.map((r) => r.label)).toEqual(['Reported by', 'Reviewed by', 'Signed off by'])
  })

  it("keeps the message's order within a row, since sign-offs are a chain", () => {
    const signed = groupTrailers(kernel).people.find((r) => r.key === 'signed-off-by')
    expect(signed?.values).toEqual(['Lorenzo <l@kernel.org>', 'Andrew <a@linux.org>'])
  })

  it('keeps Cc apart', () => {
    expect(groupTrailers(kernel).cc).toEqual(['Suren <s@google.com>', 'Liam <l@infradead.org>'])
  })

  it('reads a stable Cc as a backport, with or without versions', () => {
    expect(groupTrailers([line('Cc', '<stable@vger.kernel.org> # 7.3.x')]).related[0]).toMatchObject({ label: 'Backport', values: ['Stable 7.3.x'] })
    expect(groupTrailers([line('Cc', 'stable@vger.kernel.org')]).related[0]).toMatchObject({ label: 'Backport', values: ['Stable'] })
    expect(groupTrailers([line('Cc', 'stable@vger.kernel.org')]).cc).toEqual([])
  })

  it('ignores the case of a key, and folds spellings together', () => {
    const rows = groupTrailers([line('closes', '#1'), line('CLOSED', '#2')]).related
    expect(rows).toEqual([{ key: 'closes', label: 'Closes', values: ['#1', '#2'] }])
  })

  it("leaves a trailer it doesn't know in the message's order, as written", () => {
    expect(groupTrailers([line('Zebra', 'z'), line('Assisted-by', 'LLM')]).other.map((r) => r.label)).toEqual(['Zebra', 'Assisted-by'])
  })

  it('has empty groups for no trailers', () => {
    expect(groupTrailers([])).toEqual({ related: [], people: [], other: [], cc: [] })
  })
})

describe('asPerson', () => {
  it('reads "Name <email>"', () => {
    expect(asPerson('Ada Lovelace <ada@example.com>')).toEqual({ name: 'Ada Lovelace', email: 'ada@example.com' })
  })

  it('rejects anything else', () => {
    expect(asPerson('#12')).toBeNull()
    expect(asPerson('<ada@example.com>')).toBeNull()
    expect(asPerson('Ada <not an email>')).toBeNull()
  })
})

describe('valueParts', () => {
  const issues = 'https://github.com/o/r/issues/{n}'

  it('links issue references when the pattern is known', () => {
    expect(valueParts('#1, #22', issues)).toEqual([
      { text: '#1', href: 'https://github.com/o/r/issues/1' },
      { text: ', ' },
      { text: '#22', href: 'https://github.com/o/r/issues/22' },
    ])
  })

  it('leaves issue references as text without one', () => {
    expect(valueParts('#7', '')).toEqual([{ text: '#7' }])
  })

  it('links URLs, leaving trailing punctuation out', () => {
    expect(valueParts('see https://example.com/a?b=1.', '')).toEqual([
      { text: 'see ' },
      { text: 'https://example.com/a?b=1', href: 'https://example.com/a?b=1' },
      { text: '.' },
    ])
  })

  it('returns plain text as one part', () => {
    expect(valueParts('no links here', issues)).toEqual([{ text: 'no links here' }])
  })
})
