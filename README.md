# Mochi 🌸

柔软又可靠的轻量级 AI API 聚合网关。

## 功能

- **API 聚合转发**：OpenAI Chat Completions（`/v1/chat/completions`）、OpenAI Responses（`/v1/responses`）与 Claude 原生格式（`/v1/messages`），均支持 SSE 流式
- **跨格式互转**：OpenAI Chat/Responses、Claude、Gemini 三种上游互调；OpenAI 兼容渠道默认将 Responses 安全转换到 Chat Completions，也可为完整兼容的上游显式开启原生 Responses
- **推理与工具**：保留 `reasoning`、函数工具和 Responses 内置工具参数；Gemini 渠道支持 thought summary / signature 转换，并将 `web_search` 映射为 Google Search grounding
- **账号系统**：注册 / 登录（cookie 会话 + bcrypt），首位注册用户自动成为管理员
- **API 密钥管理**：创建、启停、删除，密钥完整值仅创建时展示一次
- **渠道管理**：官方渠道模板一键预填、连通性测试、一键从上游拉取新模型、可选择清理现有模型
- **自定义模型价格**：按每百万 token 的输入 / 输出价配置，可直接选择渠道中已添加的模型，支持 `claude-3-5*` 前缀通配
- **调用日志**：记录模型、token 数、费用、耗时、状态码
- **使用统计**：以 Tokens 为主单位的 GitHub 风格热力图、近 30 天趋势和模型用量排行，费用作为辅助信息
- **可爱 UI**：樱花粉彩主题、原创樱花 SVG 标志、深色 / 浅色模式切换、移动端适配、各 AI 厂商官方图标（[@lobehub/icons](https://github.com/lobehub/lobe-icons)）

> 计费为**仅统计花费**模式：只计算并记录每次调用的美元花费，不做余额扣费 / 充值。

## 技术栈

- 后端：Go + Gin + GORM + SQLite（`glebarez/sqlite` 纯 Go 驱动，无需 cgo）
- 前端：React 19 + Vite + Tailwind CSS 4 + Recharts + Phosphor Icons
- 交付：前端产物通过 `go:embed` 打进单个可执行文件

## 构建与运行

```bash
# 1. 构建前端
cd web
npm install
npm run build

# 2. 构建并运行后端（会 embed 上一步的 web/dist）
cd ..
go build -o mochi.exe .
./mochi.exe
```

默认监听 `:3000`，浏览器打开后注册第一个账号即为管理员。

### Docker 部署

```bash
docker compose up -d
```

数据库持久化在 `./data/`，时区通过 `docker-compose.yml` 里的 `TZ` 环境变量设置（影响每日统计的日期边界）。也可以手动构建镜像：

```bash
docker build -t mochi-api .
docker run -d -p 3000:3000 -e TZ=Asia/Shanghai -v ./data:/data mochi-api
```

### 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `3000` | HTTP 监听端口 |
| `MOCHI_DATA` | `.` | SQLite 数据库文件所在目录 |

> 每日统计按**服务器本地时区**在写入时冻结。若用 Docker 部署，请设置 `TZ` 环境变量以对齐热力图的日期边界。

## 开发

```bash
# 后端（终端 A）
go run .

# 前端 dev server（终端 B），/api 与 /v1 会代理到 :3000
cd web && npm run dev
```

`devtools/` 下有开发辅助工具：`mockupstream`（假上游，用于本地测试转发）、`seed`（回填演示日志，用于查看仪表盘图表）。

## 配置上游

在「渠道管理」中添加上游：

- **官方渠道**：下拉选择 OpenAI / Anthropic / Gemini / DeepSeek / Kimi / Grok / 通义 / 智谱 / SiliconFlow / OpenRouter，自动填好类型和 Base URL，只需再粘贴 Key
- **类型**：OpenAI 兼容、Anthropic 兼容 或 Google Gemini
- **Base URL**：如 `https://api.openai.com`（不含 `/v1` 路径）；Gemini 用 `https://generativelanguage.googleapis.com`
- **模型**：逗号分隔的模型名列表，或点「获取模型」从上游自动拉取
- **Responses 兼容模式**：OpenAI 兼容渠道默认使用「Chat 转换」；确认上游完整支持 `/v1/responses` 及其流式事件后，才选择「原生 Responses」
- **优先级**：数值越大越优先，同级随机

之后用创建的 `sk-` 密钥即可调用，`Authorization: Bearer sk-xxx` 与 `x-api-key: sk-xxx` 两种方式都支持。Gemini 渠道会自动在 OpenAI / Claude 格式与 Gemini 原生格式之间转换。

### Responses API 示例

```bash
curl http://localhost:3000/v1/responses \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "input": "检索今天的重要 AI 新闻并总结",
    "reasoning": {"effort": "medium", "summary": "auto"},
    "tools": [{"type": "web_search"}],
    "stream": true
  }'
```

OpenAI 兼容渠道默认把可移植的 Responses 请求改发到上游 `/v1/chat/completions`，再将流式或非流式结果转换回 Responses 格式，避免只有部分 Responses 兼容性的渠道返回残缺 SSE 事件。若渠道已在管理页显式选择「原生 Responses」，Mochi 才会原样请求上游 `/v1/responses`；该端点明确报告不支持时仍会自动回退。`previous_response_id`、托管工具等无法完整转换的状态型或原生能力需要启用原生 Responses。Gemini 渠道会转换常用输入、函数工具、`reasoning` 和 `web_search`，并清理 Gemini `parameters` 不接受的 JSON Schema 关键字；Anthropic 渠道暂不转换 Responses 内置托管工具，并会返回明确错误。浏览器与 WebView 客户端可直接跨域访问 `/v1`，无需借助第三方 CORS 代理。
