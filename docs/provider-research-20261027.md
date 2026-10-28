# 供应商协议核验

核验日期为 2026-10-27。以下协议说明来自供应商官方文档与 API-Load 实现。个人连接、账号盘点、来源数量和部署记录不属于公开资料；目录读取成功不能证明请求、结果完整性或质量通过。

供应商声明、API-Load 路径和具体账号验证分别记录。下表只说明协议及实现边界，不表示所有账号或型号均已实测可用。

## 协议与实现范围

| 供应商 | 官方接口及输入输出 | 本轮决定与边界 |
| --- | --- | --- |
| Cohere | `/v2/embed` 使用 texts、images 或 inputs，input_type 指定检索用途，响应按 embedding_types 返回向量。Rerank 使用 query/documents，返回相关性结果 | 新增原生 Embed。保留维度、量化类型和未来供应商字段，校验返回条数与显式维度。文字、图片分别实测一次。官方模型页本轮取回内容只列到 v4，不能拿它补写 v5 全部规格。[Embed API](https://docs.cohere.com/reference/embed)、[模型说明](https://docs.cohere.com/docs/cohere-embed) |
| Gemini | `embedContent` 可发送 inlineData；gemini-embedding-2 支持多模态，gemini-embedding-001 只支持文字。多个输入可能合成一条向量 | 保留现有原生实现，不重复之前已成功的文字和图片请求。原生请求与 OpenAI 文字转换分别展示。[官方说明](https://ai.google.dev/gemini-api/docs/embeddings) |
| Vertex | 使用服务账号、项目和区域生成授权，Google 模型使用 publisher 资源路径 | 原生 `embedContent` 路由保留 publisher 路径和 inlineData，已用合成服务核对准备后的请求。具体账号和型号仍需分别验证 |
| SiliconFlow | `/v1/embeddings` 可接收文字、text/image 对象及混合列表；`/v1/rerank` 的查询和候选分别支持结构化内容 | 补齐路由和原生执行，文字/图片对象不经过 SDK 的纯文字预检。当前网关明确支持文字及图片对象；单独 video 对象仍会拒绝。Qwen 原模型的视频能力不能当作本路径已实现能力。[Embedding](https://docs.siliconflow.cn/cn/api-reference/embeddings/create-embeddings)、[Rerank](https://docs.siliconflow.cn/cn/api-reference/rerank/create-rerank)、[Qwen 模型](https://huggingface.co/Qwen/Qwen3-VL-Embedding-8B) |
| SiliconFlow 视频 | `/v1/video/submit` 返回 requestId，`/v1/video/status` 以 POST 查询，结果包含有期限的视频 URL | 已接入带供应商前缀的原生任务路径。保留图片参考输入和原始响应，查询固定创建凭据。下载沿用供应商返回的 URL，不把它存进日志；没有虚构取消接口。仅模拟服务验证，未付费生成视频。[提交](https://docs.siliconflow.cn/docs/api/video-submit-post)、[查询](https://docs.siliconflow.cn/docs/api/video-status-post) |
| Mistral | OCR 返回文本、页面、表格及结构信息，可输入图片或文档 URL、Base64 文档，也可先上传文件。文件、签名 URL 和批任务有独立接口 | 保留 OCR 与音频接口，补齐文件上传、读取、下载、删除及批任务创建、查询、取消。OCR 的 pages_processed 单独保存为页数。未做真实 OCR 质量验证。[OCR](https://docs.mistral.ai/capabilities/document_ai/basic_ocr)、[Files](https://docs.mistral.ai/api/endpoint/files)、[Batch](https://docs.mistral.ai/api/endpoint/batch) |
| Soniox | ASR 在 api.soniox.com，文件使用 multipart，转写创建返回任务 ID。TTS 在 tts-rt.soniox.com，以 `/tts` 返回音频字节 | 新增预设，分别配置 STT 和可选 TTS 地址。保留 language、voice、audio_format 等参数。任务处理中不能删除，删除不能冒充取消。REST TTS 当前缓冲返回，尚未接入实时 WebSocket。[上传](https://soniox.com/docs/stt/api-reference/files/upload_file)、[转写](https://soniox.com/docs/stt/api-reference/transcriptions/create_transcription)、[删除限制](https://soniox.com/docs/stt/api-reference/transcriptions/delete_transcription)、[TTS](https://soniox.com/docs/api-reference/tts/generate_tts) |
| Jev | `/v1/systemone` 接收 state 与 typed questions，返回结构化决策。图片通过 assets 或 multipart 文件提供，Box 返回坐标 | 保留专用 Decisions 协议，补齐 JSON/multipart 图片与 Box。图片必须显式发送 TypeSafe-Danger-Zone-Images，不替用户打开。厂商标为实验能力，本轮没有实测质量。[说明](https://docs.typesafe.ai/)、[图片](https://docs.typesafe.ai/images.md)、[Box](https://docs.typesafe.ai/primitives/box.md) |
| Fireworks | `/inference/v1/embeddings` 与 `/inference/v1/rerank`；不同部署路径与模型 ID 不能互换 | 保留 Embedding 并补上 Rerank。既有 accounts/fireworks/models 前缀不改。专用部署是否可用仍由账号和型号决定。[指南](https://docs.fireworks.ai/guides/querying-embeddings-models)、[Rerank](https://docs.fireworks.ai/api-reference/rerank-documents) |
| Featherless | `/v1/embeddings` 官方说明使用字符串或字符串数组，目录以 embedding 输出模态标识 | 现有兼容原生文字路径可用作接入；没有因目录包含某型号就声称其图片输入可用。本轮未重新发模型请求。[官方说明](https://featherless.ai/docs/embeddings) |
| Nebius | Token Factory 的 `/v1/embeddings` 返回向量与 prompt_tokens/total_tokens，包含 service_tier、dimensions 等字段 | 沿用现有原生文字接口并保留参数；不声明未核验的图片服务能力。[官方接口](https://docs.tokenfactory.nebius.com/api-reference/inference/create-embeddings) |
| NVIDIA | NIM 多模态检索有自己的 query/passages 格式。托管重排可能使用 ai.api.nvidia.com 下的 `/v1/retrieval/.../reranking`，并不等同于 `/v1/rerank` | 现有聊天与文字 Embedding 保留。完整 NVIDIA 专用重排和多模态请求形状尚未接入，不能用通用 Rerank 入口假装支持；可继续用官方端点或现有专用客户端。[Embedding](https://docs.api.nvidia.com/nim/re/reference/nvidia-llama-nemotron-embed-vl-1b-v2-infer)、[Rerank](https://docs.api.nvidia.com/nim/reference/nvidia-llama-nemotron-rerank-vl-1b-v2-infer) |
| Cloudflare | 官方兼容入口支持 Chat 和文字 Embedding；Responses 只对指定 GPT-OSS 模型开放，且仅非流式 | 可使用供应商兼容入口；网关不自动部署 Worker。Native API 预设可保留已支持的原生 Responses；不能从网关支持流式推断 Cloudflare 也支持。[官方限制](https://developers.cloudflare.com/workers-ai/configuration/open-ai-compatibility/) |
| OpenRouter | Embeddings 文档声明文字及图片，具体模型和输入格式仍有差异 | 保留文字 Embeddings。不同多模态输入对象没有全部适配，不把 SiliconFlow 对象格式推广为 OpenRouter 的通用格式。自动参考价格优先考虑 OpenRouter 目录；手动价格和同供应商精确匹配保留。[Embeddings](https://openrouter.ai/docs/api/reference/embeddings) |

## 模型与路径证据

管理 API `/api/groups/{id}/services` 将代码已实现路由与模型资料分开返回。资料只按明确供应商及完整模型 ID 匹配，未匹配时为 null，不根据 VL 后缀推断。Embedding 的已实现接口输出为 vector，Rerank 为 relevance；查询与候选模态分别保存。

能力证据查询窗口不超过日志保留期及 30 天，每组最多扫描 200 条相关记录，每个型号最多显示 5 条不同证据，并返回截断信息。摘要不保存私人输入，结果形状检查与质量判断分开。凭据身份变化后，旧记录标为历史证据。

## 仍需专项验证的范围

本轮没有验证真实 OCR 识别质量、语音可懂度、视频内容，也没有测试订阅账号、全部模型或全部 key。ASR/TTS 实时 WebSocket、Cloudflare 原生 run 下各模型的专用格式、NVIDIA 的完整检索协议以及 OpenRouter 的全部多模态形状保留为明确限制。

非 token 单位会保留真实量或缺失状态，尚未实现全部单位的价格编辑和汇总报表。页数或图片 token 不能套用文字 token 单价。异步任务轮询不重复计费，完成后的实际消耗需要供应商账单或单独的任务结算记录，不能把每次状态查询中的累计时长重复累加。
