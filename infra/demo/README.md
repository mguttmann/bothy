# Demo data set (`infra/demo`)

Status: built M7-27. Rules: [16-operations.md](../../docs/spec/16-operations.md) section 16; how to
run it: [docs/operations/demo.md](../../docs/operations/demo.md).

The made-up, white-label data of `bothy setup demo`: only the people Alex, Sam and Robin, the
spaces Support, Marketing, Engineering and Research, made-up agents and models, `example.com`
addresses and made-up texts. No secret, no real name, no real address; `make denylist` and
`make gitleaks` cover the folder.

| File | Content |
|---|---|
| `demo.yaml` | the data set: `values` (every size and wait of the demo), `people` (exactly one `owner`), `price_source` and `prices` of the made-up models, `agents` with their role (`worker`, `reviewer`, `watcher`), `spaces` with members (grants), channels (kind, topic, operator) and threads, the demo `host` with the session users, the warm-up `prompts` (text variants the seed picks from) and the `live` prompts whose runs stay open |
| `demo.schema.json` | its JSON Schema (2020-12); `bothy setup demo` validates the data set against it before it connects anywhere, then checks the references between the sections (exactly one owner, every person, agent, space, channel and model named exists, warm-up prompts go to channels with a `worker` operator, live prompts to a `reviewer` or `watcher`, every author and poster may write there) |

## Rules for changes

- Every size and wait is a member of `values`, never a literal in code (brief M7-27 rule 9).
- The same seed must give the same content: a change of the data set changes the demo of every
  seed, and `TestDemoDeterministic` compares two runs of one seed, not a stored export.
- A new channel with an operator needs a member grant of that agent on its space.
- `make demo-e2e` shows whether every view of 25 still has data after a change.
