#!/usr/bin/env bash

# Thư mục chứa hooks của Git
HOOK_DIR=".git/hooks"
HOOK_FILE="$HOOK_DIR/commit-msg"

# Kiểm tra thư mục .git có tồn tại không
if [ ! -d ".git" ]; then
  echo "❌ Lỗi: Bạn phải chạy script này từ thư mục gốc của dự án (nơi có thư mục .git)."
  exit 1
fi

echo "🔧 Đang thiết lập Local Git Hook cho Commit Message..."

# Tạo thư mục hooks nếu chưa tồn tại
mkdir -p "$HOOK_DIR"

# Tạo file commit-msg hook
cat << 'EOF' > "$HOOK_FILE"
#!/usr/bin/env bash

# Chạy script kiểm tra commit message từ thư mục scripts
if [ -f "./scripts/verify-commit-msg.sh" ]; then
  exec ./scripts/verify-commit-msg.sh "$1"
else
  echo "⚠️ Cảnh báo: Không tìm thấy ./scripts/verify-commit-msg.sh. Bỏ qua kiểm tra commit."
fi
EOF

# Cấp quyền thực thi cho hook và script kiểm tra
chmod +x "$HOOK_FILE"
chmod +x ./scripts/verify-commit-msg.sh

echo "✅ Đã cài đặt thành công Git Hook tại: $HOOK_FILE"
echo "💡 Kể từ bây giờ, mỗi khi bạn chạy 'git commit', tin nhắn commit sẽ tự động được kiểm tra chuẩn Gitmoji + Conventional Commits."
