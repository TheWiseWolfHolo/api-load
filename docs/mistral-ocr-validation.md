# Mistral OCR 验证与文件归属

2026-10-28 核对当前官方协议，并使用一份两页合成 PDF 发起一次原生 OCR 请求。请求返回上游 429，网关日志确认只有一次上游尝试，没有识别结果。随后按用户要求停止，不改测其他型号，也不遍历凭据。因此本记录不构成 OCR 质量通过的证据。

## 协议与质量检查方法

原生入口为 `POST /v1/ocr`，访问令牌使用 `mistral` 协议，并限制相应来源与准确型号。普通 OCR 可发送 `document.type=document_url` 和 PDF Data URL，不必先上传文件。

当前官方型号 `mistral-ocr-4-1` 支持 HTML 表格、版面块、独立页眉和页脚。`pages` 使用从零开始的页号，HTML 表格可能保存在 `pages[].tables`，正文只包含引用，不能只检查 `pages[].markdown`。

合成 PDF 的两页均为栅格图像，没有隐藏文字层。样本包含中英文、金额和数量表格、页码与分栏，原文及数值预先固定。完整的后续质量检查应核对两页是否完整、数字是否准确、表格单元格是否属于正确行列，以及分栏阅读顺序；HTTP 200 本身不能证明这些条件。

普通两页 OCR 按核验时的官方单价参考约为 0.008 美元，未开启 annotation。该数字是测试前的价格参考，并非本次账单。请求没有返回 `pages_processed`，不能推断实际计费或账号余额。

供应商成功返回 `usage_info.pages_processed` 时，网关将其保存为页数。若没有 token 用量，状态为 partial；没有按页价格时保留未计价，不换算成文字 token，也不写成已知零消费。

资料来源包括 [OCR API](https://docs.mistral.ai/api/endpoint/ocr)、[OCR 4.1 型号](https://docs.mistral.ai/models/ocr-4-1)、[官方变更记录](https://docs.mistral.ai/resources/changelogs)及 [Base64 PDF 示例](https://docs.mistral.ai/resources/cookbooks/mistral-ocr-documentchunking-document_chunking_cookbook)。

## 2.1.2 文件引用修复

本地模拟服务复现了文件引用缺口。文件通过原生接口上传后，OCR 请求中的 `document.file_id` 没有进入既有资源归属检查，可能选择另一条凭据；另一个访问令牌也能把这个 ID 发往上游。这个复现没有使用真实文档或真实供应商。

2.1.2 将 FileChunk 转成既有资源引用，复用同令牌归属、凭据身份、来源和模型限制。省略 `type` 或显式设为 null 但包含 `file_id` 的请求，同样执行归属检查。空 ID、错误类型和 URL/file_id 冲突返回具体字段错误。PDF 和图片 URL 输入保留原行为。

原先失败的两个网关回归已通过，同令牌 OCR 使用上传凭据，其他令牌在发送上游前收到 404。另有 17 项请求形状检查，相关 dialect、gateway 与执行器测试通过。这些验证证明文件引用行为被修正，不能替代真实 OCR 识别质量验收。

FileChunk 的默认类型依据 [官方 Python SDK](https://raw.githubusercontent.com/mistralai/client-python/main/src/mistralai/client/models/filechunk.py)。

## 下游能力边界

客户端需要具有 OCR 操作入口，仅列出 OCR 型号不能证明其能发送文档请求。

DEEIX 的原生 OCR 客户端可配置完整的 `/v1/ocr` 地址，并读取 `pages[].markdown`。需要检查客户端是否还消费独立表格内容，不能把网关保留 HTML 表格直接等同于下游完整展示。Yoitsu 的 Embedding 连接和 New API 的文字检索兼容路径，也不自动增加文档 OCR 功能。

本轮没有开启任何客户端的自动 OCR，没有处理现有文档或重建索引。真实服务质量仍待有效的 OCR 响应验证。
