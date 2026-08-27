# rack-auto-ISO

用 Go 编写的 **iPXE 管理平台**：Web 编辑启动菜单、上传 ISO、作为 HTTP 引导服务，并可导出带自定义网络配置的 iPXE ISO，方便机架节点自动 chain 到本服务。

> 控制面请直接下载 [GitHub Release](../../releases) 二进制，不要在本机 `go build` / 调试 PXE。

## 功能

- **Web 管理**：服务器 Public URL、DHCP/静态网络、菜单可视化编辑、原始脚本覆盖
- **引导服务**：`/boot.ipxe`、`/menu.ipxe`、`/files/isos/...`
- **ISO 仓库**：上传后可在菜单项里用 `iso` / `sanboot` 引用
- **导出介质**：
  - `ipxe-custom.iso`（需环境有 `xorriso` + Release 自带 `assets/`）
  - `bundle.zip` / `embed.ipxe`（无 xorriso 时的备选）

## 快速部署（Linux）

```bash
# 从 Release 下载 linux_amd64 包并解压
tar -xzf ipxe-manager_vX.Y.Z_linux_amd64.tar.gz
cd ipxe-manager_vX.Y.Z_linux_amd64   # 或解压后的目录

# 可选：导出 ISO 需要
sudo apt-get install -y xorriso

./ipxe-manager \
  -listen :8080 \
  -data /var/lib/ipxe-manager \
  -public-url http://192.168.1.10:8080
```

浏览器打开 `http://192.168.1.10:8080`：

1. 填好 **Public URL**（装机网必须能访问）
2. 配置「导出 ISO 内嵌网络」（DHCP 或静态）
3. 编辑菜单 / 上传 ISO
4. 导出 `ipxe-custom.iso`，用 U 盘或虚拟光驱启动节点 → 自动联网并 chain 到本服务菜单

环境变量：`IPXE_LISTEN`、`IPXE_DATA`、`IPXE_PUBLIC_URL`、`IPXE_ASSETS`。

## iPXE 客户端入口

| URL | 作用 |
|-----|------|
| `/boot.ipxe` | 入口脚本（网络配置 + chain 菜单） |
| `/menu.ipxe` | 默认菜单（`?id=` 选其它菜单） |
| `/embed.ipxe` | 与导出 ISO 相同的内嵌脚本 |
| `/files/isos/<file>` | 已上传 ISO 的下载/sanboot 地址 |

DHCP 可将 next-server / filename 指到本机，或直接使用导出的 iPXE ISO（忽略 DHCP filename，强制 chain 到 Public URL）。

## 发布

推送版本标签即可触发 Actions 编译 iPXE assets 并发布多架构包：

```bash
git tag v0.1.0
git push origin v0.1.0
```

产物包含：`ipxe-manager` 二进制 + `assets/`（`ipxe.lkrn` / `ipxe.efi` / isolinux）。

## 开发说明

- 模块路径：`github.com/Songxwn/rack-auto-ISO`
- 入口：`cmd/ipxe-manager`
- UI 嵌入：`web/`（`embed.FS`）
- CI：`.github/workflows/ci.yml`、`.github/workflows/release.yml`
