'use client';

import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';

export function TerminalPanel({ url }: { url: string | undefined }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!host.current) return;
    const terminal = new Terminal({
      convertEol: true,
      cursorBlink: false,
      rows: 5,
      theme: { background: '#101318' },
    });
    terminal.open(host.current);
    if (!url) {
      terminal.write('$ git status --short\r\n  connect a resource to open a terminal\r\n');
      return () => terminal.dispose();
    }
    const endpoint = new URL(url);
    endpoint.searchParams.set('command', 'sh');
    const socket = new WebSocket(endpoint);
    let pendingInput = '';
    let disposed = false;
    terminal.write('\x1b[2mConnecting terminal…\x1b[0m\r\n');
    socket.onopen = () => {
      terminal.write('\x1b[32m$ connected\x1b[0m\r\n');
      if (pendingInput) {
        socket.send(JSON.stringify({ type: 'input', data: pendingInput }));
        pendingInput = '';
      }
      terminal.focus();
    };
    socket.onmessage = (message) => {
      try {
        const frame = JSON.parse(message.data) as { type?: string; data?: string };
        if (frame.data) {
          terminal.write(frame.type === 'error' ? `\x1b[31m${frame.data}\x1b[0m` : frame.data);
        }
      } catch {
        terminal.write(message.data);
      }
    };
    socket.onerror = () => {
      terminal.write('\r\n\x1b[31mTerminal connection failed.\x1b[0m\r\n');
    };
    socket.onclose = () => {
      if (!disposed) terminal.write('\r\n\x1b[2mTerminal disconnected.\x1b[0m\r\n');
    };
    terminal.onData((data) => {
      echoTerminalInput(terminal, data);
      const input = data.replaceAll('\r', '\n');
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'input', data: input }));
      } else if (socket.readyState === WebSocket.CONNECTING) {
        pendingInput += input;
      }
    });
    return () => {
      disposed = true;
      socket.close();
      terminal.dispose();
    };
  }, [url]);
  return (
    <section className="terminal-panel" aria-label="Agent terminal">
      <div ref={host} />
    </section>
  );
}

function echoTerminalInput(terminal: Terminal, data: string) {
  if (data === '\r') {
    terminal.write('\r\n');
    return;
  }
  if (data === '\u007f') {
    terminal.write('\b \b');
    return;
  }
  terminal.write(data);
}
