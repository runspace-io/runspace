'use client';

import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';

export function TerminalPanel({
  url,
  tokenSource,
}: {
  url: string | undefined;
  /** Gateway terminals need a token; host terminals are loopback and do not. */
  tokenSource?: (() => Promise<string>) | undefined;
}) {
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
    let pendingInput = '';
    let disposed = false;
    // Gateway terminals authenticate with a short-lived token that has to be
    // fetched first, so the socket appears a tick later. Input typed meanwhile
    // buffers, exactly as it did while the socket was CONNECTING before.
    let socket: WebSocket | undefined;
    terminal.write('\x1b[2mConnecting terminal…\x1b[0m\r\n');
    void (async () => {
      const token = await tokenSource?.();
      if (disposed) return;
      if (token) endpoint.searchParams.set('access_token', token);
      const opened = new WebSocket(endpoint);
      socket = opened;
      opened.onopen = () => {
        terminal.write('\x1b[32m$ connected\x1b[0m\r\n');
        if (pendingInput) {
          opened.send(JSON.stringify({ type: 'input', data: pendingInput }));
          pendingInput = '';
        }
        terminal.focus();
      };
      opened.onmessage = (message) => {
        try {
          const frame = JSON.parse(message.data) as { type?: string; data?: string };
          if (frame.data) {
            terminal.write(frame.type === 'error' ? `\x1b[31m${frame.data}\x1b[0m` : frame.data);
          }
        } catch {
          terminal.write(message.data);
        }
      };
      opened.onerror = () => {
        terminal.write('\r\n\x1b[31mTerminal connection failed.\x1b[0m\r\n');
      };
      opened.onclose = () => {
        if (!disposed) terminal.write('\r\n\x1b[2mTerminal disconnected.\x1b[0m\r\n');
      };
    })();
    terminal.onData((data) => {
      echoTerminalInput(terminal, data);
      const input = data.replaceAll('\r', '\n');
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'input', data: input }));
      } else {
        pendingInput += input;
      }
    });
    return () => {
      disposed = true;
      socket?.close();
      terminal.dispose();
    };
  }, [url, tokenSource]);
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
