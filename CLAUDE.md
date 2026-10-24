# CLAUDE.md

@AGENTS.md

## Claude-specific tooling

### Code Exploration Policy

Always use jCodeMunch-MCP for code navigation. Never fall back to Read, Grep, Glob, or Bash for code exploration.
**Exception:** use `Read` when you are about to edit a file — the harness requires a `Read` before `Edit`/`Write`. Use jCodeMunch to _find and understand_ code, then `Read` only the file you are changing.

This server runs the **front door** surface: three tools reach every jCodeMunch capability, so the tool list stays small and the catalogue is fetched only when you need it.

**Start any session:**

1. `order { "action": "resolve_repo", "args": { "path": "." } }` — confirm the project is indexed. If it is not: `order { "action": "index_folder", "args": { "path": "." } }`

**Then, for any task:**

- Know what you want → `order { "action": "<name>", "args": { ... } }`
- Know the goal, not the tool → `route { "query": "your task in a sentence" }` picks the action and shapes the arguments
- Want to see what exists → `menu { "query": "what you are trying to do" }` returns matching actions with example arguments
- Want the whole catalogue and the usage rules → `jcodemunch_guide`

`menu` and `jcodemunch_guide` list every action this server can run, including ones absent from your tool list. That is expected: the front door is the way to call them.

**Interpreting results:**

- A search proves absence only when the scan is citable: `_meta.verdict.evidence_ref` holds an `absent:` token, or, where `_meta.verdict` is not shown, `_meta.absence_evidence.citable` is `true`. Then report the gap; do not re-search with different wording. A `no_implementation_found` verdict, a `state` of `absent` or an empty result is not proof on its own: read the note or `absence_blocked_by`, re-index if it names the index, then search again.
- `source: ""` alongside `source_status` means the body could not be read, not that the symbol is empty.

**After editing files:**

- With PostToolUse hooks installed (Claude Code), edited files are reindexed automatically.
- Otherwise `order { "action": "register_edit", "args": { "paths": [...] } }` after an edit, batched for bulk changes.

### Documentation Exploration Policy

Always use jDocMunch-MCP for reading project docs (`.md`, `.yaml`, `.json` specs, plans). Never fall back to Read, Grep, Glob, or Bash to browse documentation.
**Exception:** use `Read` when you are about to edit a doc — the harness requires a `Read` before `Edit`/`Write`.

**Start any session:**

1. `doc_resolve_repo { "path": "." }` — confirm the docs are indexed (repo id is `local/AmgReports`). If not: `index_local { "path": "." }`

**Then, for any docs question:**

- `search_sections` — find the section that answers a question, ranked
- `get_document_outline` — before opening any doc file
- `get_section` / `get_section_excerpt` — read one section by id instead of the whole file
- `get_section_context` — a section plus what surrounds it
- `lookup_term` — what a project-specific word means here

**After editing docs:** re-run `index_local` so the index is not stale (the index reports `source_dirty: true` when it is behind the working tree).

### Output Rules (adapted from jOutputMunch)

Source: https://github.com/jgravelle/jOutputMunch

- Lead with the answer. Avoid preambles and restating the question.
- Use contractions when natural.
- Avoid filler, unnecessary openers, and closers.
- Keep answers concise. Avoid repetition and hedge-stacking.
- Explain non-obvious decisions and important domain logic.
- Provide detailed explanations when requested.
- Avoid narrating routine tool calls and searches.
- Give brief progress updates during longer tasks.
- After code changes, summarize changes and verification results.
- Report failures, unresolved issues, and important limitations.
- Show changed code with surrounding context only when useful.
- Reference files, line numbers, or symbols instead of repeating
  large tool outputs.
- Use compact JSON when appropriate. Preserve required schemas
  and meaningful empty fields, null values, and counts.
- These rules must not override user requests, project requirements,
  security guidance, testing, or required workflows.

## Claude-specific workflow

- Do not disable chrome-devtools-mcp.
- Stop the dev server in a separate step before building.
- Show the full diff before committing.

### Obsidian vault upkeep

When a feature is finished or a session ends, update the Obsidian vault through the `obsidian` MCP without asking first. Do all three:

1. **Master Dashboard:** `Projects/AmgReports Master Dashboard.md`: update the feature's status and links.
2. **Session Log:** add or update `Daily/AmgReports-Session-Summary-YYYY-MM-DD.md` (YAML frontmatter, `[[wikilinks]]` to the feature notes).
3. **Decision log:** record any new decision in the feature note and link it from the dashboard. The repo's `docs/DECISIONS.md` stays the source of truth.

Use `[[wikilinks]]` and YAML frontmatter, prefer `obsidian_append` to existing notes, and link feature notes back to the dashboard and the session summary. Vault notes never hold secrets, passwords or `.env` values. This rule does not allow commits or pushes.

## Token Saving Rules

- When running commands that produce large output (tests, logs, git diff), always pipe through `| tail -c 4000` or `| head -c 4000` to limit context.
- Do not run the full test suite (`go test ./...` or `npm test`) unless explicitly asked. Run specific tests only.
