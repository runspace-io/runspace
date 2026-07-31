# Interactive artifacts

Runspace agents present work through versioned declarative documents rather
than executable UI code.

```text
Agent -> runspace.ui/v1 document -> validator -> Resource Graph -> trusted renderer
```

## Initial registry

The approved components are `Markdown`, `CodeReference`, `DiffViewer`,
`TaskCard`, `TestReport`, `Timeline`, `DataTable`, `MetricGroup`,
`ApprovalRequest`, and `D3Artifact`. Documents compose them with `Stack` and
`Grid` layouts.

Agents discover the registry through `ui.list_components`, inspect one schema
with `ui.get_component_schema`, and publish with `ui.create_artifact`. Created
artifacts are graph nodes and are shared into the connected discussion using a
live `[[ui:artifact:...]]` reference.

## Safety

- Documents allow at most eight levels, 100 components, and 64 KiB of props per
  component.
- Tables, timelines, graph nodes, and graph edges have explicit count limits.
- Unknown component types, including raw JavaScript, are rejected.
- Markdown is rendered with GFM and an HTML sanitization pass.
- `D3Artifact` runs trusted Runspace D3 layout code over declarative node/edge
  data; no agent code reaches the DOM.
- Interactive operations call `ui.request_action`, which creates a pending
  policy node. The visualization cannot execute the operation itself.

## Composer

The channel composer stores plain Markdown and supports formatting controls,
multi-line editing, keyboard send, workspace Resource insertion, and lazy
Resource suggestions. Resource references remain structured and clickable
instead of becoming copied text.

## Next extensions

Mermaid, math, inline diff hydration, syntax tokenization, and an isolated custom
visualization worker can extend the same document boundary. Custom renderers
must retain CSP, resource budgets, no ambient credentials, no unrestricted
network, and a controlled message bridge.
