import { useState } from "react";
import type { FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { AiProviderConfigPublic, AiProviderType } from "@app/shared";
import { AI_PROVIDER_TYPES } from "@app/shared";
import { apiFetch } from "../lib/api.js";
import NavBar from "../components/NavBar.js";
import PageHeader from "../components/PageHeader.js";
import { card, input, btnPrimary, btnSecondary, btnDanger } from "../lib/ui.js";

async function fetchProviders(): Promise<AiProviderConfigPublic[]> {
  const res = await apiFetch<{ providers: AiProviderConfigPublic[] }>(
    "/ai/providers"
  );
  return res.providers;
}

export default function AiSettingsPage() {
  const queryClient = useQueryClient();
  const { data: providers = [] } = useQuery({
    queryKey: ["ai-providers"],
    queryFn: fetchProviders,
  });

  const [label, setLabel] = useState("");
  const [type, setType] = useState<AiProviderType>("openai_compatible");
  const [baseUrl, setBaseUrl] = useState("");
  const [model, setModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [isDefault, setIsDefault] = useState(false);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    await apiFetch("/ai/providers", {
      method: "POST",
      body: JSON.stringify({
        label,
        type,
        baseUrl,
        model,
        apiKey: apiKey || undefined,
        isDefault,
      }),
    });
    setLabel("");
    setBaseUrl("");
    setModel("");
    setApiKey("");
    setIsDefault(false);
    await queryClient.invalidateQueries({ queryKey: ["ai-providers"] });
  }

  async function onDelete(id: string) {
    await apiFetch(`/ai/providers/${id}`, { method: "DELETE" });
    await queryClient.invalidateQueries({ queryKey: ["ai-providers"] });
  }

  async function onSetDefault(id: string) {
    await apiFetch(`/ai/providers/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ isDefault: true }),
    });
    await queryClient.invalidateQueries({ queryKey: ["ai-providers"] });
  }

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editBaseUrl, setEditBaseUrl] = useState("");
  const [editModel, setEditModel] = useState("");
  const [editApiKey, setEditApiKey] = useState("");

  function startEdit(p: AiProviderConfigPublic) {
    setEditingId(p.id);
    setEditBaseUrl(p.baseUrl);
    setEditModel(p.model);
    setEditApiKey("");
  }

  async function saveEdit(id: string) {
    await apiFetch(`/ai/providers/${id}`, {
      method: "PATCH",
      body: JSON.stringify({
        baseUrl: editBaseUrl,
        model: editModel,
        apiKey: editApiKey || undefined,
      }),
    });
    setEditingId(null);
    await queryClient.invalidateQueries({ queryKey: ["ai-providers"] });
  }

  return (
    <div className="mx-auto max-w-lg px-4 py-8">
      <PageHeader title="Fact-checking AI" />
      <NavBar />
      <p className="mb-4 text-sm text-gray-500">
        Configure your own provider, such as OpenAI, a local Ollama model (OpenAI-compatible API), or Anthropic. API keys are encrypted at rest.
      </p>

      <div className="mb-6 flex flex-col gap-2">
        {providers.map((p) => (
          <div key={p.id} className={`${card} text-sm`}>
            <div className="flex items-center justify-between">
              <div>
                <span className="font-medium">{p.label}</span>{" "}
                <span className="text-gray-500">({p.type}, {p.model})</span>
                {p.isDefault && (
                  <span className="ml-2 rounded-full bg-yellow-100 px-2 py-0.5 text-xs">Default</span>
                )}
              </div>
              <div className="flex gap-3 text-xs">
                {!p.isDefault && (
                  <button onClick={() => void onSetDefault(p.id)} className="underline">
                    Set as default
                  </button>
                )}
                <button onClick={() => startEdit(p)} className="underline">
                  Edit
                </button>
                <button onClick={() => void onDelete(p.id)} className={btnDanger}>
                  Delete
                </button>
              </div>
            </div>
            {editingId === p.id && (
              <div className="mt-3 flex flex-col gap-2 border-t border-gray-100 pt-3">
                <input
                  value={editBaseUrl}
                  onChange={(e) => setEditBaseUrl(e.target.value)}
                  placeholder="Base URL"
                  className={`${input} text-xs`}
                />
                <input
                  value={editModel}
                  onChange={(e) => setEditModel(e.target.value)}
                  placeholder="Model name"
                  className={`${input} text-xs`}
                />
                <input
                  value={editApiKey}
                  onChange={(e) => setEditApiKey(e.target.value)}
                  placeholder="New API key (leave blank to keep unchanged)"
                  type="password"
                  className={`${input} text-xs`}
                />
                <div className="flex justify-end gap-2">
                  <button onClick={() => setEditingId(null)} className={btnSecondary}>
                    Cancel
                  </button>
                  <button onClick={() => void saveEdit(p.id)} className={btnPrimary}>
                    Save
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
        {providers.length === 0 && (
          <p className="text-sm text-gray-400">No provider configured yet.</p>
        )}
      </div>

      <form onSubmit={(e) => void onCreate(e)} className={`${card} flex flex-col gap-2`}>
        <input
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          placeholder="Name (e.g. 'My local Llama')"
          className={input}
          required
        />
        <select
          value={type}
          onChange={(e) => setType(e.target.value as AiProviderType)}
          className={input}
        >
          {AI_PROVIDER_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
        <input
          value={baseUrl}
          onChange={(e) => setBaseUrl(e.target.value)}
          placeholder="Base URL (e.g. http://localhost:11434/v1 for Ollama)"
          className={input}
          required
        />
        <input
          value={model}
          onChange={(e) => setModel(e.target.value)}
          placeholder="Model name (e.g. llama3, gpt-4o-mini, claude-3-5-sonnet)"
          className={input}
          required
        />
        <input
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="API key (leave blank for local Ollama without authentication)"
          type="password"
          className={input}
        />
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={isDefault}
            onChange={(e) => setIsDefault(e.target.checked)}
          />
          Use as default
        </label>
        <button className="self-end rounded bg-black px-4 py-2 text-sm text-white">
          Add
        </button>
      </form>
    </div>
  );
}
