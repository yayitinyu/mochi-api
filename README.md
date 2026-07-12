# Mochi 🌸

柔软又可靠的轻量级 AI API 聚合网关。参考 [new-api](https://github.com/QuantumNous/new-api) 的核心思路做的精简版：单二进制、可爱的粉彩 UI、够用就好。

## 功能

- **API 聚合转发**：OpenAI 格式（`/v1/chat/completions`）与 Claude 原生格式（`/v1/messages`），均支持 SSE 流式
- **跨格式互转**：OpenAI、Claude、Gemini 三种上游任意互调——用 OpenAI 或 Claude 格式的客户端都能调到 Gemini 模型，反之亦然
- **账号系统**：注册 / 登录（cookie 会话 + bcrypt），首位注册用户自动成为管理员
- **API 密钥管理**：创建、启停、删除，密钥完整值仅创建时展示一次
- **渠道管理**：官方渠道模板一键预填、连通性测试、一键从上游拉取模型列表
- **自定义模型价格**：按每百万 token 的输入 / 输出价配置，支持 `claude-3-5*` 前缀通配
- **调用日志**：记录模型、token 数、费用、耗时、状态码
- **使用统计**：GitHub 风格贡献热力图 + 近 30 天费用趋势 + 模型用量排行
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
- **优先级**：数值越大越优先，同级随机

之后用创建的 `sk-` 密钥即可调用，`Authorization: Bearer sk-xxx` 与 `x-api-key: sk-xxx` 两种方式都支持。Gemini 渠道会自动在 OpenAI / Claude 格式与 Gemini 原生格式之间转换。
