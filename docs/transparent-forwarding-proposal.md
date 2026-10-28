# 通用透明 HTTP 转发

本文描述 API-Load 2.1 的透明 HTTP 配置、权限和运行时限制。它是独立可选入口，不改变既有原生或兼容接口。部署与恢复见[通用运维指南](operations.md)。

## 来源和原生入口

现有静态 API-key 分组可以在 `overrides.transparent_proxy` 中启用透明转发，继续复用同一凭据池。原有原生协议和转换接口保留。也可创建 `transparent_http` 来源，它没有模型目录或探针，空型号列表不妨碍透明调用。

入口为 `/proxy/{group_id}/*path`。目标主机和可选路径前缀由管理员固定。下游路径和原始查询串追加到这个前缀后，不允许指定另一个目标主机。例如管理员配置 `https://api.cohere.ai` 后，`/proxy/1/v2/embed` 对应 `https://api.cohere.ai/v2/embed`。若配置已经包含 `/v2` 前缀，下游就应调用 `/proxy/1/embed`。

透明通道不经过 dialect JSON 检查、模型映射、参数覆盖、工具转换、SSE 内容校验或用量提取。JSON、multipart、二进制和长流按内容字节转发，HTTP 分帧由正常代理协议处理。响应状态、压缩编码和内容交给下游；响应头中的已知凭据仍须经过网关的泄漏检查。

首版不转发 100、103 等临时 1xx 响应，以避免 Gin 的延迟响应状态影响最终结果。临时响应头仍检查已知凭据；正常 103 后最终 200 的状态、头部和正文已经过定向测试。WebSocket 升级不属于这一入口。

## 配置契约

`GET /api/groups/{id}/settings` 的 `effective.transparent_proxy` 返回完整对象，原始覆盖值保存在 `overrides.transparent_proxy`。

| 字段 | 含义 |
| --- | --- |
| `enabled` | 明确启用，旧配置默认关闭 |
| `base_url` | 管理员固定的 HTTP 或 HTTPS 地址，可包含路径前缀，不接受用户名、密码、查询串和片段 |
| `auth_header` | 注入上游 key 的请求头，如 `Authorization` 或 `X-API-Key`；禁止使用 Host 等传输控制头 |
| `auth_prefix` | key 前缀，如 `Bearer `；尾部空格有意义，不会自动裁剪 |
| `strategy` | `rotate` 或 `fixed` |
| `fixed_credential_id` | 固定策略必须引用本组已存在凭据；轮换策略填写 0 |

`rotate` 只决定每个独立请求选择哪条凭据，复用既有权重、健康状态、并发和共享账号限制。它不会维持任意协议中的文件、任务或会话归属。有状态操作应使用 `fixed`；固定凭据失效、停用或冷却时明确失败，不自动换另一把 key。删除固定凭据前，须先停用透明选项或修改固定编号。

同一账号的多把 key 不会增加账号额度。透明转发不推断账号余额、项目归属、文件权限或供应商计费规则。

首版只接入只有 `api_key` 字段的静态凭据来源。Vertex 服务账号、Bedrock 签名和订阅凭据刷新仍走已有专用适配，不能把这些凭据当成静态字符串拼入请求头。

## 显式访问权限

旧访问令牌不会自动获得透明权限。透明调用同时要求 `filters.protocols` 明确包含 `transparent-http`，并配置 `filters.transparent`。

```json
{
  "group_ids": [1],
  "methods": ["POST"],
  "paths": [{"path": "/v2/embed", "match": "exact"}]
}
```

三个范围都不能为空。方法只接受 GET、HEAD、POST、PUT、PATCH、DELETE 和 OPTIONS。`exact` 只允许这个路径；`subtree` 允许路径自身及其斜杠下级，例如 `/v1/files` 可以覆盖 `/v1/files/abc`，不会覆盖 `/v1/files-other`。根路径 `/` 配合 `subtree` 表示显式允许来源下的全部路径。

路径必须使用规范绝对形式。点段、重复斜杠、编码分隔符、二次编码及反斜杠不能绕过范围检查。普通分组限制仍与透明范围相交。

透明请求体不解析，因此不能可靠识别型号，也不能承诺按型号授权。带非空 `filters.models` 的令牌不能同时获得透明权限。金额预算同样不能用于透明令牌，管理接口拒绝这组配置，网关也会再次拒绝异常配置。原始用量保留 missing，不把未知金额记成已知零消费。RPM、并发、来源、方法和路径限制仍可使用。

## 合成来源的管理请求示例

以下使用合成分组编号 1 和占位令牌，不对应任何个人部署，也不包含真实 key。先读取 `GET /api/groups/1/settings`，保留返回的完整 `overrides`。PUT 中的 overrides 是完整替换，必须把下列配置合入原对象后发送，不能清掉其他覆盖项。

```javascript
const current = (await GET('/api/groups/1/settings')).data;
const payload = {
  overrides: {
    ...current.overrides,
    transparent_proxy: {
      enabled: true,
      base_url: 'https://api.cohere.ai',
      auth_header: 'Authorization',
      auth_prefix: 'Bearer ',
      strategy: 'rotate',
      fixed_credential_id: 0
    }
  }
};
await PUT('/api/groups/1/settings', payload);
```

随后向 `POST /api/access-keys` 发送下列请求体。管理请求使用管理员自己的安全登录状态，并按现有接口携带新的 `Idempotency-Key`。生成的访问令牌只交给目标客户端保存，不写入文档或日志。

```json
{
  "name": "Synthetic transparent embed scope",
  "filters": {
    "groups_restricted": true,
    "groups": [1],
    "protocols": ["transparent-http"],
    "models": [],
    "transparent": {
      "group_ids": [1],
      "methods": ["POST"],
      "paths": [{"path": "/v2/embed", "match": "exact"}]
    }
  },
  "rpm_limit": 3,
  "concurrency_limit": 1
}
```

下游请求的示例令牌只是占位文本。

```http
POST /proxy/1/v2/embed HTTP/1.1
Authorization: Bearer synthetic-transparent-client-token
Content-Type: application/json

{"model":"embed-v5.0-fast","texts":["A red square."],"input_type":"search_document","embedding_types":["float"],"output_dimension":768}
```

这里的 model、input_type 和 output_dimension 由供应商解释。若需要网关检查向量条数、记录图片 token 或按模型限制调用，应继续使用已有 `/v2/embed` 原生适配。

## 转发与失败边界

网关入口只通过请求头认证，不支持仅在查询串中携带网关令牌。网关认证头不会交给供应商，由来源配置注入选中凭据。查询串中与已认证网关令牌精确匹配的 `key` 项会被剔除，其他业务查询片段保留原始编码与顺序。逐跳头和 Host 由代理规则处理，重定向直接返回下游，不携带上游凭据自动跟随跳转。Location 和正文中的 URL 不会自动改写，下载、状态查询与取消请求需要客户端继续使用配置的网关入口；任意外部 URL 或新主机不会自动转成代理请求。fixed 只固定同一来源中的凭据，不代表未知协议的整个资源生命周期已经自动适配。

当前每次透明调用只尝试一条凭据，包括 GET 和 HEAD。POST、文件上传和未知任务可能已经被上游接受，网关不会在传输失败或流开始后重新发送。调用方应结合供应商协议判断能否重试，不能把透明转发理解为自动幂等保证。

不透明业务响应的 401 或 403 默认不会记为凭据失败。管理员可以用显式状态规则配置已知上游的冷却或失败行为，正文没有解析，因此错误关键词规则不能匹配错误正文。429 只临时冷却选中的凭据，不触发第二次发送。

已有请求体上限、首响应超时、总时限、流空闲超时及下游写入限制继续生效，取消会传播给上游。WebSocket、HTTP Upgrade、CONNECT 和 TRACE 不在首版范围内。

全局内容审查或脱敏开启时，透明入口会明确拒绝请求。这些策略需要理解或改写内容，不能在不告知管理员的情况下跳过。关闭或调整这类全局策略仍需单独审查其对原生调用的影响。

响应头与流结束后的 trailer 会检查已知网关或供应商凭据。若 trailer 在响应已开始后回显凭据，连接会中断且不重试。

响应体保持不透明，不扫描正文中的凭据。因此不能承诺任意供应商响应正文都不会回显密钥。管理员应只授权业务接口，避免把密钥管理、账户管理和调试回显资源包含进路径范围；`/` 的 `subtree` 授权尤其需要审查。

日志只保存来源、凭据编号、状态、阶段、时间和请求标识，不记录请求体、返回文件或签名下载链接。透明服务不会生成模型能力验证结论，也不会因上游返回 HTTP 200 就声称 OCR、向量或音频结果完整。

## 本地验证与边界

配置和权限检查已覆盖旧令牌默认拒绝、显式协议许可、分组与方法范围、路径边界、编码绕过、模型和金额限制冲突、空模型来源、固定凭据归属，以及原生路由保留。相关 control、state、loader、channel、protocol 和 scheduler 包检查通过。

核心转发的未知 JSON、multipart、二进制、流式响应、重定向、取消和鉴权隔离由网关定向验收负责。实际通过范围以对应测试报告和上线记录为准。本地模拟服务不能代替任意供应商的真实协议验收。
