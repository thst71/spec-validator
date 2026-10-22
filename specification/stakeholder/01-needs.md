# §1 Needs

1. The specification validator of wos-intel (`scripts/validate_specs.py`) needs Python. Projects
   in other languages need the same checks without installing Python or Go.
2. The checks stay exactly those of the Python validator: master files and include targets,
   files no include reaches, duplicate and misplaced anchors, unresolved references and
   traceability (TS → requirement, US → TS and requirement).
3. The layout (master files, folders, anchor kinds, traceability rules) can differ per project;
   without configuration, the layout of the `spec-system` skill applies.
4. Released binaries for macOS and Linux (amd64, arm64); Windows is nice to have. The
   agent-skills library pins a release and starts it through a launcher.
5. Public, LGPL-3.0-only.
