# 原生服务接口与限制

本文描述原生服务实现和公开验证范围，不包含个人部署或账号盘点。升级与恢复见[通用运维指南](operations.md)，官方协议来源见[供应商研究](provider-research-20261027.md)。

## 接口

| 用途 | 下游接口 | 执行方式 |
| --- | --- | --- |
| Cohere Embedding | `POST /v2/embed` | cohere 预设，原生 texts/images/inputs，保留 input_type、embedding_types、output_dimension |
| SiliconFlow 多模态检索 | `POST /v1/embeddings`、`POST /v1/rerank` | 处理文字和 text/image 内容对象，查询与候选分别支持已实现的图片形状 |
| Vertex Embedding | `POST /v1beta/models/{model}:embedContent` | 使用服务账号项目与区域，转至 publisher 原生路径；batchEmbedContents 尚未声明支持 |
| 原生多协议来源 | 已有 `/v1`、`/v1beta` 和 `/v2/embed` 路径 | native_api 预设复用多协议执行器，实际接口是否开放仍由上游决定 |
| Mistral OCR | `POST /v1/ocr` | 保留 document 和页面参数，解析供应商明确报告的 pages_processed |
| Mistral 文件 | `POST /native/mistral/v1/files`；`GET/DELETE .../files/{id}`；`GET .../{id}/content`、`GET .../{id}/url` | multipart 保留二进制，后续操作使用原创建凭据 |
| Mistral 批任务 | `POST /native/mistral/v1/batch/jobs`；`GET/DELETE .../jobs/{id}`；`POST .../jobs/{id}/cancel` | 输入文件必须属于同一凭据，任务响应登记的输出文件可继续下载 |
| Soniox ASR | `POST /native/soniox/v1/files`、`POST .../transcriptions`、`GET .../transcriptions/{id}`、`GET .../{id}/transcript` | 保留文件、语言、说话人和翻译参数，资源查询固定来源 |
| Soniox 删除 | `DELETE /native/soniox/v1/files/{id}`、`DELETE .../transcriptions/{id}` | 不自动删除输入文件，处理中任务的删除限制由上游返回 |
| Soniox TTS | `POST /native/soniox/tts` | 使用独立 TTS 地址，返回原始音频字节；当前为缓冲式 REST 响应 |
| SiliconFlow 视频 | `POST /native/siliconflow/v1/video/submit`、`POST .../video/status` | 保留 requestId，状态查询固定来源，结果按供应商 URL 下载 |
| Jev | `POST /v1/systemone` | 独立 Decisions、JSON assets、multipart request 和图片文件；支持 Box 及图片实验开关 |

Mistral 已有音频和 FIM 入口继续保留。未知原生路径不会被解释成聊天或转发到管理接口。普通原生响应缓冲上限为 32 MiB，Embedding 为 64 MiB；请求上限按可用内存计算，限制在 16 到 128 MiB。超过限制明确拒绝，不承诺覆盖供应商全部最大文件尺寸。

## 授权与转换

所有数据接口使用 API-Load 访问令牌。管理员按分组、协议和必要型号授权，上游凭据留在网关中。

无 model 的文件和任务管理操作需要显式 `allow_model_less_resources`，仅对已实现的 native-services 资源操作生效。资源仍属于创建它的访问令牌。带 model 的生成请求继续检查型号范围；之后缩小型号列表不会自动撤销已有资源的管理权限，关闭资源授权才会阻止相应无 model 操作。

转换路径不会静默删除无法表达的工具、图片或精确思考预算。遇到关键语义损失时返回明确错误，不发送上游请求。可表达的字段继续转换，原生接口保留供应商字段。

## 调度与资源归属

既有优先级、权重和软粘性继续使用。权重分担独立请求，不能保证一把 key 的额度恰好耗尽后才换下一把。共享账号的分组可设置相同 account_pool，按最小非零并发和 RPM 上限共同约束。

共享限制当前在同一进程中执行，重启会重置 RPM 窗口，多副本尚不共享计数。到达限制返回 429，不排队。管理员负责账号归属；系统不按 key 数推算额度，也不提供通用自动余额识别。

资源归属复用持久化 checkpoint，区分供应商、资源类型、ID、访问令牌和凭据身份。记录缺失或来源不可用时拒绝，不随机换 key。当前仍是有界记录，异常退出可能丢失最近一次 checkpoint 之后的新增绑定。

## 用量与证据

`request_logs.measured_units` 与 `request_logs.service_evidence` 为可空 JSON。提取器保留 Cohere billed_units、Rerank search_units 与 Mistral OCR pages_processed。没有真实测量的单位保持缺失，未配置对应价格时显示未计价，不能把未知当成免费。

能力页面分开模型声明、供应商范围、实现路径和账号请求证据。查询窗口不超过日志保留期及 30 天，每组扫描最多 200 条相关记录，每个型号最多保留 5 条不同证据，并返回截断信息。摘要不保存文字、图片、URL 或文件名。

目录读取成功不等于请求成功；HTTP 成功不等于结果完整或质量良好。429 保留失败状态，没有识别结果时不能声称 OCR 或检索质量通过。

## 检查范围

合成检查覆盖 Cohere 文字、图片和混合输入保真，Vertex 路径及 inlineData，图片查询和候选，Jev multipart，Soniox 独立 TTS 地址，以及文件、视频轮询和资源归属。另检查关键语义损失不发送请求、共享账号限制、非 token 用量缺失和历史数据库迁移。

Linux 容器与 PostgreSQL 合成升级使用已有本地镜像、独立内部网络和临时数据，检查旧凭据解密与访问令牌兼容。可复用工具为 `scripts/verify-native-container.py`。合成服务通过不能代替真实供应商或生产副本验收。

完整 Go 套件曾受 Windows 路径、系统代理、慢连接及 WSL Bash 环境影响，不能将定向测试通过写成全量通过。OCR 识别质量、语音可懂度、视频内容、多副本限额和全部账号仍需分别验证。异步任务最终用量与非 token 价格编辑尚未完整实现，轮询不能重复累计任务消费。
