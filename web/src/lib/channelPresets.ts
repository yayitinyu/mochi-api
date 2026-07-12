// Official channel presets. Selecting one pre-fills name, type and base URL
// so the user only needs to paste an API key.
export interface ChannelPreset {
  id: string;
  label: string;
  type: 'openai' | 'anthropic' | 'gemini';
  baseUrl: string;
  hint?: string;
}

export const CHANNEL_PRESETS: ChannelPreset[] = [
  { id: 'openai', label: 'OpenAI', type: 'openai', baseUrl: 'https://api.openai.com' },
  { id: 'anthropic', label: 'Anthropic', type: 'anthropic', baseUrl: 'https://api.anthropic.com' },
  {
    id: 'gemini',
    label: 'Google Gemini',
    type: 'gemini',
    baseUrl: 'https://generativelanguage.googleapis.com',
  },
  { id: 'deepseek', label: 'DeepSeek', type: 'openai', baseUrl: 'https://api.deepseek.com' },
  {
    id: 'moonshot',
    label: 'Moonshot (Kimi)',
    type: 'openai',
    baseUrl: 'https://api.moonshot.cn',
  },
  { id: 'xai', label: 'xAI (Grok)', type: 'openai', baseUrl: 'https://api.x.ai' },
  {
    id: 'qwen',
    label: '通义千问 (DashScope)',
    type: 'openai',
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode',
  },
  { id: 'zhipu', label: '智谱 (GLM)', type: 'openai', baseUrl: 'https://open.bigmodel.cn/api/paas/v4' },
  {
    id: 'siliconflow',
    label: 'SiliconFlow',
    type: 'openai',
    baseUrl: 'https://api.siliconflow.cn',
  },
  { id: 'openrouter', label: 'OpenRouter', type: 'openai', baseUrl: 'https://openrouter.ai/api' },
  { id: 'custom', label: '自定义…', type: 'openai', baseUrl: '' },
];
