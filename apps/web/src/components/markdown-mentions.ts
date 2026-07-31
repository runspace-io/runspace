type MarkdownNode = {
  type: string;
  value?: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: MarkdownNode[];
};

export function rehypeMentions() {
  return (tree: MarkdownNode) => decorateMentions(tree, false);
}

function decorateMentions(node: MarkdownNode, blocked: boolean): void {
  const nextBlocked = blocked || ['a', 'code', 'pre'].includes(node.tagName ?? '');
  if (!node.children) return;
  node.children = node.children.flatMap((child) => {
    if (child.type === 'text' && child.value && !nextBlocked) return mentionNodes(child.value);
    decorateMentions(child, nextBlocked);
    return child;
  });
}

function mentionNodes(value: string): MarkdownNode[] {
  const nodes: MarkdownNode[] = [];
  const pattern = /@[\w]+(?:[.-][\w]+)*/g;
  let cursor = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > 0 && /[\w@]/.test(value[index - 1] ?? '')) continue;
    if (index > cursor) nodes.push({ type: 'text', value: value.slice(cursor, index) });
    nodes.push({
      type: 'element',
      tagName: 'span',
      properties: { className: ['chat-mention'] },
      children: [{ type: 'text', value: match[0] }],
    });
    cursor = index + match[0].length;
  }
  if (cursor < value.length) nodes.push({ type: 'text', value: value.slice(cursor) });
  return nodes.length ? nodes : [{ type: 'text', value }];
}
