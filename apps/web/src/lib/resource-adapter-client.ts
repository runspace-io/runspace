export type CapabilityDescriptor = {
  id: string;
  label: string;
  description: string;
  mode: 'query';
  risk: 'read';
};

export type ResourceAdapter = {
  manifest: {
    id: 'github-cli' | 'postgresql' | 'digitalocean-cli';
    name: string;
    description: string;
    executable: string;
    resource_type: string;
    capabilities: CapabilityDescriptor[];
  };
  status: 'ready' | 'not_installed';
  path?: string;
};

export type LocalCapabilityResource = {
  id: string;
  adapter_id: ResourceAdapter['manifest']['id'];
  title: string;
  profile: string;
  workspace_id: string;
  capabilities: CapabilityDescriptor[];
};

const hostAgentURL =
  process.env.NEXT_PUBLIC_HOST_AGENT_URL?.replace(/\/$/, '') ?? 'http://127.0.0.1:7799';

export async function discoverResourceAdapters(userID: string): Promise<ResourceAdapter[]> {
  const result = await request<{ adapters: ResourceAdapter[] }>('/v1/resource-adapters', userID);
  return result.adapters ?? [];
}

export function connectCapabilityResource(input: {
  userID: string;
  workspaceID: string;
  adapterID: ResourceAdapter['manifest']['id'];
  title: string;
  profile: string;
}): Promise<LocalCapabilityResource> {
  const gatewayURL = new URL(process.env.NEXT_PUBLIC_API_URL ?? '/gateway', window.location.origin)
    .toString()
    .replace(/\/$/, '');
  return request('/v1/capability-resources', input.userID, {
    adapter_id: input.adapterID,
    title: input.title,
    profile: input.profile,
    workspace_id: input.workspaceID,
    gateway_url: gatewayURL,
  });
}

async function request<T>(path: string, userID: string, body?: object): Promise<T> {
  let response: Response;
  try {
    const init: NonNullable<Parameters<typeof fetch>[1]> = {
      method: body ? 'POST' : 'GET',
      headers: { 'Content-Type': 'application/json', 'X-User-ID': userID },
    };
    if (body) init.body = JSON.stringify(body);
    response = await fetch(`${hostAgentURL}${path}`, init);
  } catch {
    throw new Error('Runspace Host Agent is not reachable.');
  }
  const payload = (await response.json().catch(() => ({}))) as T & { error?: string };
  if (!response.ok) throw new Error(payload.error ?? `Host Agent returned ${response.status}.`);
  return payload;
}
