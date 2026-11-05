import { marked } from 'marked'
import { describe, expect, it } from 'vitest'
import { mdxToMarkdown as md } from './mdx'

const types = (text: string) =>
  marked
    .lexer(md(text))
    .map((t) => t.type)
    .filter((t) => t !== 'space')

describe('mdxToMarkdown', () => {
  it('leaves plain Markdown alone', () => {
    const text = '# Title\n\nSome *text* with a [link](a.md).\n\n- one\n  - two\n\n    indented in a list\n'
    expect(md(text)).toBe(text)
  })

  it('removes frontmatter', () => {
    expect(md('---\ntitle: Hello\n---\n\n# Title\n')).toBe('# Title\n')
  })

  it('treats a rule later on as a rule', () => {
    expect(md('# Title\n\n---\n\ntext\n\n---\n')).toBe('# Title\n\n---\n\ntext\n\n---\n')
  })

  it('removes imports and exports, including ones over several lines', () => {
    const text = `import A from './a'\nimport { B,\n  C } from "./b"\n\nexport const meta = {\n  title: 'x',\n\n  more: true,\n}\n\n# Title\n`
    expect(md(text)).toBe('# Title\n')
  })

  it('keeps prose that only begins with the words', () => {
    expect(md('importing and exporting\n')).toBe('importing and exporting\n')
    expect(md('Words first\nimport this line\n')).toBe('Words first\nimport this line\n')
  })

  it('removes comments', () => {
    expect(md('before {/* a\nlong note */} after\n')).toBe('before  after\n')
  })

  it('keeps the Markdown inside a component', () => {
    expect(types('<Note>\n\n**Bold** text\n\n</Note>\n')).toEqual(['paragraph'])
    expect(md('<Note>\n**Bold** text\n</Note>\n').trim()).toBe('**Bold** text')
  })

  it('keeps what is inside an inline component', () => {
    expect(md('Hello <Badge color="red">new</Badge> world\n')).toBe('Hello new world\n')
  })

  it('reads attributes over several lines, with > inside values', () => {
    expect(md(`<Foo\n  a="x>y"\n  on={() => { return 1 > 0 }}\n  b='>'\n>\ninside\n</Foo>\n`).trim()).toBe('inside')
  })

  it('does not read indented content as code', () => {
    const text = '<Tabs>\n  <TabItem value="a">\n    Some text\n\n    - one\n      - two\n  </TabItem>\n</Tabs>\n'
    expect(md(text).trim()).toBe('Some text\n\n- one\n  - two')
    expect(types(text)).toEqual(['paragraph', 'list'])
  })

  it('shows the name of a component with nothing inside', () => {
    expect(md('<Figure src="a.png" />\n')).toBe('`<Figure />`\n')
  })

  it('drops self-closing HTML, and turns an img into an image', () => {
    expect(md('one<br />two\n')).toBe('onetwo\n')
    expect(md('<img src="a b.png" alt="A" />\n')).toBe('![A](<a b.png>)\n')
  })

  it('removes fragments', () => {
    expect(md('<>\ninside\n</>\n').trim()).toBe('inside')
  })

  it('leaves code exactly as written', () => {
    const fenced = '```jsx\nimport A from "a"\n\n<Foo a={1} />\n{/* note */}\n```\n'
    expect(md(fenced)).toBe(fenced)
    expect(md('Use `<Foo />` and ``a ` <b>`` here\n')).toBe('Use `<Foo />` and ``a ` <b>`` here\n')
  })

  it('keeps a fence inside a component, dedented', () => {
    expect(md('<Tab>\n  ```js\n  <Foo />\n  ```\n</Tab>\n').trim()).toBe('```js\n<Foo />\n```')
  })

  it('leaves text that only looks like a tag', () => {
    for (const text of ['a < b and c > d\n', 'See <https://example.com> now\n', 'Mail <me@example.com>\n', 'Written \\<Foo> here\n', 'x <y\n\nz> w\n']) {
      expect(md(text)).toBe(text)
    }
  })

  it('handles CRLF', () => {
    expect(md('---\r\ntitle: x\r\n---\r\nimport A from "a"\r\n\r\n# Title\r\n')).toBe('# Title\n')
  })

  it('survives an unclosed component, fence or comment', () => {
    expect(md('<Foo>\ntext\n').trim()).toBe('text')
    expect(md('```\n<Foo />\n')).toBe('```\n<Foo />\n')
    expect(md('text {/* never closed\n').trim()).toBe('text')
  })
})
