# spec-validator

Validates a modular AsciiDoc specification as described by the `spec-system` skill of
[agent-skills](https://github.com/thst71/agent-skills): one static binary, no Python or Go needed.

It checks that

- master files and every `include::` target exist (nested includes resolve relative to the including file),
- every `.adoc` file below a master's folder is reached by an include,
- anchors (`[[FR-1.1]]`) are unique and live in the folder of their kind; the shorthand `[#FR-1.1]` is rejected, because Asciidoctor reads the dot as a role and gives `FR-1.1` and `FR-1.2` the same id `FR-1`,
- every reference (`<<FR-1.1>>`) resolves,
- items trace to their parents: each TS references a requirement, each US a TS and a requirement.

## Usage

```sh
spec-validator [-config <file>] [project-root]              # validate
spec-validator -fix [project-root]                          # rewrite [#ID] to [[ID]], then validate
spec-validator -masters [project-root]                      # list the master files
spec-validator -link <flattened-dir> [project-root]         # link references across masters
```

`-masters` and `-link` serve the release pipeline: each master is flattened with `asciidoctor-reducer` into `<flattened-dir>`, `-link` turns references to items of another master into `xref:other-master.adoc#ID[ID]`, and Asciidoctor renders one HTML file per master whose links point to each other.

| Exit code | Meaning |
|---|---|
| 0 | `specifications are valid` |
| 1 | problems, one per line on stderr, then `<n> problem(s) found` |
| 2 | usage, configuration or read error |

In projects using agent-skills, run it through the launcher `.agents/skills/spec-system/bin/validate-specs`: it runs the release the project pins in `.agents/spec-validator.lock` (an installed `spec-validator` of that version, or the release downloaded once and verified against the lock). `validate-specs --update [v<version>]` pins a release (agent-skills v0.4.0 or later).

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

This repository follows the agent-skills workflow, vendored into `.agents/skills/`; see [AGENTS.md](AGENTS.md). The quality gate is `task check` ([go-task](https://taskfile.dev) and `golangci-lint` required). Releases come from signed tags as the agent-skills `release` skill describes; each release carries the binaries, the documentation as HTML and `checksums.txt`.

## License

[LGPL-3.0-only](COPYING.LESSER) (which builds on the [GPL-3.0](COPYING)).
