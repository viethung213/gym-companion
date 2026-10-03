-- Default system roles for auth.roles
INSERT INTO auth.roles (id, name, description) VALUES
    ('admin', 'Administrator', 'Quản trị viên toàn quyền hệ thống'),
    ('user', 'End User', 'Người dùng tập luyện thông thường'),
    ('brand', 'Brand / Gym Partner', 'Đối tác thương hiệu / phòng tập')
ON CONFLICT (id) DO NOTHING;
