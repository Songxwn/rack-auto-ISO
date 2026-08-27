#!/usr/bin/env bash
# 在 Linux 控制面用 curl 拉取编排文件并部署最新 GHCR 镜像。
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/Songxwn/rack-auto-ISO/master/scripts/deploy-compose.sh | bash
# 或:
#   IPXE_PUBLIC_URL=http://192.168.1.10:8081 bash deploy-compose.sh

set -euo pipefail

REPO_RAW="${IPXE_REPO_RAW:-https://raw.githubusercontent.com/Songxwn/rack-auto-ISO/master}"
DIR="${IPXE_DEPLOY_DIR:-$PWD/ipxe-manager}"
IMAGE_TAG="${IPXE_IMAGE_TAG:-latest}"
HOST_PORT="${IPXE_HOST_PORT:-8081}"
PUBLIC_URL="${IPXE_PUBLIC_URL:-}"

if [[ -z "$PUBLIC_URL" ]]; then
  # 尝试猜测本机首个非回环 IPv4
  GUESS_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}' || true)"
  if [[ -z "$GUESS_IP" ]]; then
    GUESS_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  if [[ -n "$GUESS_IP" ]]; then
    PUBLIC_URL="http://${GUESS_IP}:${HOST_PORT}"
  else
    PUBLIC_URL="http://127.0.0.1:${HOST_PORT}"
  fi
  echo "==> IPXE_PUBLIC_URL 未设置，使用 ${PUBLIC_URL}"
  echo "    装机网若不同，请重新导出: IPXE_PUBLIC_URL=http://<可达IP>:${HOST_PORT} $0"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "需要已安装 Docker（含 docker compose 插件）" >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "需要 docker compose 插件" >&2
  exit 1
fi

mkdir -p "$DIR"
cd "$DIR"

echo "==> curl 下载 docker-compose.yml / .env.example"
curl -fsSL "${REPO_RAW}/docker-compose.yml" -o docker-compose.yml
curl -fsSL "${REPO_RAW}/.env.example" -o .env.example

if [[ ! -f .env ]]; then
  cp .env.example .env
fi

# 写入/更新关键配置
tmp="$(mktemp)"
grep -vE '^(IPXE_PUBLIC_URL|IPXE_HOST_PORT|IPXE_IMAGE_TAG)=' .env >"$tmp" || true
{
  echo "IPXE_PUBLIC_URL=${PUBLIC_URL}"
  echo "IPXE_HOST_PORT=${HOST_PORT}"
  echo "IPXE_IMAGE_TAG=${IMAGE_TAG}"
  cat "$tmp"
} >.env
rm -f "$tmp"

echo "==> 拉取最新镜像 ghcr.io/songxwn/rack-auto-iso:${IMAGE_TAG}"
docker compose pull

echo "==> 启动"
docker compose up -d

echo
echo "部署完成"
echo "  目录:   ${DIR}"
echo "  Web:    ${PUBLIC_URL}"
echo "  健康检查: curl -fsS ${PUBLIC_URL}/api/health"
echo "  日志:   docker compose -f ${DIR}/docker-compose.yml logs -f"
