# rack-auto-ISO

用 Go 编写的 **iPXE 管理平台**：Web 编辑启动菜单、上传 ISO、作为 HTTP 引导服务，并可导出带自定义网络配置的 iPXE ISO，方便机架节点自动 chain 到本服务。

> 控制面请用 **curl 下载最新 Release 二进制** 部署，不要在本机 `go build` / 调试 PXE。  
> 默认监听端口：**8081**。

## 功能

- **Web 管理**：服务器 Public URL、DHCP/静态网络、菜单可视化编辑、原始脚本覆盖
- **引导服务**：`/boot.ipxe`、`/menu.ipxe`、`/files/isos/...`
- **ISO 仓库**：上传后可在菜单项里用 `iso` / `sanboot` 引用
- **导出介质**：
  - `ipxe-custom.iso`：BIOS（isolinux+INITRD）+ UEFI（FAT `efi.img` ESP）isohybrid
  - `bundle.zip` / `embed.ipxe`

---

## 部署教程（curl 下载最新 Release）

在 **Linux 控制面**执行：

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

# 导出 iPXE ISO 需要
sudo apt-get update
sudo apt-get install -y xorriso dosfstools mtools

# 将 <装机网可达IP> 换成控制面地址
./ipxe-manager \
  -listen :8081 \
  -data "$PWD/data" \
  -public-url "http://<装机网可达IP>:8081"
```

验证：

```bash
curl -fsS "http://127.0.0.1:8081/api/health"
```

### 部署后操作

1. 浏览器打开 Public URL（如 `http://192.168.1.10:8081`）
2. 确认 **Public URL**、导出 ISO 内嵌网络（DHCP/静态）
3. 编辑菜单 / 上传 ISO
4. 导出 `ipxe-custom.iso`（光驱或 U 盘启动；Secure Boot 需关闭）

环境变量：`IPXE_LISTEN`、`IPXE_DATA`、`IPXE_PUBLIC_URL`、`IPXE_ASSETS`。  
arm64 将资源名中的 `linux_amd64` 换成 `linux_arm64`。

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
git tag v0.1.4
git push origin v0.1.4
```

产物：多架构二进制包 + `assets/`（`ipxe.lkrn` / `ipxe.efi` / isolinux / isohdpfx）。

## 开发说明

- 模块路径：`github.com/Songxwn/rack-auto-ISO`
- 入口：`cmd/ipxe-manager`
- UI 嵌入：`web/`（`embed.FS`）
- CI：`.github/workflows/ci.yml`、`release.yml`
