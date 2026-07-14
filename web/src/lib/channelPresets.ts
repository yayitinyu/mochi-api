// Official channel presets. Selecting one pre-fills name, type, base URL and
// icon so the user only needs to paste an API key.
//
// Base URL path conventions (mirrored by the relay):
//   - no trailing marker: standard version path is appended (/v1/...)
//   - trailing "/":  the URL is a complete API prefix, only the endpoint
//     leaf is appended (chat/completions, messages, ...)
//   - trailing "#":  the URL is the exact endpoint, used as-is
export type PresetCategory = 'global' | 'cn' | 'aggregator';

export interface ChannelPreset {
  id: string;
  label: string;
  type: 'openai' | 'anthropic' | 'gemini';
  baseUrl: string;
  category: PresetCategory;
  /** Icon key resolved by ChannelIcon; falls back to the type icon. */
  icon: string;
  hint?: string;
}

export const PRESET_CATEGORY_LABELS: Record<PresetCategory, string> = {
  global: '国际官方',
  cn: '国内官方',
  aggregator: '聚合与本地',
};

export const CHANNEL_PRESETS: ChannelPreset[] = [
  // --- 国际官方 ---
  { id: 'openai', label: 'OpenAI', type: 'openai', baseUrl: 'https://api.openai.com', category: 'global', icon: 'openai' },
  { id: 'anthropic', label: 'Anthropic', type: 'anthropic', baseUrl: 'https://api.anthropic.com', category: 'global', icon: 'anthropic' },
  { id: 'gemini', label: 'Google Gemini', type: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com', category: 'global', icon: 'gemini' },
  { id: 'xai', label: 'xAI (Grok)', type: 'openai', baseUrl: 'https://api.x.ai', category: 'global', icon: 'grok' },
  { id: 'mistral', label: 'Mistral', type: 'openai', baseUrl: 'https://api.mistral.ai', category: 'global', icon: 'mistral' },
  { id: 'groq', label: 'Groq', type: 'openai', baseUrl: 'https://api.groq.com/openai', category: 'global', icon: 'groq' },
  { id: 'together', label: 'Together AI', type: 'openai', baseUrl: 'https://api.together.xyz', category: 'global', icon: 'together' },
  { id: 'fireworks', label: 'Fireworks AI', type: 'openai', baseUrl: 'https://api.fireworks.ai/inference', category: 'global', icon: 'fireworks' },
  {
    id: 'perplexity',
    label: 'Perplexity',
    type: 'openai',
    baseUrl: 'https://api.perplexity.ai/',
    category: 'global',
    icon: 'perplexity',
    hint: '接口不带 /v1 前缀，末尾斜杠表示完整 API 前缀',
  },
  { id: 'cohere', label: 'Cohere', type: 'openai', baseUrl: 'https://api.cohere.ai/compatibility', category: 'global', icon: 'cohere' },
  { id: 'nvidia', label: 'NVIDIA NIM', type: 'openai', baseUrl: 'https://integrate.api.nvidia.com', category: 'global', icon: 'nvidia' },
  {
    id: 'github-models',
    label: 'GitHub Models',
    type: 'openai',
    baseUrl: 'https://models.github.ai/inference/',
    category: 'global',
    icon: 'github',
    hint: '接口不带 /v1 前缀，使用 GitHub PAT 作为密钥',
  },

  // --- 国内官方 ---
  { id: 'deepseek', label: 'DeepSeek', type: 'openai', baseUrl: 'https://api.deepseek.com', category: 'cn', icon: 'deepseek' },
  {
    id: 'deepseek-anthropic',
    label: 'DeepSeek (Claude 格式)',
    type: 'anthropic',
    baseUrl: 'https://api.deepseek.com/anthropic',
    category: 'cn',
    icon: 'deepseek',
    hint: 'Anthropic 兼容层挂在 /anthropic 子路径',
  },
  { id: 'moonshot', label: 'Moonshot (Kimi)', type: 'openai', baseUrl: 'https://api.moonshot.cn', category: 'cn', icon: 'kimi' },
  { id: 'qwen', label: '通义千问 (DashScope)', type: 'openai', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode', category: 'cn', icon: 'qwen' },
  {
    id: 'zhipu',
    label: '智谱 GLM',
    type: 'openai',
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4/',
    category: 'cn',
    icon: 'zhipu',
    hint: '接口版本为 /v4，末尾斜杠表示完整 API 前缀',
  },
  { id: 'zai', label: 'Z.ai (GLM 国际版)', type: 'openai', baseUrl: 'https://api.z.ai/api/paas/v4/', category: 'cn', icon: 'zai' },
  { id: 'doubao', label: '豆包 (火山方舟)', type: 'openai', baseUrl: 'https://ark.cn-beijing.volces.com/api/v3/', category: 'cn', icon: 'doubao', hint: '接口版本为 /api/v3' },
  { id: 'qianfan', label: '百度千帆', type: 'openai', baseUrl: 'https://qianfan.baidubce.com/v2/', category: 'cn', icon: 'qianfan', hint: '接口版本为 /v2' },
  { id: 'hunyuan', label: '腾讯混元', type: 'openai', baseUrl: 'https://api.hunyuan.cloud.tencent.com', category: 'cn', icon: 'hunyuan' },
  { id: 'stepfun', label: '阶跃星辰', type: 'openai', baseUrl: 'https://api.stepfun.com', category: 'cn', icon: 'stepfun' },
  { id: 'minimax', label: 'MiniMax', type: 'openai', baseUrl: 'https://api.minimaxi.com', category: 'cn', icon: 'minimax' },
  { id: 'baichuan', label: '百川智能', type: 'openai', baseUrl: 'https://api.baichuan-ai.com', category: 'cn', icon: 'baichuan' },
  { id: 'yi', label: '零一万物', type: 'openai', baseUrl: 'https://api.lingyiwanwu.com', category: 'cn', icon: 'yi' },
  { id: 'spark', label: '讯飞星火', type: 'openai', baseUrl: 'https://spark-api-open.xf-yun.com', category: 'cn', icon: 'spark' },
  {
    id: 'xiaomi',
    label: '小米 MiMo',
    type: 'anthropic',
    baseUrl: 'https://api.xiaomimimo.com/anthropic',
    category: 'cn',
    icon: 'xiaomi',
  },
  {
    id: 'longcat',
    label: 'LongCat (美团)',
    type: 'anthropic',
    baseUrl: 'https://api.longcat.chat/anthropic',
    category: 'cn',
    icon: 'longcat',
  },

  // --- 聚合与本地 ---
  { id: 'openrouter', label: 'OpenRouter', type: 'openai', baseUrl: 'https://openrouter.ai/api', category: 'aggregator', icon: 'openrouter' },
  { id: 'siliconflow', label: 'SiliconFlow', type: 'openai', baseUrl: 'https://api.siliconflow.cn', category: 'aggregator', icon: 'siliconflow' },
  { id: 'modelscope', label: '魔搭 ModelScope', type: 'openai', baseUrl: 'https://api-inference.modelscope.cn', category: 'aggregator', icon: 'modelscope' },
  { id: 'aihubmix', label: 'AiHubMix', type: 'openai', baseUrl: 'https://aihubmix.com', category: 'aggregator', icon: 'aihubmix' },
  {
    id: 'novita',
    label: 'Novita AI',
    type: 'openai',
    baseUrl: 'https://api.novita.ai/v3/openai/',
    category: 'aggregator',
    icon: 'novita',
    hint: '接口挂在 /v3/openai 前缀下',
  },
  { id: 'ollama', label: 'Ollama (本地)', type: 'openai', baseUrl: 'http://localhost:11434', category: 'aggregator', icon: 'ollama', hint: '本地部署无需 API Key，可随意填写' },
];
