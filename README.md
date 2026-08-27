# rack-auto-ISO

用 Go 编写的 **iPXE 管理平台**：Web 编辑启动菜单、上传 ISO、作为 HTTP 引导服务，并可导出带自定义网络配置的 iPXE ISO，方便机架节点自动 chain 到本服务。

> 控制面请用下方 **curl** 拉取最新发布物部署，不要在本机 `go build` / 调试 PXE。  
> 默认监听端口：**8081**。

## 功能

- **Web 管理**：服务器 Public URL、DHCP/静态网络、菜单可视化编辑、原始脚本覆盖
- **引导服务**：`/boot.ipxe`、`/menu.ipxe`、`/files/isos/...`
- **ISO 仓库**：上传后可在菜单项里用 `iso` / `sanboot` 引用
- **导出介质**：
  - `ipxe-custom.iso`（镜像内已含 `xorriso` + `assets/`）
  - `bundle.zip` / `embed.ipxe`

---

## 部署教程（推荐：curl + Docker 最新镜像）

在 **Linux 控制面**执行（需已安装 Docker 与 Compose 插件）。脚本会用 curl 下载编排文件，再 `docker compose pull` 拉取 GHCR 上的最新镜像并启动。

### 一键部署

```bash
# 将 <装机网可达IP> 换成控制面地址
export IPXE_PUBLIC_URL=http://<装机网可达IP>:8081

curl -fsSL https://raw.githubusercontent.com/Songxwn/rack-auto-ISO/master/scripts/deploy-compose.sh | bash
```

未设置 `IPXE_PUBLIC_URL` 时，脚本会按本机 IP 自动填一个；若装机网网卡不同，请用上面的 `export` 再跑一次。

### 分步部署（同样基于 curl）

```bash
mkdir -p ~/ipxe-manager && cd ~/ipxe-manager

# 1) curl 下载编排文件
curl -fsSL -o docker-compose.yml \
  https://raw.githubusercontent.com/Songxwn/rack-auto-ISO/master/docker-compose.yml
curl -fsSL -o .env.example \
  https://raw.githubusercontent.com/Songxwn/rack-auto-ISO/master/.env.example

# 2) 配置 Public URL（装机网必须能访问）
cp -n .env.example .env
# 编辑 .env，例如:
#   IPXE_PUBLIC_URL=http://192.168.1.10:8081
#   IPXE_IMAGE_TAG=latest

# 3) curl 语义上的“下最新镜像”：拉取 GHCR latest 并启动
docker compose pull
docker compose up -d

# 4) 验证
curl -fsS "http://127.0.0.1:8081/api/health"
```

固定版本时把 `.env` 里 `IPXE_IMAGE_TAG` 设为 `v0.1.2`（或其它 [Release](https://github.com/Songxwn/rack-auto-ISO/releases) 标签），再执行 `docker compose pull && docker compose up -d`。

镜像地址：`ghcr.io/songxwn/rack-auto-iso`（公开包，一般无需登录）。

### 部署后操作

1. 浏览器打开 Public URL（如 `http://192.168.1.10:8081`）
2. 确认 **Public URL**、导出 ISO 内嵌网络（DHCP/静态）
3. 编辑菜单 / 上传 ISO
4. 导出 `ipxe-custom.iso`，节点启动后自动 chain 到本服务

常用命令：

```bash
cd ~/ipxe-manager   # 或脚本使用的 IPXE_DEPLOY_DIR
docker compose logs -f
docker compose pull && docker compose up -d   # 升级到最新镜像
docker compose down
```

---

## 备选：curl 下载最新 Release 二进制

不跑 Docker 时，用 GitHub API + curl 拉最新 `linux_amd64` 包：

```bash
set -euo pipefail
REPO=Songxwn/rack-auto-ISO
API="https://api.github.com/repos/${REPO}/releases/latest"

TAG=$(curl -fsSL "$API" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
ASSET="ipxe-manager_${TAG}_linux_amd64.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

mkdir -p ~/ipxe-manager && cd ~/ipxe-manager
curl -fsSL -o "$ASSET" "$URL"
tar -xzf "$ASSET"
sudo apt-get install -y xorriso   # 导出 ISO 需要

./ipxe-manager \
  -listen :8081 \
  -data "$PWD/data" \
  -public-url "http://<装机网可达IP>:8081"
```

环境变量：`IPXE_LISTEN`、`IPXE_DATA`、`IPXE_PUBLIC_URL`、`IPXE_ASSETS`。

---

## iPXE 客户端入口

| URL | 作用 |
|-----|------|
| `/boot.ipxe` | 入口脚本（网络配置 + chain 菜单） |
| `/menu.ipxe` | 默认菜单（`?id=` 选其它菜单） |
| `/embed.ipxe` | 与导出 ISO 相同的内嵌脚本 |
| `/files/isos/<file>` | 已上传 ISO 的下载/sanboot 地址 |

## 发布

```bash
git tag v0.1.2
git push origin v0.1.2
```

- Release：多架构二进制 + `assets/`
- GHCR：`ghcr.io/songxwn/rack-auto-iso:vX.Y.Z` 与 `:latest`
- `master` 推送也会更新 `:latest`

## 开发说明

- 模块路径：`github.com/Songxwn/rack-auto-ISO`
- 入口：`cmd/ipxe-manager`
- 编排：`docker-compose.yml`、`scripts/deploy-compose.sh`
- CI：`.github/workflows/ci.yml`、`docker.yml`、`release.yml`
