# spec-validator

Validates a modular AsciiDoc specification as described by the `spec-system` skill of
[agent-skills](https://github.com/thst71/agent-skills): one static binary, no Python or Go needed.

It checks that

- master files and every `include::` target exist (nested includes resolve relative to the including file),
- every `.adoc` file below a master's folder is reached by an include,
- anchors (`[#FR-1.1]`) are unique and live in the folder of their kind,
- every reference (`<<FR-1.1>>`) resolves,
- items trace to their parents: each TS references a requirement, each US a TS and a requirement.

## Usage

```sh
spec-validator [-config <file>] [project-root]
```

| Exit code | Meaning |
|---|---|
| 0 | `specifications are valid` |
| 1 | problems, one per line on stderr, then `<n> problem(s) found` |
| 2 | usage, configuration or read error |

In projects using agent-skills, run it through the launcher `.agents/skills/spec-system/bin/validate-specs`: it downloads the pinned release once, verifies its checksum and runs it.

## Installation

- Download an archive from the [releases](https://github.com/thst71/spec-validator/releases) and check it against `checksums.txt`, or
- `go install github.com/thst71/spec-validator/cmd/spec-validator@latest`.

macOS and Linux are the targets. Windows binaries are built; Windows is nice to have:

> "Is Windows a real target or just nice to have?" – Claude
>
> "Windows is nice to have, quote me quoting you in the readme :-)" – Thomas Strauß

## Configuration

Without configuration, the layout of the `spec-system` skill applies. A `.spec-validator.json` in the project root (or `-config <file>`) replaces it:

```json
{
  "documents": [
    {"master": "specification-web.adoc", "folder": "specification/requirements", "kinds": ["FR", "NFR"]},
    {"master": "tech-specification-web.adoc", "folder": "specification/tech-specification", "kinds": ["TS"]},
    {"master": "backlog-web.adoc", "folder": "specification/backlog", "kinds": ["US"]},
    {"master": "release-notes.adoc", "folder": "specification/release-notes", "kinds": []}
  ],
  "traceability": [
    {"kind": "TS", "requires": ["FR", "NFR"], "name": "requirement"},
    {"kind": "US", "requires": ["FR", "NFR"], "name": "requirement"},
    {"kind": "US", "requires": ["TS"], "name": "technical specification"}
  ]
}
```

A traceability rule requires every item of `kind` to reference at least one item of the `requires` kinds; `name` appears in the message.

## Development

This repository follows the agent-skills workflow, vendored into `.agents/skills/`; see [AGENTS.md](AGENTS.md). The quality gate is `task check` ([go-task](https://taskfile.dev) and `golangci-lint` required). A tag `v<semver>` publishes the release.

## License

[LGPL-3.0-only](COPYING.LESSER) (which builds on the [GPL-3.0](COPYING)).
