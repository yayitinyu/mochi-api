import type { ComponentType } from 'react';
import {
  Anthropic,
  Claude,
  DeepSeek,
  Doubao,
  Gemini,
  Grok,
  Hunyuan,
  Meta,
  Minimax,
  Mistral,
  Moonshot,
  OpenAI,
  Qwen,
  Wenxin,
  Yi,
  Zhipu,
} from '@lobehub/icons';
import { SparkleIcon } from '@phosphor-icons/react';

type IconComponent = ComponentType<{ size?: number }>;

// Model name pattern -> official brand icon. First match wins.
const MODEL_ICONS: [RegExp, IconComponent][] = [
  [/claude/i, Claude.Color],
  [/gpt|^o[1345](-|$)|davinci|dall-e|whisper|text-embedding|chatgpt/i, OpenAI],
  [/gemini|gemma/i, Gemini.Color],
  [/deepseek/i, DeepSeek.Color],
  [/qwen|qwq|qvq/i, Qwen.Color],
  [/moonshot|kimi/i, Moonshot],
  [/grok/i, Grok],
  [/llama/i, Meta.Color],
  [/mistral|mixtral|codestral/i, Mistral.Color],
  [/glm|chatglm/i, Zhipu.Color],
  [/doubao/i, Doubao.Color],
  [/hunyuan/i, Hunyuan.Color],
  [/ernie|wenxin/i, Wenxin.Color],
  [/^yi-|01-ai/i, Yi.Color],
  [/minimax|abab/i, Minimax.Color],
];

export function ModelIcon({ name, size = 16 }: { name: string; size?: number }) {
  for (const [pattern, IconCmp] of MODEL_ICONS) {
    if (pattern.test(name)) {
      return (
        <span className="inline-flex shrink-0 items-center" aria-hidden>
          <IconCmp size={size} />
        </span>
      );
    }
  }
  return (
    <span className="inline-flex shrink-0 items-center text-sakura-400" aria-hidden>
      <SparkleIcon size={size} weight="duotone" />
    </span>
  );
}

// Channel type -> provider icon (openai | anthropic | gemini).
export function ProviderIcon({ type, size = 16 }: { type: string; size?: number }) {
  const IconCmp: IconComponent =
    type === 'anthropic' ? Anthropic : type === 'gemini' ? Gemini.Color : OpenAI;
  return (
    <span className="inline-flex shrink-0 items-center" aria-hidden>
      <IconCmp size={size} />
    </span>
  );
}
