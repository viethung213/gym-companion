#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# SCRIPT DEPLOY FITAI SERVICES TRÊN SINGLE VPS (BẢN TỰ ĐỘNG SO SÁNH IMAGE)
# ==============================================================================

DEPLOY_DIR="/opt/gym-companion"
COMPOSE_FILE="docker-compose.prod.yml"
SERVICE_NAME="${1:-all}"
IMAGE_TAG="${2:-latest}"
GHCR_OWNER="${GHCR_OWNER:-viethung213}"

cd "$DEPLOY_DIR"

echo "=================================================================="
echo " [$(date +'%Y-%m-%d %H:%M:%S')] BẮT ĐẦU KIỂM TRA & TRIỂN KHAI"
echo " Target Service   : ${SERVICE_NAME}"
echo " Image Tag        : ${IMAGE_TAG}"
echo " GitHub Registry  : ghcr.io/${GHCR_OWNER}"
echo "=================================================================="

# 1. ĐĂNG NHẬP GHCR
TOKEN_FILE="./secrets/ghcr-token.txt"
if [ -f "$TOKEN_FILE" ]; then
    cat "$TOKEN_FILE" | docker login ghcr.io -u "$GHCR_OWNER" --password-stdin > /dev/null 2>&1
    echo ">> [1/4] Đăng nhập GitHub Container Registry thành công."
else
    echo ">> [1/4] Dùng session Docker hiện tại trên máy..."
fi

export GHCR_OWNER="$GHCR_OWNER"
export IMAGE_TAG="$IMAGE_TAG"

# 2. HÀM DEPLOY THÔNG MINH CHO 1 SERVICE
deploy_single_service() {
    local SVC="$1"
    local FULL_IMAGE="ghcr.io/${GHCR_OWNER}/fitai-${SVC}:${IMAGE_TAG}"

    echo "--------------------------------------------------"
    echo ">> Đang kiểm tra service: [${SVC}]"

    # Lấy Image ID hiện tại trên VPS trước khi pull
    OLD_IMAGE_ID=$(docker images -q "$FULL_IMAGE" 2>/dev/null || echo "none")

    # Thử kéo image mới nhất từ GitHub
    echo "   -> Đang kiểm tra phiên bản mới trên GitHub..."
    if ! docker compose -f "$COMPOSE_FILE" pull "$SVC" 2>&1; then
        echo "   [!] CẢNH BÁO: Chưa tìm thấy image '$FULL_IMAGE' trên GitHub Packages."
        echo "       (Hãy đảm bảo bạn đã chạy workflow build cho service này trên GitHub)"
        return 0
    fi

    # Lấy Image ID sau khi pull
    NEW_IMAGE_ID=$(docker images -q "$FULL_IMAGE" 2>/dev/null || echo "none")

    # Kiểm tra xem container hiện tại có đang chạy không
    CONTAINER_RUNNING=$(docker ps -q -f name="fitai-${SVC}" 2>/dev/null || echo "")

    # So sánh: Nếu image không đổi và container đang chạy ngon lành -> Bỏ qua
    if [ "$OLD_IMAGE_ID" = "$NEW_IMAGE_ID" ] && [ -n "$CONTAINER_RUNNING" ] && [ "$OLD_IMAGE_ID" != "none" ]; then
        echo "   -> [INFO] Image của '${SVC}' trên GitHub KHÔNG THAY ĐỔI."
        echo "   -> Container 'fitai-${SVC}' đang chạy bản mới nhất. Bỏ qua khởi động lại!"
        return 0
    fi

    # Nếu là lần đầu hoặc có image mới -> Khởi chạy container
    echo "   -> [UPDATE] Phát hiện thay đổi hoặc container chưa chạy. Đang cập nhật..."
    docker compose -f "$COMPOSE_FILE" up -d --no-deps "$SVC"

    # Kiểm tra trạng thái
    sleep 3
    local STATUS
    STATUS=$(docker inspect --format='{{.State.Status}}' "fitai-${SVC}" 2>/dev/null || echo "not_found")
    echo "   -> Trạng thái container 'fitai-${SVC}': ${STATUS}"
}

# 3. THỰC THI KIỂM TRA
echo ">> [2/4] Kiểm tra phiên bản image từ GitHub..."
docker network inspect fitai-network >/dev/null 2>&1 || docker network create fitai-network
if [ "$SERVICE_NAME" = "all" ]; then
    # Duyệt qua từng service đã cấu hình
    SERVICES=("gateway" "kafka" "redis" "core" "ai")
    for S in "${SERVICES[@]}"; do
        deploy_single_service "$S"
    done
else
    deploy_single_service "$SERVICE_NAME"
fi

# 4. DỌN DẸP DỮ LIỆU RÁC (CHỐNG TRÀN Ổ CỨNG)
echo "--------------------------------------------------"
echo ">> [3/4] Dọn dẹp layer/image cũ không còn dùng..."
docker image prune -f > /dev/null 2>&1

echo ">> [4/4] Kiểm tra danh sách container đang chạy:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo "=================================================================="
echo " [$(date +'%Y-%m-%d %H:%M:%S')] QUÁ TRÌNH KIỂM TRA HOÀN TẤT!"
echo "=================================================================="

