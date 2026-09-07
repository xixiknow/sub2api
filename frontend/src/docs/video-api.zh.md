# 视频 API 接入

用本站 API 密钥调用视频生成。先创建任务，再轮询状态，完成后下载视频。

价格、时长、分辨率、素材上限见本页上方「价格与特性」。实际扣费以你密钥所在分组的定价为准。

将下文中的 `<BASE_URL>` 换成站点地址（本页会自动填成当前域名），`<API_KEY>` 换成「API 密钥」里复制的 key。

## 1. 鉴权

所有接口使用同一请求头：

```http
Authorization: Bearer <API_KEY>
Content-Type: application/json
```

密钥所在分组需要开通对应视频模型。可用 `GET /v1/models` 查看当前 key 能用的模型名。

## 2. 调用流程

1. 在控制台创建 API 密钥。
2. 按模型选择创建接口，`POST` JSON，保存响应里的 `id`。
3. 每 3–5 秒 `GET /v1/videos/{id}` 查询状态。
4. `status` 为 `completed` 后，`GET /v1/videos/{id}/content` 下载视频。

创建不是幂等的。请求超时后不要立刻原样重放，以免重复扣费。

## 3. 创建接口

每个模型只接受一条创建路径，用错会返回 `DRAMA_VIDEO_INVALID_PATH`。

| 创建接口 | 模型 |
| --- | --- |
| `POST /v1/videos` | `minimax-h3`、`seedance2.0-A`、`seedance2.0-fast-A`、`seedance2.0-Mini-A`、`seedance-2.0-C`、`seedance2.5-A`、`seedance-2.5-B` |
| `POST /v1/video/generations` | `seedance2.0-B`、`seedance2.0-fast-B`、`seedance2.0-E`、`seedance2.0-F`、`seedance2.0-fast-F` |

成功时 HTTP `202`，响应头带 `Location: /v1/videos/{id}` 和 `Retry-After: 5`。请保存 `id`（形如 `vidtask_...`）。`task_id` 与 `id` 相同，仅作兼容。

```json
{
  "id": "vidtask_...",
  "task_id": "vidtask_...",
  "object": "video",
  "model": "seedance2.0-A",
  "status": "queued",
  "progress": 0,
  "created_at": 1785740000
}
```

无论从哪条路径创建，查询和下载都用 `/v1/videos/{id}`。

## 4. 查询与下载

```bash
curl --request GET "<BASE_URL>/v1/videos/<TASK_ID>" \
  --header "Authorization: Bearer <API_KEY>"
```

| 状态 | 含义 | 客户端 |
| --- | --- | --- |
| `queued` | 已受理 | 继续轮询 |
| `in_progress` | 生成中 | 继续轮询 |
| `completed` | 完成 | 下载 |
| `failed` | 失败 | 读 `error`，停止 |
| `canceled` | 已取消 | 停止 |

不要在 `queued` / `in_progress` 时调用下载。完成后：

```bash
curl --request GET "<BASE_URL>/v1/videos/<TASK_ID>/content" \
  --header "Authorization: Bearer <API_KEY>" \
  --output video.mp4
```

## 5. 计费

创建成功会先冻结费用；失败退回，完成后按实结算。

- **按秒**：费用 = 该分辨率单价 × 时长（秒）。模型：`minimax-h3`、A 系列、`seedance2.5-A`。
- **按条**：一口价，不乘时长。模型：B 系列、C、E、F、`seedance-2.5-B`。

单价和开放分辨率见本页上方表格。未列出的分辨率不可用。

## 6. 公共字段

只提交模型支持的字段。`seconds` 与 `duration` 同时出现时必须一致。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 必填，使用上方表格中的公开名 |
| `prompt` | string | 必填 |
| `seconds` | integer | 输出时长；部分模型必填，见各模型说明。兼容 `duration` |
| `resolution` | string | `480p` / `720p` / `1080p` / `4k`，以该模型有价档为准 |
| `aspect_ratio` | string | 画面比例。兼容 `ratio`、`aspectRatio` |
| `generate_audio` | boolean | 部分模型支持 |
| `references` | array | 参考素材，见下一节 |
| `task_mode` | string | 仅 B 系列：`text` / `first_frame` / `first_last_frame` / `references` |

## 7. 参考素材

素材放在 `references` 数组。`source` 必须是服务端能直接访问的公网 HTTPS URL（或 Data URI），无需登录。

```json
{
  "type": "image",
  "role": "reference",
  "source": "https://cdn.example.com/ref.png"
}
```

| 字段 | 说明 |
| --- | --- |
| `type` | `image` / `video` / `audio`，以模型是否支持为准 |
| `role` | 普通参考用 `reference`；首尾帧用 `first_frame` / `last_frame`（仅图片，且仅支持该能力的模型） |

- A 系列、`seedance2.5-A`：prompt 里用 `@图1`、`@视频1`、`@音频1` 按数组中同类素材顺序引用。
- `seedance-2.0-C`：图片用 `@Image1`、`@Image2`。
- `minimax-h3`：不强制占位符，建议在 prompt 里说明各素材用途。
- 首尾帧必须成对提交，且不要和普通参考混用。`seedance2.0-A` 的首尾帧还要求 `aspect_ratio` 为 `auto`。

## 8. 按模型示例

### `minimax-h3` — `POST /v1/videos`

按秒。时长 4–15，默认 4。分辨率 480p / 720p / 1080p。比例仅 `16:9`、`9:16`。最多 9 图 / 3 视频 / 3 音频，视频+音频合计 ≤ 3，总共 ≤ 12。无首尾帧。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "minimax-h3",
    "prompt": "一只橘猫在阳光窗台伸懒腰，镜头缓慢前推",
    "seconds": 6,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

建议先用配套 skills 整理提示词后再提交。

### `seedance2.0-A` — `POST /v1/videos`

按秒。时长 4–15，默认 4。分辨率 480p / 720p / 1080p。比例含 `auto`。最多 9 图 / 3 视频 / 3 音频，共 12。有参考时 prompt 必须引用 `@图N` / `@视频N` / `@音频N`。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-A",
    "prompt": "电影感城市夜景，人物沿湿润街道向前走，镜头平稳跟拍",
    "seconds": 8,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance2.0-fast-A` — `POST /v1/videos`

按秒，快速版。仅 480p。时长 4–15，默认 4。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-fast-A",
    "prompt": "海浪拍岸，阳光碎在水面上",
    "seconds": 4,
    "resolution": "480p"
  }'
```

### `seedance2.0-Mini-A` — `POST /v1/videos`

按秒，Mini。分辨率 480p / 720p。时长 4–15，默认 4。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-Mini-A",
    "prompt": "微距镜头下露珠从叶尖落下",
    "seconds": 4,
    "resolution": "720p"
  }'
```

### `seedance2.0-B` — `POST /v1/video/generations`

按条。时长 4–15。分辨率 480p / 720p / 1080p / 4k。支持 `adaptive`。图+音，无视频参考。支持首尾帧。可选 `generate_audio`、`web_search`、`priority`、`task_mode`。

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-B",
    "prompt": "参考人物形象，海边日落，镜头缓慢向前推进",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "generate_audio": true
  }'
```

### `seedance2.0-fast-B` — `POST /v1/video/generations`

按条，快速。时长 4–15。分辨率 480p / 720p。图+音，支持首尾帧。

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-fast-B",
    "prompt": "城市延时，车流灯轨",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance-2.0-C` — `POST /v1/videos`

按条。时长 **必须** 5–15。仅 720p。比例仅 `16:9`、`9:16`。图+音，无视频、无首尾帧。有图时 prompt 用 `@Image1` 起编号。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance-2.0-C",
    "prompt": "@Image1 中的角色走过雨后街道，霓虹倒影",
    "seconds": 8,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance2.0-E` — `POST /v1/video/generations`

按条。时长 5–15，默认 5。仅 720p。图 / 视频 / 音频，最多 15 个素材。无首尾帧。不要传 B 系列的 `web_search` / `priority` 等可选字段。

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-E",
    "prompt": "航拍群山云海，阳光从云缝落下",
    "seconds": 8,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.0-F` — `POST /v1/video/generations`

按条。时长 5–15，默认 5。分辨率 720p / 1080p。图+音，无视频参考，无首尾帧。

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-F",
    "prompt": "厨房里煎牛排，油脂滋滋作响，浅景深",
    "seconds": 6,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.0-fast-F` — `POST /v1/video/generations`

按条，快速。时长 5–15，默认 5。仅 720p。图+音，无视频、无首尾帧。

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-fast-F",
    "prompt": "咖啡杯上热气旋转，窗边晨光",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.5-A` — `POST /v1/videos`

按秒。时长 4–30，默认 4。分辨率 480p / 720p / 1080p。无 `auto` 比例。最多 30 图 / 10 视频 / 10 音频，共 50。支持首尾帧。有素材时 prompt 必须引用。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.5-A",
    "prompt": "长镜头穿过竹林，风吹叶响，光斑浮动",
    "seconds": 12,
    "resolution": "1080p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance-2.5-B` — `POST /v1/videos`

按条。时长固定 30 秒，必须传 `seconds: 30`。仅 720p。最多 30 图 / 3 视频，无音频、无首尾帧。

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance-2.5-B",
    "prompt": "纪录片风格，集市从清晨到正午的光影变化",
    "seconds": 30,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

## 9. 错误

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "DRAMA_VIDEO_INVALID_RESOLUTION",
    "message": "model seedance-2.0-C does not support resolution 1080p"
  }
}
```

| HTTP | 常见 code | 处理 |
| ---: | --- | --- |
| 400 | `DRAMA_VIDEO_INVALID_PATH` | 换对创建接口 |
| 400 | `DRAMA_VIDEO_INVALID_RESOLUTION` / `INVALID_ASPECT_RATIO` / `INVALID_SECONDS` / `INVALID_PROMPT` / `INVALID_FIELD` | 按该模型规则改参数，不要原样重试 |
| 401 | `API_KEY_REQUIRED` | 检查 `Authorization` |
| 403 | `DRAMA_VIDEO_FORBIDDEN` | 任务不属于这把 key |
| 404 | `DRAMA_VIDEO_TASK_NOT_FOUND` | 核对 `id` |
| 409 | `DRAMA_VIDEO_NOT_READY` | 尚未 `completed`，继续轮询 |
| 503 | `DRAMA_VIDEO_NO_ACCOUNT` | 当前分组没有可用账号，稍后或联系管理员 |

轮询请看任务 JSON 里的 `status` 和 `error`，不要只看 HTTP 200。
