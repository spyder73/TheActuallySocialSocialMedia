import type { AiMode } from "@app/shared";

const FACTCHECK_SYSTEM_PROMPT =
  "You are a careful fact-checking assistant for a social media platform. " +
  "You are given the context of a post (author, date, and text). Identify " +
  "verifiable factual claims and assess their accuracy to the best of your ability. " +
  "Point out missing context or nuance, and clearly state any uncertainty " +
  "when you are unsure. Respond concisely and neutrally, independent of " +
  "political viewpoint. Respond in English.";

const EXPLAIN_SYSTEM_PROMPT =
  "You are a helpful assistant that explains social media posts and " +
  "clarifies terminology, allusions, and historical or social " +
  "context needed to understand them. Respond concisely, neutrally, " +
  "and informatively, in English.";

const CUSTOM_BASE_SYSTEM_PROMPT =
  "You are an assistant on a social media platform helping a " +
  "user understand a specific post. You are given the post context " +
  "and the user’s instruction or question about it. Follow the user’s " +
  "instruction as well as possible, in relation to the post provided.";

export const FACTCHECK_SUMMARY_SYSTEM_PROMPT =
  "You receive several independent fact-check results for the same social media post " +
  "from different users (possibly produced by different AI models). " +
  "Summarize them in 1–3 concise, neutral sentences: where they agree, " +
  "where they differ. Do not repeat each result; give only the " +
  "essence. Respond in English.";

export function systemPromptForMode(mode: AiMode): string {
  switch (mode) {
    case "factcheck":
      return FACTCHECK_SYSTEM_PROMPT;
    case "explain":
      return EXPLAIN_SYSTEM_PROMPT;
    case "custom":
      return CUSTOM_BASE_SYSTEM_PROMPT;
  }
}

export interface PostContext {
  authorUsername: string;
  authorDisplayName: string | null;
  createdAt: Date;
  text: string | null;
  hasImage: boolean;
}

export function buildPostContextBlock(post: PostContext): string {
  const lines = [
    "Post context:",
    `Author: @${post.authorUsername}${post.authorDisplayName ? ` (${post.authorDisplayName})` : ""}`,
    `Posted: ${post.createdAt.toISOString()}`,
    `Text: "${post.text ?? "(no text)"}"`,
  ];
  if (post.hasImage) {
    lines.push("Note: The post also contains an image that you cannot see.");
  }
  return lines.join("\n");
}

export function buildInitialUserMessage(
  mode: AiMode,
  post: PostContext,
  customPrompt?: string
): string {
  const context = buildPostContextBlock(post);
  if (mode === "custom") {
    return `${context}\n\nUser request: ${customPrompt}`;
  }
  return context;
}
