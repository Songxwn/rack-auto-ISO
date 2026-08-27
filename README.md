# rack-auto-ISO

用 Go 编写的 **iPXE 管理平台**：Web 编辑启动菜单、上传 ISO、作为 HTTP 引导服务，并可导出带自定义网络配置的 iPXE ISO，方便机架节点自动 chain 到本服务。

> 控制面请直接下载 [GitHub Release](../../releases) 二进制或使用 GHCR 镜像，不要在本机 `go build` / 调试 PXE。  
> 默认监听端口：**8081**。

## 功能

- **Web 管理**：服务器 Public URL、DHCP/静态网络、菜单可视化编辑、原始脚本覆盖
- **引导服务**：`/boot.ipxe`、`/menu.ipxe`、`/files/isos/...`
- **ISO 仓库**：上传后可在菜单项里用 `iso` / `sanboot` 引用
- **导出介质**：
  - `ipxe-custom.iso`（需环境有 `xorriso` + Release/`assets/`；镜像内已含 xorriso）
  - `bundle.zip` / `embed.ipxe`（无 xorriso 时的备选）

## Docker Compose（推荐）

镜像：`ghcr.io/songxwn/rack-auto-iso`（`master` 推送打 `:latest`，版本标签打 `:vX.Y.Z`）。

```bash
# 若 GHCR 包尚未公开，先登录（公开仓库包可在 GitHub Packages 设为 Public）
# echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

export IPXE_PUBLIC_URL=http://192.168.1.10:8081
docker compose up -d
```

浏览器打开 `http://192.168.1.10:8081`。数据卷 `ipxe-data` 持久化菜单与上传的 ISO。

可选环境变量：`IPXE_HOST_PORT`（宿主机端口，默认 8081）、`IPXE_IMAGE_TAG`（默认 `latest`）。

## 快速部署（二进制）

```bash
tar -xzf ipxe-manager_vX.Y.Z_linux_amd64.tar.gz
# 可选：导出 ISO 需要（Release 包外若无 xorriso）
sudo apt-get install -y xorriso

./ipxe-manager \
  -listen :8081 \
  -data /var/lib/ipxe-manager \
  -public-url http://192.168.1.10:8081
```

1. 填好 **Public URL**（装机网必须能访问）
2. 配置「导出 ISO 内嵌网络」（DHCP 或静态）
3. 编辑菜单 / 上传 ISO
4. 导出 `ipxe-custom.iso`，启动节点 → 自动联网并 chain 到本服务菜单

环境变量：`IPXE_LISTEN`、`IPXE_DATA`、`IPXE_PUBLIC_URL`、`IPXE_ASSETS`。

## iPXE 客户端入口

| URL | 作用 |
|-----|------|
| `/boot.ipxe` | 入口脚本（网络配置 + chain 菜单） |
| `/menu.ipxe` | 默认菜单（`?id=` 选其它菜单） |
| `/embed.ipxe` | 与导出 ISO 相同的内嵌脚本 |
| `/files/isos/<file>` | 已上传 ISO 的下载/sanboot 地址 |

## 发布

```bash
git tag v0.1.1
git push origin v0.1.1
```

- Release：多架构二进制包 + `assets/`
- GHCR：`ghcr.io/songxwn/rack-auto-iso:v0.1.1` 与 `:latest`
- `master` 每次推送也会构建并推送 `:latest` / `:sha-xxxxxxx`

## 开发说明

- 模块路径：`github.com/Songxwn/rack-auto-ISO`
- 入口：`cmd/ipxe-manager`
- UI 嵌入：`web/`（`embed.FS`）
- 编排：`docker-compose.yml`、`Dockerfile`
- CI：`.github/workflows/ci.yml`、`docker.yml`、`release.yml`
