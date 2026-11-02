// SPDX-License-Identifier: Apache-2.0

// The sample documents of the document viewer tests (ticket FEAT-markdown-viewer): one Markdown
// file with every element the viewer renders.

/// A Markdown document with headings, emphasis, inline code, links (web, relative, anchor),
/// nested and ordered lists, task lists, a quote, a table, a fenced code block with a language,
/// a horizontal rule, raw HTML and images by URL and by another attachment.
const String sampleMarkdown = r'''
# Release notes

The **new** viewer shows *Markdown* with `inline code`, a [web link](https://example.com/docs) and <b>no HTML</b>.

## Steps

1. Download the build
   - check the size
   - check the digest
2. Install the build

- [x] Write the notes
- [ ] Review the notes

> A quote of a step that matters.

| Step | Owner | Status |
|------|-------|--------|
| Build | Alex | done |
| Review | Sam | open |

```dart
final answer = 42; // the answer
print('step $answer');
```

---

### Links

See [the setup](setup.md), go back to [the top](#release-notes), and look at ![the diagram](diagram.png) or ![a remote chart](https://example.com/chart.png).
''';
