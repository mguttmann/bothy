// SPDX-License-Identifier: Apache-2.0

// Package title decides the title source of a thread (docs/spec/09-agents.md section 12,
// docs/spec/03-data-model.md threads.title_source): whether a title a principal gives at
// creation is a placeholder, which source a new thread gets and which source a title change
// leaves. It is the only code that knows placeholders and title sources, so that a title a person
// chose is never overwritten automatically (docs/REUSE.md L23).
//
// The placeholder words are data per language in wordlists/<lang>.txt, embedded at build time;
// the default of thread.placeholder_titles is rendered from them (DefaultConfigText), so the words
// exist once (docs/DECISIONS.md D-L4). The effective list of that key is what the server compares
// against (NewPlaceholders).
//
// The package imports only the standard library and kernel-core (docs/spec/02-architecture.md
// section 5); the app layer classifies the principal into a By.
package title
