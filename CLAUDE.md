## Commits

- Prefix the subject line with `ai:` when the commit's subject is AI-related technical work — e.g. agent configs, prompts, AI tooling/workflows. This is a **topical** prefix: it marks the *work being committed* as AI-related, not every commit an AI happens to make. Commits that aren't about AI use no special prefix.

## Agent skills

### Issue tracker

Issues live as markdown files under `.scratch/<feature>/` in this repo — no GitHub/GitLab issues, no `gh`/`glab`. See `docs/agents/issue-tracker.md`.

### Triage labels

Triage state is recorded as a `Status:` line near the top of each issue file, using the five canonical role names as-is (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
