#!/usr/bin/env bash

# Thư mục chứa hooks của Git
HOOK_DIR=".git/hooks"
HOOK_COMMIT_MSG="$HOOK_DIR/commit-msg"
HOOK_PREPARE_MSG="$HOOK_DIR/prepare-commit-msg"

# Kiểm tra thư mục .git có tồn tại không
if [ ! -d ".git" ]; then
  echo "❌ Lỗi: Bạn phải chạy script này từ thư mục gốc của dự án (nơi có thư mục .git)."
  exit 1
fi

echo "🔧 Đang thiết lập Local Git Hooks cho dự án..."

# Tạo thư mục hooks nếu chưa tồn tại
mkdir -p "$HOOK_DIR"

# 1. Tạo file commit-msg hook (Xác thực thông điệp commit)
cat << 'EOF' > "$HOOK_COMMIT_MSG"
#!/usr/bin/env bash

# Chạy script kiểm tra commit message từ thư mục scripts
if [ -f "./scripts/verify-commit-msg.sh" ]; then
  exec ./scripts/verify-commit-msg.sh "$1"
else
  echo "⚠️ Cảnh báo: Không tìm thấy ./scripts/verify-commit-msg.sh. Bỏ qua kiểm tra commit."
fi
EOF

# 2. Tạo file prepare-commit-msg hook (Tự động chèn emoji cho merge commit)
cat << 'EOF' > "$HOOK_PREPARE_MSG"
#!/bin/sh
COMMIT_MSG_FILE=$1
COMMIT_SOURCE=$2

if [ "$COMMIT_SOURCE" = "merge" ]; then
  ORIGINAL_MSG=$(cat "$COMMIT_MSG_FILE")
  echo ":twisted_rightwards_arrows: chore(merge): $ORIGINAL_MSG" > "$COMMIT_MSG_FILE"
fi
EOF

# Cấp quyền thực thi cho các hooks và script kiểm tra
chmod +x "$HOOK_COMMIT_MSG"
chmod +x "$HOOK_PREPARE_MSG"
chmod +x ./scripts/verify-commit-msg.sh

echo "✅ Đã cài đặt thành công Git Hooks tại: $HOOK_DIR"
echo "💡 Kể từ bây giờ:"
echo "   - Mỗi khi bạn commit, tin nhắn sẽ được kiểm tra chuẩn Gitmoji + Conventional Commits."
echo "   - Mỗi khi bạn merge, tin nhắn merge sẽ tự động được thêm emoji và kiểu tương thích."
