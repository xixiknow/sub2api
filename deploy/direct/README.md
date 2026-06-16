# direct.aiigo.cloud

仿照 `api.aiigo.cloud` 新增的子域名，**复用同一个 sub2api 后端**（`localhost:8080`），不新起实例。

仓库里能落地的就是「接入配置」；域名→服务的实际映射在服务器上（BT-Panel / nginx 或 Caddy），需在服务器执行。

## 接入步骤

1. **DNS**
   在 Cloudflare zone（`97c85d7d5aba81220997351c818c23f1`）添加 `direct` 记录，指向与 `api.aiigo.cloud` 相同的服务器 IP。

2. **证书**
   在服务器上运行 `docs/certs/aiigo.cloud/direct/issue-direct.aiigo.cloud.sh`，
   用 acme.sh + Cloudflare DNS-01 签发 ECC 证书（与 api 同流程、同 token）。
   > 证书文件必须实际签发，不能手动复制 api 的 `.cer/.key`。

3. **反向代理（二选一）**
   - **Caddy**：把 `code/sub2api/deploy/direct/Caddyfile` 的 site 块并入服务器 Caddyfile。
     Caddy 会自动管证书，可跳过第 2 步的 `--install-cert`。
   - **nginx / BT-Panel**：在面板新建站点 `direct.aiigo.cloud`，
     证书指向第 2 步安装的 `/www/server/panel/vhost/cert/direct.aiigo.cloud/`，
     反代到 `http://localhost:8080`（与 api 站点配置一致）。

## 不改动的部分

客户端配置（`.hermes/config.yaml`、`code/telegram-shop/config.yaml`）保持指向 `api.aiigo.cloud`。
`direct` 只是同后端的另一个入口，是否让某个客户端改走 `direct` 由你按需手动切换。
