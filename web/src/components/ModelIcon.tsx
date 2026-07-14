import type { ComponentType } from 'react';
import {
  AiHubMix,
  Anthropic,
  Baichuan,
  BaiduCloud,
  Claude,
  Cohere,
  DeepSeek,
  Doubao,
  Fireworks,
  Gemini,
  Github,
  Grok,
  Groq,
  Hunyuan,
  Kimi,
  LongCat,
  Meta,
  Minimax,
  Mistral,
  ModelScope,
  Moonshot,
  Novita,
  Nvidia,
  Ollama,
  OpenAI,
  OpenRouter,
  Perplexity,
  Qwen,
  SiliconCloud,
  Spark,
  Stepfun,
  Together,
  Wenxin,
  XiaomiMiMo,
  Yi,
  ZAI,
  Zhipu,
} from '@lobehub/icons';
import { SparkleIcon } from '@phosphor-icons/react';

type IconComponent = ComponentType<{ size?: number }>;
type BrandIcon = IconComponent & { Color?: IconComponent };

// Prefer the branded color variant when the icon set provides one.
function colorOf(icon: BrandIcon): IconComponent {
  return icon.Color ?? icon;
}

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

// Preset icon key -> official brand icon (see channelPresets.ts).
const CHANNEL_ICONS: Record<string, BrandIcon> = {
  openai: OpenAI,
  anthropic: Anthropic,
  gemini: Gemini,
  grok: Grok,
  mistral: Mistral,
  groq: Groq,
  together: Together,
  fireworks: Fireworks,
  perplexity: Perplexity,
  cohere: Cohere,
  nvidia: Nvidia,
  github: Github,
  deepseek: DeepSeek,
  kimi: Kimi,
  qwen: Qwen,
  zhipu: Zhipu,
  zai: ZAI,
  doubao: Doubao,
  qianfan: BaiduCloud,
  hunyuan: Hunyuan,
  stepfun: Stepfun,
  minimax: Minimax,
  baichuan: Baichuan,
  yi: Yi,
  spark: Spark,
  xiaomi: XiaomiMiMo,
  longcat: LongCat,
  openrouter: OpenRouter,
  siliconflow: SiliconCloud,
  modelscope: ModelScope,
  aihubmix: AiHubMix,
  novita: Novita,
  ollama: Ollama,
};

// ChannelIcon resolves a channel's icon: a preset key maps to a brand icon,
// an http(s) URL renders as an image, anything else falls back to the
// channel-type icon.
export function ChannelIcon({
  icon,
  type,
  size = 16,
}: {
  icon?: string;
  type: string;
  size?: number;
}) {
  if (icon && CHANNEL_ICONS[icon]) {
    const IconCmp = colorOf(CHANNEL_ICONS[icon]);
    return (
      <span className="inline-flex shrink-0 items-center" aria-hidden>
        <IconCmp size={size} />
      </span>
    );
  }
  if (icon && /^https?:\/\//.test(icon)) {
    return (
      <img
        src={icon}
        alt=""
        aria-hidden
        width={size}
        height={size}
        className="inline-block shrink-0 rounded-sm object-contain"
        onError={(e) => {
          // A broken URL degrades to an invisible box; hide it instead.
          (e.target as HTMLImageElement).style.display = 'none';
        }}
      />
    );
  }
  return <ProviderIcon type={type} size={size} />;
}
