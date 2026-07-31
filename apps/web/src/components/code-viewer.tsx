'use client';

import Editor from '@monaco-editor/react';
import type { ApiRepositoryFile } from '@/lib/api-client';

export function CodeViewer({ file }: { file?: ApiRepositoryFile | undefined }) {
  if (!file) {
    return (
      <section className="code-viewer code-viewer-empty" aria-label="Code preview">
        Select a resource file to inspect it.
      </section>
    );
  }
  return (
    <section className="code-viewer" aria-label="Code preview">
      <div className="code-viewer-header">
        <span>{file.path}</span>
        <span className="file-type">READ ONLY</span>
      </div>
      <Editor
        height="180px"
        language={languageForPath(file.path)}
        value={file.content}
        theme="vs-dark"
        options={{
          minimap: { enabled: false },
          readOnly: true,
          lineNumbers: 'on',
          padding: { top: 10 },
        }}
      />
    </section>
  );
}

export function languageForPath(path: string): string {
  const extension = path.split('.').pop()?.toLowerCase();
  return (
    {
      css: 'css',
      go: 'go',
      html: 'html',
      js: 'javascript',
      json: 'json',
      md: 'markdown',
      ts: 'typescript',
      tsx: 'typescript',
      yaml: 'yaml',
      yml: 'yaml',
    }[extension ?? ''] ?? 'plaintext'
  );
}
