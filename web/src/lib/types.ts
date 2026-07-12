export const ROLE_ADMIN = 10;

export interface User {
  id: number;
  username: string;
  role: number;
}

export interface Token {
  id: number;
  name: string;
  key_preview: string;
  status: number;
  created_at: number;
}

export interface Channel {
  id: number;
  name: string;
  type: string;
  base_url: string;
  api_key: string;
  models: string;
  priority: number;
  status: number;
  created_at: number;
}

export interface ModelPrice {
  id: number;
  model: string;
  input_price: number;
  output_price: number;
}

export interface LogEntry {
  id: number;
  user_id: number;
  created_at: number;
  day: string;
  token_name: string;
  channel_id: number;
  model_name: string;
  prompt_tokens: number;
  completion_tokens: number;
  cost_micros: number;
  use_time_ms: number;
  is_stream: boolean;
  code: number;
}

export interface PeriodStat {
  requests: number;
  tokens: number;
  cost_micros: number;
}

export interface StatsSummary {
  today: PeriodStat;
  week: PeriodStat;
  month: PeriodStat;
}

export interface DailyStat {
  day: string;
  requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost_micros: number;
}

export interface ModelStat {
  model_name: string;
  requests: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost_micros: number;
}
