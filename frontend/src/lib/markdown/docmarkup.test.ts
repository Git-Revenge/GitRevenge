import { describe, expect, it } from 'vitest'
import { asciidocInline, asciidocToMarkdown } from './asciidoc'
import { rstInline, rstToMarkdown } from './rst'

describe('AsciiDoc', () => {
  it('turns the title, its byline and sections into headings', () => {
    const md = asciidocToMarkdown('= The Guide\nAna Cruz <ana@example.com>\n:toc:\n\n== Install\n\nRun it.\n\n=== From source\n')
    expect(md).toBe('# The Guide\n\n*Ana Cruz <ana@example.com>*\n\n## Install\n\nRun it.\n\n### From source\n')
  })

  it('reads its inline marks as Markdown’s', () => {
    expect(asciidocInline('a *bold* and _italic_ word, `code` and +lit+')).toBe('a **bold** and *italic* word, `code` and `lit`')
    expect(asciidocInline('**un**constrained and __it__alic')).toBe('**un**constrained and *it*alic')
    expect(asciidocInline('a #highlight# and [.role]#role#')).toBe('a highlight and role')
    // Inside literal text, nothing is a mark.
    expect(asciidocInline('`*not bold*`')).toBe('`*not bold*`')
  })

  it('reads links, cross references and images', () => {
    expect(asciidocInline('see https://example.com[the site] or link:docs/a.html[A]')).toBe('see [the site](https://example.com) or [A](docs/a.html)')
    expect(asciidocInline('in xref:install.adoc#src[Install], <<setup,Setup>>, <<other>>')).toBe('in [Install](install.adoc#src), Setup, other')
    expect(asciidocInline('logo: image:img/logo.png[Logo, 40]')).toBe('logo: ![Logo](img/logo.png)')
    expect(asciidocToMarkdown('image::img/shot.png[A shot]')).toBe('![A shot](img/shot.png)\n')
  })

  it('fills in attribute references', () => {
    expect(asciidocToMarkdown(':project: Git Revenge\n\nWelcome to {project}.')).toBe('Welcome to Git Revenge.\n')
  })

  it('turns lists into Markdown lists, nested by depth', () => {
    expect(asciidocToMarkdown('* one\n** two\n* three\n\n. first\n.. inner\n\nCPU:: the brain\n')).toBe('- one\n  - two\n- three\n\n1. first\n   1. inner\n\n- **CPU** the brain\n')
  })

  it('doesn’t read C++ in a sentence as a description list', () => {
    expect(asciidocToMarkdown('Use std::vector here.')).toBe('Use std::vector here.\n')
  })

  it('fences listing blocks with their language, and literal blocks without', () => {
    expect(asciidocToMarkdown('[source,python]\n----\nprint("*hi*")\n----\n')).toBe('```python\nprint("*hi*")\n```\n')
    expect(asciidocToMarkdown('....\nas is\n....\n')).toBe('```\nas is\n```\n')
  })

  it('shows a passthrough block as code, never as HTML', () => {
    expect(asciidocToMarkdown('++++\n<script>x()</script>\n++++\n')).toBe('```html\n<script>x()</script>\n```\n')
  })

  it('quotes admonitions, in a paragraph or a block', () => {
    expect(asciidocToMarkdown('NOTE: Back it up\nfirst.')).toBe('> **Note:** Back it up first.\n')
    expect(asciidocToMarkdown('[WARNING]\n====\nIt *will* delete.\n====\n')).toBe('> **Warning:**\n>\n> It **will** delete.\n')
  })

  it('gives a block its title', () => {
    expect(asciidocToMarkdown('.Example\n----\nx\n----\n')).toBe('**Example**\n\n```\nx\n```\n')
  })

  it('turns a table into a Markdown table, with its header row', () => {
    const md = asciidocToMarkdown('|===\n|Name |Score\n\n|Ana |3\n|Bo |5\n|===\n')
    expect(md).toBe('| Name | Score |\n| --- | --- |\n| Ana | 3 |\n| Bo | 5 |\n')
    // One cell to a line, with the header named in the options.
    expect(asciidocToMarkdown('[options="header"]\n|===\n|A\n|B\n|1\n|2\n|===\n')).toBe('| A |\n| --- |\n| B |\n| 1 |\n| 2 |\n')
  })

  it('leaves out comments, and names what an include would bring in', () => {
    expect(asciidocToMarkdown('// a note to self\n////\nhidden\n////\ninclude::parts/intro.adoc[]\n')).toBe("*(This includes parts/intro.adoc, which isn't shown here.)*\n")
  })
})

describe('reStructuredText', () => {
  it('turns titles into headings, by the order their styles first appear', () => {
    const md = rstToMarkdown('=====\nGuide\n=====\n\nInstall\n-------\n\nText.\n\nFrom source\n~~~~~~~~~~~\n\nMore\n----\n')
    expect(md).toBe('# Guide\n\n## Install\n\nText.\n\n### From source\n\n## More\n')
  })

  it('reads its inline marks as Markdown’s', () => {
    expect(rstInline('**strong**, *em*, ``lit`` and `role`')).toBe('**strong**, *em*, `lit` and *role*')
    expect(rstInline(':math:`x^2`, :code:`run()`, :ref:`Install <install>` and :py:func:`~os.path.join`')).toBe('$x^2$, `run()`, Install and `os.path.join`')
  })

  it('reads inline links, named links and anonymous ones', () => {
    expect(rstInline('see `the site <https://example.com>`_ and `docs <https://d.io>`__')).toBe('see [the site](https://example.com) and [docs](https://d.io)')
    const md = rstToMarkdown('Read Python_ and `the guide`_.\n\n.. _Python: https://python.org\n.. _the guide: https://example.com/guide\n')
    expect(md).toBe('Read [Python](https://python.org) and [the guide](https://example.com/guide).\n')
  })

  it('fills in substitutions, text and images', () => {
    expect(rstToMarkdown('|name| is |logo|\n\n.. |name| replace:: Git Revenge\n.. |logo| image:: img/logo.png\n')).toBe('Git Revenge is ![logo](img/logo.png)\n')
  })

  it('turns a paragraph ending :: into a literal block', () => {
    expect(rstToMarkdown('Run this::\n\n    make all\n    make test\n\nDone.\n')).toBe('Run this:\n\n```\nmake all\nmake test\n```\n\nDone.\n')
    expect(rstToMarkdown('Before ::\n\n    code\n')).toBe('Before\n\n```\ncode\n```\n')
    expect(rstToMarkdown('::\n\n    alone\n')).toBe('```\nalone\n```\n')
  })

  it('fences code directives with their language, past their options', () => {
    expect(rstToMarkdown('.. code-block:: python\n   :linenos:\n\n   print("hi")\n')).toBe('```python\nprint("hi")\n```\n')
  })

  it('quotes admonitions, with their text', () => {
    expect(rstToMarkdown('.. note:: Back it up.\n\n   Really.\n')).toBe('> **Note:** Back it up.\n>\n> Really.\n')
    expect(rstToMarkdown('.. warning::\n\n   It *will* delete.\n')).toBe('> **Warning:** It *will* delete.\n')
  })

  it('shows images and figures, with a figure’s caption', () => {
    expect(rstToMarkdown('.. image:: img/a.png\n   :alt: A shot\n')).toBe('![A shot](img/a.png)\n')
    expect(rstToMarkdown('.. figure:: img/b.png\n\n   The caption.\n')).toBe('![](img/b.png)\n\n*The caption.*\n')
  })

  it('turns lists into Markdown lists', () => {
    expect(rstToMarkdown('- one\n- two\n  runs on\n\n  - inner\n\n1. first\n#. second\n')).toBe('- one\n- two runs on\n\n  - inner\n\n1. first\n1. second\n')
    expect(rstToMarkdown(':Author: Ana\n:Version: 1.0\n')).toBe('- **Author:** Ana\n- **Version:** 1.0\n')
  })

  it('turns a definition list into terms and their definitions', () => {
    expect(rstToMarkdown('CPU\n   The brain.\n\nRAM : memory\n   Short term.\n')).toBe('**CPU**\n\nThe brain.\n\n**RAM** *(memory)*\n\nShort term.\n')
  })

  it('quotes an indented block, and keeps tables as they’re drawn', () => {
    expect(rstToMarkdown('Said:\n\n   Be kind.\n')).toBe('Said:\n\n> Be kind.\n')
    expect(rstToMarkdown('=====  =====\nA      B\n=====  =====\n1      2\n=====  =====\n')).toBe('```\n=====  =====\nA      B\n=====  =====\n1      2\n=====  =====\n```\n')
  })

  it('leaves out comments, targets and site directives, and names the rest', () => {
    expect(rstToMarkdown('.. a comment\n   on two lines\n\n.. toctree::\n   :maxdepth: 2\n\n   intro\n\nText.\n\n.. graphviz:: x.dot\n')).toBe("Text.\n\n*(A graphviz directive, which isn't shown here.)*\n")
  })

  it('keeps a line that would read as Markdown structure as text', () => {
    expect(rstToMarkdown('# not a heading\n')).toBe('\\# not a heading\n')
  })

  it('shows math, doctests and line blocks', () => {
    expect(rstToMarkdown('.. math::\n\n   e^{i\\pi} + 1 = 0\n')).toBe('$$\ne^{i\\pi} + 1 = 0\n$$\n')
    expect(rstToMarkdown('>>> 1 + 1\n2\n')).toBe('```python\n>>> 1 + 1\n2\n```\n')
    expect(rstToMarkdown('| Roses are red\n| Violets are blue\n')).toBe('Roses are red\\\nViolets are blue\n')
  })
})
