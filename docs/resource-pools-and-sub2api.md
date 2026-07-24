# 共享资源池、Sub2API 与 Claude Code 接入

这套模式适合以下场景：同一批官方采购的上游 URL + Key，需要同时服务 OpenAI 格式的 Chat/Batch 与 Anthropic 格式的 Coding；Sub2API 只负责下游用户、额度和用量，api-load 是唯一接触真实 Key 的调度层。

```text
Claude Code / Chat 客户端
        ↓
Sub2API
下游 Key、用户鉴权、额度与用量统计
        ↓
api-load
真实 Key、粘性路由、额度状态与故障迁移
        ↓
官方上游 URL + Key
```

## 配置顺序

1. 在 api-load 的“资源池”页面创建一个池。默认粘性 TTL 为 3600 秒，普通限流迁移前最多等待 2000 毫秒。
2. 按 `name | upstream URL | key` 批量录入物理资源。URL 与 Key 是不可拆分的一对；同一 Key 配不同 URL 会被视为不同物理资源。
3. 创建两个标准分组，例如 `chat` 与 `coding`：
   - `chat` 使用 OpenAI 渠道；
   - `coding` 使用 Anthropic 渠道；
   - 两者绑定同一个资源池；
   - 分组不再维护自己的 upstream 与官方 Key。
4. 在 Sub2API 中分别把 OpenAI 与 Anthropic 上游指向对应的 api-load 分组：
   - `https://api-load.example.com/proxy/chat`
   - `https://api-load.example.com/proxy/coding`
5. 官方 Key 只保存在 api-load。Sub2API 只保存调用 api-load 所需的下游代理凭据，不能再做真实 Key 轮询、同请求跨上游重试或 Anthropic→OpenAI 格式转换。

## 会话与项目粘性

推荐让客户端或 Sub2API 透传一个稳定的项目标识：

```http
X-Api-Load-Affinity: company/project-a
```

api-load 只保存该值的 HMAC，不保存原始项目名。优先级如下：

1. `X-Api-Load-Affinity`；
2. 请求 `metadata.session_id` / `metadata.user_id`；
3. `cache_control` 标记内容；
4. IP、User-Agent、下游 Key、system 与首条 user 消息组成的稳定摘要；
5. 无法稳定识别时不做粘性绑定。

成功请求会刷新 1 小时滑动 TTL。粘性有利于项目上下文与上游缓存持续命中，但它不是永久锁死：除 404 外，上游调用失败会自动禁用当前物理资源；普通可重放请求会迁移，并在成功后永久重绑到另一物理资源。

## Claude Code

Claude Code 官方支持用 `ANTHROPIC_BASE_URL` 接入网关，用 `ANTHROPIC_CUSTOM_HEADERS` 添加一行一个的自定义请求头。Bearer 网关使用 `ANTHROPIC_AUTH_TOKEN`；要求 `x-api-key` 的网关改用 `ANTHROPIC_API_KEY`，不要同时设置两种凭据变量。参考 Anthropic 官方的 [LLM gateway 配置](https://code.claude.com/docs/en/llm-gateway-connect) 与 [settings 配置](https://code.claude.com/docs/en/settings)。

PowerShell 临时会话示例：

```powershell
$env:ANTHROPIC_BASE_URL = "https://sub2api.example.com"
$env:ANTHROPIC_AUTH_TOKEN = "<sub2api-downstream-key>"
$env:ANTHROPIC_CUSTOM_HEADERS = "X-Api-Load-Affinity: company/project-a"
claude
```

也可以写入 Claude Code 的用户级或项目本地 `settings.json`：

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "https://sub2api.example.com",
    "ANTHROPIC_AUTH_TOKEN": "<sub2api-downstream-key>",
    "ANTHROPIC_CUSTOM_HEADERS": "X-Api-Load-Affinity: company/project-a"
  }
}
```

`ANTHROPIC_BASE_URL` 填 Sub2API 暴露的 Anthropic 兼容基地址，不要填到具体的 `/v1/messages`。如果 Sub2API 要求 `x-api-key`，把示例中的 `ANTHROPIC_AUTH_TOKEN` 换成 `ANTHROPIC_API_KEY`。

## 流式检查

Claude Code 的 `/v1/messages` 应保持原生 Anthropic 协议与 SSE，不要在 Sub2API 中转成 OpenAI 格式。api-load 会逐块转发并立即 `Flush`，同时发送 `X-Accel-Buffering: no`。如果外层还有 Nginx、CDN 或 WAF，也必须关闭响应缓冲。

不要只看最终响应是否正确；用 `curl -N` 或客户端日志确认首个 SSE event 会立即到达。例如：

```bash
curl -N https://sub2api.example.com/v1/messages \
  -H "Authorization: Bearer <sub2api-downstream-key>" \
  -H "X-Api-Load-Affinity: company/project-a" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  --data '{"model":"claude-model","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"reply slowly with three short lines"}]}'
```

如果思考内容和回答仍然一次性出现，按顺序检查：Sub2API 是否保留 `stream:true`、响应 `Content-Type` 是否为 `text/event-stream`、是否发生协议转换、反向代理是否缓冲、客户端是否确实逐事件读取。

## 故障行为

| 上游结果 | api-load 行为 |
| --- | --- |
| 普通 429 | 自动禁用该 URL + Key，所有协议停止使用；普通可重放请求迁移并在成功后重绑 |
| 明确 quota / credit / billing exhausted，或 402 | 默认把该 URL + Key 标为凭据失效，Chat 与 Coding 都停止使用，需手动测试或启用恢复；池配置了配额自动恢复调度时改为全局冷却到下一个恢复点，到点自动回到轮转 |
| 401 / 403 | 自动禁用该 URL + Key，所有协议停止使用，需管理员恢复 |
| 其他 4xx（404 除外）、网络错误 / 5xx | 自动禁用该 URL + Key；普通可重放请求可以迁移 |
| 404 | 不计失败次数，也不改变资源状态 |
| SSE 已经开始 | 绝不跨 Key 续写；把已产生内容拼到另一响应会破坏协议与语义 |

自动禁用的资源会立即停止调度；“恢复”会清除失败计数、全局冷却与停用原因。只有资源池显式配置了“配额自动恢复”时，配额/账单类失败才会进入全局冷却。原始 Key 不会通过管理 API 返回，页面只显示末四位掩码。

## Batch 与 File

OpenAI Batch 和 File 都是上游账户级对象，不能在不同官方 Key 之间随意查询：

- File 上传成功后，file ID 会绑定创建它的物理资源；
- 创建 Batch 时，api-load 根据 `input_file_id` 强制回到 File 的原资源；
- batch ID 以及响应里的 input/output/error file ID 会持久化绑定；
- 查询、取消、文件元数据与内容下载只访问原资源；
- 原资源不可用时明确返回错误，不会换 Key；
- File/Batch 创建遇到网络错误或 5xx 时不自动重放，避免“上游已创建、网关却没收到响应”造成重复任务。

`GET /v1/batches` 是账户级列表，单次请求只能看到当前调度到的一个官方账户。需要跨资源总览时应在业务层按资源汇总，不应伪装成一次可迁移的普通请求。

拥有 Batch/File 绑定的物理资源不能被永久删除，只能先停用；对应对象不再需要后再做数据清理。资源池仍被分组引用时同样禁止删除。

## 运行检查

- api-load 日志里的 `resource_id` 应能确认请求实际落到哪一对 URL + Key；
- 相同 `X-Api-Load-Affinity` 的连续 Coding 请求应保持同一 `resource_id`；
- 普通 429 后最多约 2 秒应切换资源，后续请求保持在新资源；
- quota/401/403 后，同一物理资源在 OpenAI 与 Anthropic 两条路由都不再被选择；
- Sub2API 不应执行第二次真实上游选择或长达数十秒的同账户重试；
- 生产多实例必须共享数据库与 Redis，否则不同 api-load 实例无法共享粘性、冷却和资源状态。
