# Video API

Create a video job with your site API key, poll until it finishes, then download the file.

Prices, duration, resolutions, and reference limits are in **Prices & capabilities** above. Your group pricing still applies at request time.

Replace `<BASE_URL>` with this site (this page fills in the current origin) and `<API_KEY>` with a key from **API Keys**.

## 1. Auth

All endpoints use the same headers:

```http
Authorization: Bearer <API_KEY>
Content-Type: application/json
```

The key's group must allow the video model. `GET /v1/models` lists models available to the key.

## 2. Flow

1. Create an API key in the console.
2. `POST` JSON to the model's create endpoint and save `id`.
3. Poll `GET /v1/videos/{id}` every 3–5 seconds.
4. When `status` is `completed`, download with `GET /v1/videos/{id}/content`.

Create is not idempotent. Do not immediately replay a timed-out request or you may be charged twice.

## 3. Create endpoints

Each model accepts only one create path. The wrong path returns `DRAMA_VIDEO_INVALID_PATH`.

| Endpoint | Models |
| --- | --- |
| `POST /v1/videos` | `minimax-h3`, `seedance2.0-A`, `seedance-2.0-C`, `seedance2.5-A`, `seedance-2.5-B` |
| `POST /v1/video/generations` | `seedance2.0-B`, `seedance2.0-fast-B`, `seedance2.0-E`, `seedance2.0-F`, `seedance2.0-fast-F` |

Success is HTTP `202` with `Location: /v1/videos/{id}` and `Retry-After: 5`. Save `id` (`vidtask_...`). `task_id` matches `id` for compatibility.

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

Poll and download always use `/v1/videos/{id}`, regardless of the create path.

## 4. Poll and download

```bash
curl --request GET "<BASE_URL>/v1/videos/<TASK_ID>" \
  --header "Authorization: Bearer <API_KEY>"
```

| Status | Meaning | Client |
| --- | --- | --- |
| `queued` | Accepted | Keep polling |
| `in_progress` | Generating | Keep polling |
| `completed` | Done | Download |
| `failed` | Failed | Read `error`, stop |
| `canceled` | Canceled | Stop |

Do not download while `queued` or `in_progress`. When complete:

```bash
curl --request GET "<BASE_URL>/v1/videos/<TASK_ID>/content" \
  --header "Authorization: Bearer <API_KEY>" \
  --output video.mp4
```

## 5. Billing

A successful create holds balance first. Failures release it; completion captures the actual cost.

- **Per second**: price × duration. Models: `minimax-h3`, A-series, `seedance2.5-A`.
- **Per clip**: flat fee, duration does not multiply. Models: B-series, C, E, F, `seedance-2.5-B`.

See the table above for unit prices. Resolutions not listed are unavailable.

## 6. Common fields

Only send fields the model supports. If both `seconds` and `duration` are present they must match.

| Field | Type | Notes |
| --- | --- | --- |
| `model` | string | Required. Use a public name from the table |
| `prompt` | string | Required |
| `seconds` | integer | Output length; required for some models. Alias `duration` |
| `resolution` | string | `480p` / `720p` / `1080p` / `4k` as priced for the model |
| `aspect_ratio` | string | Aspect ratio. Aliases `ratio`, `aspectRatio` |
| `generate_audio` | boolean | Some models |
| `references` | array | See the next section |
| `task_mode` | string | B-series only: `text` / `first_frame` / `first_last_frame` / `references` |

## 7. References

Put assets in `references`. `source` must be a public HTTPS URL (or Data URI) the server can fetch with no login. The console asset library submits `asset://<id>`; this site rewrites it to a signed HTTPS URL before calling upstream.

```json
{
  "type": "image",
  "role": "reference",
  "source": "https://cdn.example.com/ref.png"
}
```

| Field | Notes |
| --- | --- |
| `type` | `image` / `video` / `audio`, if the model allows it |
| `role` | `reference` for normal refs; `first_frame` / `last_frame` for stills on models that support them |

- A-series and `seedance2.5-A`: cite `@图1`, `@视频1`, `@音频1` in prompt order.
- `seedance-2.0-C`: cite images as `@Image1`, `@Image2`.
- `minimax-h3`: placeholders are optional; describe how each asset is used.
- First/last frames must be a pair and must not mix with normal refs. `seedance2.0-A` also requires `aspect_ratio` `auto`.

## 8. Examples

### `minimax-h3` — `POST /v1/videos`

Per second. 4–15s, default 4. 480p / 720p / 1080p. Aspect `16:9` or `9:16` only. Up to 9 images / 3 videos / 3 audios; video+audio combined ≤ 3; 12 total. No first/last frames.

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "minimax-h3",
    "prompt": "An orange cat stretches on a sunny windowsill, slow push-in",
    "seconds": 6,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance2.0-A` — `POST /v1/videos`

Per second. 4–15s, default 4. 480p / 720p / 1080p. Aspect includes `auto`. Up to 9 images / 3 videos / 3 audios, 12 total. Prompts with refs must cite `@图N` / `@视频N` / `@音频N`.

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-A",
    "prompt": "Cinematic rainy city street, tracking shot following a walker",
    "seconds": 8,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance2.0-B` — `POST /v1/video/generations`

Per clip. 4–15s. 480p / 720p / 1080p / 4k. Supports `adaptive`. Images + audio, no video refs. First/last frames allowed. Optional `generate_audio`, `web_search`, `priority`, `task_mode`.

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-B",
    "prompt": "Keep the character likeness, sunset on the beach, slow dolly in",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "generate_audio": true
  }'
```

### `seedance2.0-fast-B` — `POST /v1/video/generations`

Per clip, fast. 4–15s. 480p / 720p. Images + audio. First/last frames allowed.

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-fast-B",
    "prompt": "City timelapse with light trails",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance-2.0-C` — `POST /v1/videos`

Per clip. Duration **required**, 5–15s. 720p only. Aspect `16:9` or `9:16`. Images + audio, no video, no first/last frames. Cite images as `@Image1`.

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance-2.0-C",
    "prompt": "The character in @Image1 walks a rainy street with neon reflections",
    "seconds": 8,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance2.0-E` — `POST /v1/video/generations`

Per clip. 5–15s, default 5. 720p only. Images / video / audio, up to 15 refs. No first/last frames. Do not send B-series optional fields such as `web_search` or `priority`.

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-E",
    "prompt": "Aerial mountains above the clouds, sunbeams through gaps",
    "seconds": 8,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.0-F` — `POST /v1/video/generations`

Per clip. 5–15s, default 5. 720p / 1080p. Images + audio, no video refs, no first/last frames.

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-F",
    "prompt": "Steak sizzling in a pan, shallow depth of field",
    "seconds": 6,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.0-fast-F` — `POST /v1/video/generations`

Per clip, fast. 5–15s, default 5. 720p only. Images + audio, no video, no first/last frames.

```bash
curl --request POST "<BASE_URL>/v1/video/generations" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.0-fast-F",
    "prompt": "Steam curling from a coffee cup in morning window light",
    "seconds": 5,
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }'
```

### `seedance2.5-A` — `POST /v1/videos`

Per second. 4–30s, default 4. 480p / 720p / 1080p. No `auto` aspect. Up to 30 images / 10 videos / 10 audios, 50 total. First/last frames allowed. Prompts with assets must cite them.

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance2.5-A",
    "prompt": "A long take through a bamboo grove, wind in the leaves",
    "seconds": 12,
    "resolution": "1080p",
    "aspect_ratio": "16:9"
  }'
```

### `seedance-2.5-B` — `POST /v1/videos`

Per clip. Duration is fixed at 30s (`seconds: 30` required). 720p only. Up to 30 images / 3 videos, no audio, no first/last frames.

```bash
curl --request POST "<BASE_URL>/v1/videos" \
  --header "Authorization: Bearer <API_KEY>" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "seedance-2.5-B",
    "prompt": "Documentary light on a market from dawn to noon",
    "seconds": 30,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

## 9. Errors

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "DRAMA_VIDEO_INVALID_RESOLUTION",
    "message": "model seedance-2.0-C does not support resolution 1080p"
  }
}
```

| HTTP | Common code | Action |
| ---: | --- | --- |
| 400 | `DRAMA_VIDEO_INVALID_PATH` | Use the model's create endpoint |
| 400 | `DRAMA_VIDEO_INVALID_RESOLUTION` / `INVALID_ASPECT_RATIO` / `INVALID_SECONDS` / `INVALID_PROMPT` / `INVALID_FIELD` | Fix the field; do not retry unchanged |
| 401 | `API_KEY_REQUIRED` | Check `Authorization` |
| 403 | `DRAMA_VIDEO_FORBIDDEN` | The job does not belong to this key |
| 404 | `DRAMA_VIDEO_TASK_NOT_FOUND` | Check `id` |
| 409 | `DRAMA_VIDEO_NOT_READY` | Still not `completed`; keep polling |
| 410 | `DRAMA_VIDEO_CONTENT_MISSING` | The file expired or was removed; download is no longer available |
| 503 | `DRAMA_VIDEO_NO_ACCOUNT` | No account available in this group |

When polling, read `status` and `error` in the JSON. Do not treat HTTP 200 as success by itself.

## 10. Retention and data safety

Generated videos and uploaded assets are a temporary relay. We do not guarantee files will not be lost, corrupted, or unrecoverable. Download and keep your own copies.

- Task JSON may include `expires_at` (Unix seconds). After that, `GET /v1/videos/{id}/content` returns 410.
- Default retention is about 30 days. Admins can change it in system settings (`0` means no automatic cleanup). Changing the setting does not rewrite expiry on already completed jobs.
- Unused library assets are also cleaned up after the asset retention window. Do not upload content you do not have the right to use.
