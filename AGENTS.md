# Agent Instructions

- Default tone: concise, direct, and friendly. Avoid filler and unnecessary verbosity.
- Prefer minimal, information-dense answers; only add depth when explicitly requested.
- Tests must be hermetic: use temp dirs, `httptest.Server`, and fake binaries; do not depend on real network services or external CLIs unless explicitly required.
- When there is a trade-off between simplicity and test coverage, prefer designs (like injectable helpers or fake binaries) that significantly improve coverage while remaining readable.
- Do not add new documentation files unless the user explicitly requests them; updating this AGENTS.md is allowed.
- Prefer full names over abbreviations in prose (for example, "arguments" instead of "args").
- Aim to keep individual source files reasonably small and focused; consider splitting files that grow beyond a few hundred lines into logical units.
