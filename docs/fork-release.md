# Fork 发版说明与约束（xixiknow/sub2api）

> 本文档描述本 fork（`xixiknow/sub2api`）独立于上游（`Wei-Shaw/sub2api`）的发版流程、
> 版本更新检查链路、以及发版时必须遵守的约束。发版前请通读一遍。

## 1. 版本更新检查链路（后台"当前版本"卡片）

管理员侧边栏的版本徽章（`frontend/src/components/common/VersionBadge.vue`）→
`GET /admin/system/check-updates` → 后端 `UpdateService.CheckUpdate`
（`backend/internal/service/update_service.go`）→ 请求
`https://api.github.com/repos/xixiknow/sub2api/releases/latest` →
`compareVersions` 与当前二进制内嵌版本逐段比较（X.Y.Z，忽略 `-rc1` 等预发布后缀）→
有更新时返回 release 信息（更新日志链接、下载地址等）。结果 Redis 缓存 20 分钟。

- **当前版本号**：构建时嵌入二进制（`backend/cmd/server/VERSION` 经 go:embed；CI 发版时
  版本号取自 git tag，VERSION 文件仅对 source 构建/本地开发生效）。
- **"立即更新"**：后端从该 release 的 assets 下载对应平台压缩包（≤500MB，域名白名单限
  `github.com` / `objects.githubusercontent.com`），sha256 校验后原子替换二进制，随后提示
  重启。**只有 release 构建的二进制（BuildType=release）可用；source 构建只显示 git pull 提示。**
- **"查看更新日志"**：链接来自 GitHub API 返回的 release 页面 URL，指向本 fork 的 release。
- **版本回滚**：取本 fork 仓库中严格小于当前版本的最近 3 个 release；脚本部署用
  `curl -sSL https://raw.githubusercontent.com/xixiknow/sub2api/<tag>/deploy/install.sh | bash -s -- rollback <tag>`，
  Docker 部署用 `ghcr.io/xixiknow/sub2api:<version>` 镜像 tag。

## 2. 发版流程（tag 驱动，与分支无关）

CI（`.github/workflows/release.yml`）完全由 tag 触发，**构建的是 tag 指向的 commit，不从
main 构建**——tag 打在任何分支上都可以（本 fork 的发版分支即可）。

1. **（可选）预演**：GitHub Actions → Release → 手动 `workflow_dispatch`，输入分支/ref 并勾选
   `dry_run`，只构建不发布，验证整条流水线。
2. **打 tag 并推送**：
   ```bash
   git tag vX.Y.Z <发版commit>
   git push origin vX.Y.Z
   ```
3. CI 自动完成：前端构建 → 多平台二进制 + `checksums.txt` → GHCR 镜像
   `ghcr.io/xixiknow/sub2api:{X.Y.Z,latest,M.M, M}` → 在本 fork 发布 GitHub Release。
4. 发布成功后 CI 自动把 `backend/cmd/server/VERSION` 同步提交到**默认分支（main）**
   （`sync-version-file` job）。main 上的 VERSION 文件由 CI 维护，**不要手动编辑**。
5. 部署新版本后，在后台版本卡片点刷新，应看到"最新版本"来自本 fork 的 release。

## 3. 发版约束

| # | 约束 | 原因 |
|---|------|------|
| 1 | tag 必须 `v` 前缀 + 严格 semver：`X.Y.Z` 或 `X.Y.Z-后缀`（正则 `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?`） | `release_matrix.py` 校验，不合规格直接失败 |
| 2 | 新版本号必须**严格大于**线上已部署版本，客户端才会提示更新 | `compareVersions` 逐段比较 |
| 3 | 预发布后缀（`-rc1`）在版本比较中被忽略：`v0.2.16-rc1` 与 `v0.2.16` 视为同一版本 | `parseVersion` 截断后缀后比较；正式版发布后 `-rc1` 用户不会收到更新提示，正式版号必须递增 |
| 4 | 预览版请在 GitHub Release 上勾选 **prerelease** | `/releases/latest` 只选中正式 release（不含 draft/prerelease），避免预览版推给所有用户 |
| 5 | **禁止重打/移动已发布的版本号 tag** | 更新检查结果缓存 20 分钟；且同版本号比较结果为"无更新"，已部署用户收不到推送 |
| 6 | 打 tag 的 commit 必须包含当次发版所用的 `.github/workflows/release.yml` 与 `.github/release-tools/` | push tag 事件使用 tag commit 上的 workflow 与工具版本 |
| 7 | release assets 必须包含平台压缩包与 `checksums.txt`（CI 自动产出，勿手动删改） | "立即更新"与回滚都依赖这两类 asset |
| 8 | `UPDATE_GITHUB_TOKEN` 环境变量可选配置 | GitHub API 匿名限流（60 次/小时/IP）时避免检查失败 |
| 9 | 回滚到**切换仓库之前**的旧 tag（v0.2.0–v0.2.4）时，脚本内容仍是上游版（旧 tag 内文件不会变） | 旧 tag 两仓库内容一致无实际风险，但回滚目标建议选择切换之后发布的版本 |

## 4. 首次发版清单

- [ ] 线上正在运行的二进制为 v0.2.7（旧构建）；首个 fork tag 必须 **> 0.2.7**，建议直接用
      `v0.2.15`（与当前代码内 VERSION 一致）或更高。
- [ ] 确认 tag 指向的 commit 已包含本次更新源切换（`update_service.go`、`VersionBadge.vue`、
      `deploy/install.sh`、`deploy/docker-deploy.sh`、`deploy/docker-compose*.yml`）。
- [ ] 首次发版前可先 `workflow_dispatch` + `dry_run` 预演。
- [ ] 发版后验证：`curl https://api.github.com/repos/xixiknow/sub2api/releases/latest`
      返回新 release；后台版本卡片刷新后"最新版本"显示 fork 的版本号。
- [ ] Docker 部署注意：`deploy/docker-compose*.yml` 默认镜像已切到
      `ghcr.io/xixiknow/sub2api:latest`；已部署环境若通过 `.env` 覆盖过 `SUB2API_IMAGE`，
      需同步改为 fork 镜像。

## 5. 与上游保持同步

- 合并上游代码：`git fetch upstream && git merge upstream/main`（本仓库 `upstream` remote
  指向 `Wei-Shaw/sub2api`）。
- Go module 路径仍为 `github.com/Wei-Shaw/sub2api`（import 路径，**不是**发版仓库引用，
  不要"顺手"改成 fork 路径——那会破坏所有 import 并失去与上游同步的能力）。
- 更新源相关硬编码共 5 处文件，同步上游时若冲突需保留 fork 值：
  `backend/internal/service/update_service.go`（`githubRepo`）、
  `frontend/src/components/common/VersionBadge.vue`（`GITHUB_REPO`/`DOCKER_IMAGE`）、
  `deploy/install.sh`（`GITHUB_REPO` 等 3 处）、`deploy/docker-deploy.sh`（`GITHUB_RAW_URL`）、
  `deploy/docker-compose*.yml`（默认镜像）。
