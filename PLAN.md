# Go 原生 Agent 实现计划

本计划面向逐步学习和实现：每次完成一个小步骤，验收通过后再继续。以 Go 标准库构建核心，通过 HTTP 调用 DeepSeek，不引入 Agent 框架。第一版定位为本地 CLI 助手，支持多轮对话、工具调用、取消、历史保存和执行记录。

## 当前进度

以下状态根据 2026-10-14 的项目代码核对；代码存在不代表所有场景已经测试。

- [x] 命令行读取任务，检查空输入。
- [x] HTTP 调用模型；API Key 来自 DEEPSEEK_API_KEY。
- [x] 解析模型返回的 tool_calls。
- [x] 执行工具，将结果通过 tool_call_id 返回模型。
- [x] Agent 循环，最多请求模型 10 次。
- [x] 时间、加法、等待工具；带参数工具进行基本校验。
- [x] 工具注册表，将定义和执行函数放在一起。
- [x] Ctrl+C 取消、单次 HTTP 请求超时、任务总超时。
- [x] context 传入模型请求和工具。
- [x] Agent.Run 从模型通信代码中独立出来。
- [x] 交互式多轮会话与本地 JSON 会话保存、加载。
- [x] 模型接口和第一个工具调用历史测试。
- [x] 使用 pgx 连接 Supabase Postgres；会话可保存到数据库并跨进程加载（用户已验证）。
- [x] 阶段十 W1：HTTP 服务（health、创建/读取会话）完成并通过 curl 验收。
- [ ] 数据库存储失败路径、更多离线测试、执行记录与上下文管理。

当前文件职责：main.go 负责入口；agent.go 负责循环；model.go 负责模型协议和 HTTP；registry.go 负责注册；tools.go 负责具体操作。

## 总体执行流程

用户输入 → 构建会话上下文 → 请求模型 → 判断回答或工具调用 → 校验并执行工具 → 追加工具结果 → 再次请求模型 → 保存最终回答。

约定：模型只能请求已注册工具；程序掌握执行权限。assistant 工具调用消息只能追加一次，每个调用都必须有对应 ID 的 tool 结果。最终回答也需要进入会话历史。

## 阶段一：整理现有闭环

### 1. 修正当前细节

实现内容：

- 工具执行后立刻检查 ctx.Err()；取消和超时直接终止，不当作普通工具错误继续请求模型。
- HTTP 非成功响应保留状态和有限大小的错误响应体，不打印 API Key。
- wait_seconds 的 Schema 增加 required: ["seconds"]。
- 等待工具错误文案与实际范围统一为 1 到 30 秒。
- 进度日志输出到 stderr 并换行；stdout 用于最终回答。
- 删除重复的临时错误打印，让 main 统一报告终止错误。

验收：正常任务成功；缺少 Key 有明确提示；等待过程中 Ctrl+C 立即退出；非法参数报错；HTTP 错误能看见状态和原因。

### 2. 完善注册表

实现内容：初始化时拒绝重复名称、空名称和空执行函数；工具定义列表维持固定顺序。保留现有函数式执行接口，暂不引入复杂插件机制。

验收：重复注册不会静默覆盖；增加工具时无需修改 Agent 循环。

## 阶段二：多轮对话（下一步）

### 3. 增加 Session

新建 session.go，定义 Session，保存 ID 和 Messages。会话创建时追加一次系统消息。将任务执行入口调整为 Run(ctx, session, task)，使每轮沿用同一会话，而不是重新构建历史。

每轮追加 user 消息；执行中追加 assistant 工具调用及 tool 结果；成功时追加最终 assistant 回答。

验收：先说“记住数字 123”，再问“刚才的数字加上 456”，模型能使用前文并调用加法工具。

### 4. 明确失败时的历史策略

单轮执行使用临时消息副本。仅在得到有效最终回答时，把这一轮消息提交到 Session；失败或取消时保留此前已完成的历史，未完成轮次记录到执行日志。复制消息时同时复制 ToolCalls 切片，避免共享可变数据。

注意：历史回滚不会撤销已经发生的工具副作用。后续加入写入工具时必须另外记录执行状态。

验收：工具执行过程中取消，再发送新问题，不会带入缺少 tool 结果的 assistant 消息。

### 5. 增加交互式 CLI

无任务参数时进入交互模式，有参数时保留单次任务模式。使用 bufio.Scanner 读取输入并设置输入上限；支持 /exit 和 /new；空输入跳过；EOF 正常退出。

每轮创建独立的任务超时 ctx。初版 Ctrl+C 可以退出整个交互程序；之后如有需要，再实现“取消本轮并继续输入”。

验收：连续输入多个问题；/new 清空旧会话；/exit 和 EOF 正常退出；超时作用于每轮而非整个交互会话。

## 阶段三：保存与加载

### 6. 保存会话快照

新建 store.go，先用标准库实现 JSON 文件存储。格式含 schema_version、session_id、更新时间和 Messages。默认写入项目 data/sessions/，加入 .gitignore；会话 ID 由程序生成，不直接把用户输入拼成路径。

先写同目录临时文件，关闭后重命名替换正式文件；文件使用限制性权限。写入失败必须报告，不能声称保存成功。只保存对话与必要元数据，不保存 Key。

验收：退出后重新加载，仍能引用此前内容；文件损坏有明确错误；保存失败不覆盖原有有效文件。

### 7. 提供会话加载入口

增加显式 --session 参数。若使用 flag 包，调整参数解析，不能再把全部 os.Args[1:] 当作任务。第一版同一个会话只允许一个进程写入。

验收：两个不同会话互不影响；缺失文件行为明确；重新启动后能继续已完成对话。

## 阶段四：可测试的模型层

### 8. 引入小型模型接口

当需要测试 Agent 循环时再加入接口：

    type ModelClient interface {
        Generate(context.Context, ChatRequest) (Message, error)
    }

把现有 HTTP 请求实现移入 DeepSeekClient，持有 Endpoint、APIKey 和复用的 http.Client；由 main 创建并注入 Agent。配置从环境变量或 CLI 读取；保留当前已验证可用的模型名，启动时检查必填项。

验收：真实模型仍能使用；假模型可以按预设顺序返回工具调用与最终回答。

### 9. 添加有意义的测试

使用 testing 和 httptest，不依赖真实 API Key，不产生模型调用费用。

覆盖：

- 模型直接回答，不执行工具。
- 一次和多轮工具调用，历史顺序及 ID 正确。
- 同一条模型消息含多个调用，每个都有结果。
- 缺失参数与合法零值能区分；普通工具错误交回模型。
- 未知工具终止，并且不污染已完成会话。
- 取消与总超时；达到步数上限后不继续请求。
- HTTP 非成功响应、畸形 JSON、空 choices。
- 保存加载往返、损坏文件与失败轮次不提交。

验收：go test ./...、go vet ./... 通过；引入共享状态或并发后运行 go test -race ./...。

## 阶段四补充：接入已有 Supabase 数据库

目标：让现有会话保存和加载能力可选择使用 Supabase。先保留本地 JSON 存储，确认远程读写与重启恢复正常后，再决定是否设为默认。Agent 的工具调用循环不直接操作数据库。

### S1. 确定访问模型与数据库边界

已确认本项目仅在本人控制的本机运行。采用 `github.com/jackc/pgx/v5/pgxpool` 连接 Supabase Postgres，由环境变量提供数据库连接串，不使用 Data API secret key。优先使用仅能访问 `agent_sessions` 的专用数据库角色；凭据不能提交到仓库或打印到日志。将来若改为多用户服务，再设计用户身份与按 owner_id 隔离的 RLS 策略。

核对现有 Supabase 项目的表、认证配置和 Data API 设置；不直接改动已有业务表。决定使用专用的 agent_sessions 表以及是否需要单独的消息表。第一版可沿用现有快照结构，用一行保存一个会话；未来需要消息级查询时再拆表。

验收：从 Supabase Dashboard 的 Connect 面板确认本机网络适用的连接方式（IPv6 直连，或 IPv4 的 Session pooler）、表所在 schema、数据库角色和现有数据是否需要迁移；连接串只保存在本机环境中，不记录到文档。

### S2. 抽象会话存储

为保存和加载定义一个小型 SessionStore 接口，签名包含 context.Context；把当前文件实现包装为 FileSessionStore。让 main 根据配置选择存储实现。Agent.Run 仍只管理本轮内存消息，成功后由入口保存。保存失败要明确报告，不能静默回退到本地并让用户误以为远端已保存。

验收：现有本地模式和 --session 加载继续工作；用假存储测试保存失败、加载失败的传播。

### S3. 建表与权限

使用已创建的专用会话表：稳定的 session_id 主键、版本号、JSONB 消息快照、更新时间；必要时增加 owner_id。表位于 public schema，继续启用 RLS 并拒绝 anon/authenticated。为本机程序配置专用数据库登录角色，只授予此表需要的 SELECT、INSERT、UPDATE 和对应 RLS 策略。直连 Postgres 不依赖 Data API 的表暴露设置；不要为直连而开放 anon/authenticated 的 Data API 权限。

验收：用专用角色执行读写测试；无权限角色不能读写；角色不能访问其他业务表。若未来升级为多用户模式，另行验证用户 A 无法读取用户 B 的会话。

### S4. 用 pgx 实现 PostgresSessionStore

已安装并固定 `github.com/jackc/pgx/v5`，使用 `pgxpool` 复用连接；当前代码从 DATABASE_URL 读取 Postgres 连接串，后续改名为更明确的 DATABASE_URL。已按 session_id 读取一行，保存时使用参数化 SQL `INSERT ... ON CONFLICT ... DO UPDATE`，messages 用 JSONB 存储。后续补连接超时、错误路径测试与凭据日志检查。

验收：离线测试覆盖消息编解码和存储错误传播；在真实项目中保存一轮对话，重启后用同一 session_id 继续，并确认数据库只有对应的一行。数据库连接失败和权限错误应有清楚提示。

### S5. 迁移现有本地会话（可选）

只有在确认需要已有历史时才实现显式导入命令；先预览数量与冲突，再导入。不在程序启动时自动上传 data/sessions/。对已存在的 session_id 明确选择跳过或覆盖，并保留本地原文件。

验收：导入后能读取；重复运行不会意外覆盖远端新历史；失败项可单独重试。

## 阶段五：执行记录与运行预算

### 10. 添加事件日志

用追加写入的 JSONL 保存每次 Run 的事件：run_started、model_requested、model_responded、tool_requested、tool_completed、run_completed、run_failed、run_cancelled、run_limit_reached。

字段包括 run_id、session_id、时间、轮次、调用 ID、耗时和状态；敏感参数和内容允许脱敏。使用 log/slog 输出运行日志。会话快照用于继续对话，事件日志用于排查，二者职责分开。

第一版采用单写入者，不承诺多进程日志一致性。日志不可用于直接重放不明状态的写入工具。

验收：一次任务的生命周期和工具顺序可追踪；异常任务有终止状态；日志无 Key。

### 11. 统一预算配置

增加 RunLimits：最大模型请求次数、最大工具调用次数、任务总时间、单工具超时、最大工具输出大小。工具调用按实际次数累计，避免一条消息含大量调用绕过步数限制。

解析模型 usage，记录服务返回的 token 数；费用估算只有在明确配置价格时才展示。单工具超时不自动重试有副作用的工具。

验收：每种上限都明确终止或返回受控工具错误；输出裁剪有标记；取消仍优先结束任务。

## 阶段六：更实用的只读工具

### 12. 文件读取工具

增加 read_file，参数为相对路径。限定在配置的工作目录内；使用 Go 的受限目录访问能力（例如 os.Root）防止 ..、绝对路径和符号链接逃逸，避免只做字符串前缀判断。限制文件大小，明确二进制文件处理方式。

验收：可读取允许目录内的文本；路径越界和符号链接逃逸被拒绝；大文件输出受控。

### 13. HTTP 查询工具（按需求）

增加受限的 HTTP GET 工具。配置允许的主机，限制重定向、响应体大小、超时和内容类型；检查重定向后的目标。未明确允许时不访问本机或内网地址。

外部网页与文件内容都是工具数据，不能改写系统规则或扩大工具权限。

验收：允许站点可查询；不允许目标被拒绝；取消、中断和超大响应可控。

## 阶段七：上下文管理

### 14. 管理上下文长度

先限制工具输出和消息规模，再考虑摘要。完整历史保留在存储中，给模型的上下文单独构建。JSON 字节数只作保守启发式，不能当作精确 token 数。

裁剪以完整轮次或完整工具调用组为单位，不能留下只有调用而没有结果的消息。保留系统规则、当前任务和近期完整轮次。

验收：长会话仍能请求模型；关键工具配对不被破坏；裁剪行为可解释。

### 15. 增加旧对话摘要

达到阈值时总结旧轮次，保留用户约束、事实、已完成动作及待办。摘要只是压缩上下文，原始历史不删除；记录摘要来源范围。摘要内容不作为新的系统指令。

验收：近期对话保留原文；旧信息可用于继续任务；摘要失败不会破坏历史。

## 阶段八：按需求增加写入能力

### 16. 写文件与权限策略

先支持工作目录内的草稿写入，限定路径和大小。工具标明只读、可撤销写入或外部副作用；模型提出动作，程序按已获授权决定执行。授权不足时形成可审阅动作，再等待用户决定。

验收：未授权操作被拒绝；授权写入成功；路径限制仍有效；结果可审阅。

### 17. 幂等与中断恢复

写入前保存调用意图，完成后保存结果。对外部副作用使用服务端幂等键或查询状态；恢复时区分未开始、已完成和结果未知。

收到网络错误不代表远端未执行。结果未知的写入不自动重放；要求查询远端状态或用户处理。初版恢复只继续已有完整检查点，避免假装具备任意断点恢复能力。

验收：重复提交不会重复产生可避免的副作用；未知状态有明确标记；已完成工具不重复执行。

## 阶段九：可选产品化

### 18. HTTP 与 SSE

确有前端或其他调用者需求时增加 net/http 服务。提供创建任务、查询状态、取消任务和 SSE 事件流。默认仅监听本机；远程暴露时增加认证、会话隔离和并发限制。

SSE 用于展示执行进度；模型 token 流式输出是独立功能，需要单独处理提供商的 SSE 协议及工具参数分片。

验收：客户端看到任务进度；取消能到达执行链；不同任务上下文隔离。

### 19. 再整理目录

文件数量与复用需求明显增加后，再迁移到 cmd/agent 和 internal/agent、model、tool、store、api。先保持简单结构，不为了分层增加循环依赖或大量空接口。

验收：CLI 行为保持一致；测试通过；依赖方向清晰。

## 阶段十：配套 Web 前端 go-agent-web

目标：在独立项目 `../go-agent-web`（Next.js App Router + TypeScript + Tailwind）中提供单会话流式聊天界面。浏览器只与 Next.js 通信，开发期由 Next.js 的 rewrites 把 `/api/*` 代理到本机 Go 服务；生产化部署不在本阶段范围。

边界与安全约定：

- Go HTTP 服务只监听 `127.0.0.1`，不配置 CORS；跨域问题由 Next.js 代理解决。
- DEEPSEEK_API_KEY 只存在于 Go 进程环境，DATABASE_URL 只存在于本机环境；两者都不进入前端产物、URL、响应体或日志。
- 这里的“流式”指执行事件进度（模型请求、工具调用、完成、失败），token 级流式输出属于独立功能，另行设计。
- 失败或取消的轮次不写入会话历史；前端在收到终止事件后重新拉取会话，避免显示未保存的消息。
- 每一步只做一件事，用 curl 或浏览器完成验收后再进入下一步。

### W1. Go 最小 HTTP 服务（已完成）

`server.go`：`Server{store SessionStore}`、`NewServer`、`Handler` 返回 `http.ServeMux`，注册 `GET /api/health`、`POST /api/sessions`、`GET /api/sessions/{id}`。统一 `writeJSON` / `writeError`。

- `POST /api/sessions`：`NewSession()` 后用 store 保存，返回 201 和 `{id, messages}`。
- `GET /api/sessions/{id}`：调用 `store.Load`；`ErrSessionNotFound` 返回 404，非法 ID 返回 400，其他错误返回 500 且不泄漏数据库细节。
- `main.go`：新增 `serve` 子命令，复用现有 `--store` / `--read-root` 逻辑（抽成 `openSessionStore`）；地址默认 `127.0.0.1:8080`；`http.Server` 设置 `ReadHeaderTimeout`，**不设置 WriteTimeout**（为后续 SSE 留出长连接）；用 `signal.NotifyContext` + `Shutdown` 优雅退出。

完成情况（2026-10-27）：`go-agent-web` 脚手架已生成；`server.go` 提供 health 与两个会话接口，`main.go` 抽出 `openSessionStore` 并新增 `serve` 子命令。`go vet ./...`、`go test ./...` 通过；curl 验收为 health 200、创建 201、读取 200、不存在 404、非法 ID 400、DELETE 405；`lsof` 确认仅监听 127.0.0.1；Ctrl+C 可正常退出。无效 ID 分支补了 `return`，避免写完 400 后继续调用 Load。

验收：`go vet ./...`、`go test ./...` 通过；curl 健康检查返回 `{"status":"ok"}`；创建会话返回 201；用返回的 ID 能读回同样的消息；不存在的 ID 返回 404；CLI 单次任务与交互模式行为不变。

### W2. 事件钩子与 SSE（下一步）

新增 `events.go`：`RunEvent` 与事件类型常量 `run_started`、`model_requested`、`model_responded`、`tool_requested`、`tool_completed`、`assistant_message`、`run_completed`、`run_failed`、`run_cancelled`。事件只含排查所需字段：`step`、`tool`、`call_id`、`arguments`、`result`、`error`、`duration_ms`，不打印 Key。

`Agent.RunWithEvents(ctx, session, task, emit)`：在现有循环的模型请求前后、工具执行前后、最终提交处调用 `emit`；`Run` 改为调用 `RunWithEvents` 并传 nil，保证现有测试与 CLI 行为不变（`emit == nil` 时零开销）。

`POST /api/sessions/{id}/messages`，请求体 `{"content":"..."}`：

- 先 `Load` 会话，缺失返回 404；同一会话同时只允许一个 Run（内存互斥表），冲突返回 409。
- 响应头 `Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`，每条事件写完立即 `Flush`。
- 事件帧格式 `data: {json}\n\n`；终止事件为 `run_completed` / `run_failed` / `run_cancelled` 之一，结束后返回。
- 成功后保存会话；失败或取消不保存，历史保持失败前的状态。
- 请求 ctx 断开即取消 Run，不再继续请求模型或执行工具。

为本地联调准备可控的假模型（如 `--model fake`），按脚本返回固定的工具调用与最终回答，避免每次调试都消耗真实额度；真实模型仍通过环境变量启用。

验收：`curl -N` 能看到按顺序到达的事件，工具事件带 `call_id` 且严格成对；中途 Ctrl+C 能终止服务端 Run 并收到取消事件；并发请求同一会话得到 409；`go test -race ./...` 通过。

### W3. Next.js 脚手架与代理

在 `go-agent-web` 中确认依赖与结构：TypeScript、Tailwind、App Router、`src/` 目录；补充 `react-markdown` 与 `remark-gfm`。

`next.config.ts` 增加 rewrites：`/api/:path*` → `http://127.0.0.1:8080/api/:path*`，不把后端地址写进浏览器代码。

`src/lib/types.ts` 手写与 Go 对齐的类型：`Message`、`SessionPayload`、`RunEvent`（用可辨识联合），字段名与 JSON 标签一致。

验收：`npm run dev` 后经 `localhost:3000/api/health` 拿到与直连相同的结果；`curl -N` 经代理请求 SSE 时事件是逐条到达而不是被缓冲；`next build` 通过。

### W4. 会话创建与历史渲染

`src/lib/api.ts` 封装 `createSession`、`getSession`；会话 ID 存 localStorage，页面加载时尝试恢复，404 则新建。注意 React StrictMode 下 effect 会执行两次，用 ref 或请求取消避免重复创建两个会话。

组件：`MessageList`、`MessageBubble`（区分 user / assistant，Markdown 渲染）、`HeaderBar`（显示会话 ID 与新建按钮）、`Composer`（输入框与发送按钮，本步可先不接流式）。system 消息和 tool 消息不直接展示为聊天气泡。

验收：刷新页面历史仍在；换浏览器或清空 localStorage 得到新会话；新建按钮能切换会话且旧会话不丢失。

### W5. SSE 客户端、工具卡片、取消与错误

`src/lib/sse.ts`：因为要 POST，不能用 `EventSource`；用 `fetch` + `ReadableStream` + `TextDecoder({stream:true})`，按空行分帧，兼容一帧被拆到多个 chunk、多个帧落在同一 chunk 的情况；只处理 `data:` 行，忽略注释与心跳。

`ToolCallCard`：以 `call_id` 为 key 合并 `tool_requested` 与 `tool_completed`，展示工具名、参数、结果、耗时与错误；结果过长时折叠。`assistant_message` 到达后作为正式气泡入列。

交互：发送中禁用输入并显示“停止”按钮，点击时 `AbortController.abort()`；收到终止事件后调用 `getSession` 刷新，保证界面与已保存历史一致；409、网络中断、服务端错误都要有可见提示。

验收：问“现在几点”能看到工具卡片从进行中变为完成；工具超时能显示错误结果但对话继续；取消后界面回到可输入状态且历史与服务端一致；断开 Go 服务后有明确错误提示。

### W6. 切换 Postgres 与整体验收

Go 服务改用 `--store postgres` 启动，前端代码不变，验证跨进程、跨重启的历史恢复。

安全复查：`lsof -iTCP -sTCP:LISTEN` 确认只监听 127.0.0.1；检查响应头没有 `Access-Control-Allow-Origin`；搜索前端产物与日志，确认没有 Key 或连接串；`.env` 与 `data/` 均未被提交。

端到端验收：冷启动 Go → 新建会话 → 多轮含工具的对话 → 重启 Go → 刷新浏览器仍能恢复历史 → 取消与失败路径不污染历史。可选：`next build && next start` 走一次生产构建，确认代理与流式仍正常。

## 每一步的工作方式

1. 明确本次行为和验收输入。
2. 只实现该步骤，保留已跑通的闭环。
3. go fmt ./...；阶段四以后运行对应测试和 go vet ./...。
4. 必要时用真实模型验证一条代表性任务，真实模型行为不作为唯一测试依据。
5. 完成后提交一个小的 Git commit，更新本计划的进度。

## 第一版完成标准

完成阶段一至五即形成可维护的 CLI 基础版本：支持交互多轮对话、三个现有工具、取消和预算、会话保存加载、执行记录及离线测试。阶段六按使用需求增加至少一个实用只读工具。

上下文摘要、写入恢复、HTTP 服务和多 Agent 都按实际需求推进；长期记忆、向量数据库、任意 Shell 和自动并行执行不作为第一版前置条件。

## 下一次具体任务

当前推进阶段十 W2：给 `Agent` 增加运行事件钩子，实现 `POST /api/sessions/{id}/messages` 的 SSE 响应与同会话互斥，并准备可控的本地假模型。

CLI 侧的后续仍是：检查不存在的会话 ID、数据库断连与取消时的错误行为；之后补离线测试，再开始执行记录与预算。
