# §1 Needs

1. The specification validator of wos-intel (`scripts/validate_specs.py`) needs Python. Projects
   in other languages need the same checks without installing Python or Go.
2. The checks stay exactly those of the Python validator: master files and include targets,
   files no include reaches, duplicate and misplaced anchors, unresolved references and
   traceability (TS → requirement, US → TS and requirement).
3. The layout (master files, folders, anchor kinds, traceability rules) can differ per project;
   without configuration, the layout of the `spec-system` skill applies.
4. Released binaries for macOS and Linux (amd64, arm64); Windows is nice to have. Each
   project pins a release; the launcher of the agent-skills library verifies and starts it.
5. Public, LGPL-3.0-only.
6. Asciidoctor reads `[#FR-1.2]` as the id `FR-1` with the role `2`, so all items `FR-1.x`
   share one id in the rendered documents. Anchors are written `[[FR-1.2]]`; the validator
   rejects the shorthand and can rewrite existing documents.
7. Releases attach every master as one HTML file. References to items of another master must
   become links between these files; the release pipeline flattens each master with
   `asciidoctor-reducer` and needs the masters and the links from the validator.
