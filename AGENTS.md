# Workspace Guidelines & Rules

spec-validator checks modular AsciiDoc specifications (includes, anchors, references, traceability). This file holds the project's facts and rules; the workflows are skills in `.agents/skills/`.

## How to Work Here

1. **Environment**: Before the first terminal command, apply [environment](.agents/skills/environment/SKILL.md).
2. **Skills**: Before a matching task, read the skill and follow it. Load the referenced skills and profiles only when a step needs them.

| Task | Skill |
|---|---|
| New feature, from idea to backlog | [create-new-feature](.agents/skills/create-new-feature/SKILL.md) |
| Requirement / tech specification / user story | [create-requirement](.agents/skills/create-requirement/SKILL.md), [create-tech-specification](.agents/skills/create-tech-specification/SKILL.md), [create-user-story](.agents/skills/create-user-story/SKILL.md) |
| Change of existing specification items | [update-feature](.agents/skills/update-feature/SKILL.md), [update-requirement](.agents/skills/update-requirement/SKILL.md), [update-user-story](.agents/skills/update-user-story/SKILL.md) |
| Implement a story | [implement-user-story](.agents/skills/implement-user-story/SKILL.md) |
| Fix a defect | [fix-bug](.agents/skills/fix-bug/SKILL.md) |
| Done check | [definition-of-done](.agents/skills/definition-of-done/SKILL.md) |
| Commit | [commit](.agents/skills/commit/SKILL.md) |
| Release | [release](.agents/skills/release/SKILL.md) |
| Commands, test tooling | [toolchain](.agents/skills/toolchain/SKILL.md) |
| Specification layout and rules | [spec-system](.agents/skills/spec-system/SKILL.md) |

## Precedence

1. This file.
2. The project skills in `.agents/skills/`.
3. Personal instructions and skills of the user (e.g. `~/.claude/skills`, `~/.agents/`). They may only cover the machine environment and how to communicate; they never override a project rule or a project skill of the same purpose.

`.agents/skills/` is the vendored agent-skills library (version in `.agents/skills/VERSION`); adapt it here, under "Project Facts", never by editing it. Update it with `.agents/skills/vendor.sh v<version>`, then commit `chore(agents): vendor agent-skills v<version>`.

---

## Project Facts

### Components

| Path | Toolchain profile |
|---|---|
| `.` (`cmd/`, `internal/`) | [go-task](.agents/skills/toolchain/go-task/PROFILE.md) |

The contract tasks are defined in `Taskfile.yml`. Deviation from the profile: `validate-specs` runs this repository's validator from source (`go run ./cmd/spec-validator`), not the library's launcher, which would run a pinned release of this very tool.

### Test Pyramid Limits

- Unit tests (`internal/spec`) build document trees in `t.TempDir()`.
- Component tests drive `run` in `cmd/spec-validator` in-process.
- The render chain test (`internal/spec/render_test.go`) renders a project with both reports in the Asciidoctor image of release-actions. It needs Docker and skips itself without it or with `-short`. Its work folder is a unique `os.MkdirTemp` folder below `$HOME/.cache`, because Docker may share only the home directory (Colima), and its cleanup removes only that folder.
- Blackbox tests of a release (download, checksum, `-version`) are manual.

### Releases

Releases follow the [release](.agents/skills/release/SKILL.md) skill: a signed tag `v<semver>`, pushed by the user, runs `.github/workflows/release.yml`. A release changes nothing in agent-skills: each project pins a validator release in its `.agents/spec-validator.lock` and raises it with `validate-specs --update`. The archive names, the archive layout, `checksums.txt` and the `-version` output are the interface of that launcher (FR-2.1); changing one of them is a major release. Vendoring a new agent-skills version calls for no release of its own; it ships with the next one.

### Commit Scopes

`spec` (validation rules), `config`, `cli`, `agents`; workflows and GoReleaser without scope.

---

## Self-Correction & Debugging Protocol

- **Do Not Guess**: Read stack traces and error messages carefully before editing code.
- **Isolate**: Reproduce defects with a minimal automated test before modifying code.
- **Limit Loops**: If a test or build fails more than 3 times for the same reason, stop and ask the user for guidance.
- **Zero Unapproved Dependencies**: The validator uses the Go standard library only; do not add modules unless specified in the technical specification or requested by the user.
