-- Migration: 09-seed-food-items.sql
-- Description: Comprehensive Seed Data for Food Catalog and NutiFood Products (200+ Food Items per 100g)

INSERT INTO nutrition.food_items (name, category, calories_per_100g, protein_per_100g, carbs_per_100g, fat_per_100g, allergen_tags, protein_source, carb_source, is_nutifood_product, status)
VALUES
    -- =========================================================================
    -- 1. PROTEIN SOURCES (Nguồn Đạm - Động vật & Thực vật)
    -- =========================================================================
    ('Ức gà không da tươi', 'PROTEIN', 165.0, 31.0, 0.0, 3.6, '[]'::jsonb, 'CHICKEN', NULL, FALSE, 'Active'),
    ('Đùi gà tháo da', 'PROTEIN', 209.0, 26.0, 0.0, 10.9, '[]'::jsonb, 'CHICKEN', NULL, FALSE, 'Active'),
    ('Cánh gà nướng', 'PROTEIN', 290.0, 27.0, 0.0, 19.5, '[]'::jsonb, 'CHICKEN', NULL, FALSE, 'Active'),
    ('Thịt gà xay nạc', 'PROTEIN', 143.0, 23.0, 0.0, 5.7, '[]'::jsonb, 'CHICKEN', NULL, FALSE, 'Active'),
    ('Gà tây phi lê', 'PROTEIN', 135.0, 30.0, 0.0, 1.5, '[]'::jsonb, 'TURKEY', NULL, FALSE, 'Active'),

    ('Thịt bò nạc mông', 'PROTEIN', 250.0, 26.0, 0.0, 15.0, '[]'::jsonb, 'BEEF', NULL, FALSE, 'Active'),
    ('Thăn bò Úc', 'PROTEIN', 217.0, 26.1, 0.0, 11.8, '[]'::jsonb, 'BEEF', NULL, FALSE, 'Active'),
    ('Bắp bò tươi', 'PROTEIN', 201.0, 34.0, 0.0, 6.3, '[]'::jsonb, 'BEEF', NULL, FALSE, 'Active'),
    ('Thịt bò xay 90/10', 'PROTEIN', 176.0, 20.0, 0.0, 10.0, '[]'::jsonb, 'BEEF', NULL, FALSE, 'Active'),
    ('Sườn bò nướng', 'PROTEIN', 290.0, 24.0, 0.0, 21.0, '[]'::jsonb, 'BEEF', NULL, FALSE, 'Active'),

    ('Thịt heo thăn nạc', 'PROTEIN', 143.0, 26.0, 0.0, 3.5, '[]'::jsonb, 'PORK', NULL, FALSE, 'Active'),
    ('Thịt heo vai nạc', 'PROTEIN', 186.0, 21.0, 0.0, 11.0, '[]'::jsonb, 'PORK', NULL, FALSE, 'Active'),
    ('Ba chỉ heo tươi', 'PROTEIN', 518.0, 9.0, 0.0, 53.0, '[]'::jsonb, 'PORK', NULL, FALSE, 'Active'),
    ('Chả lụa heo nạc', 'PROTEIN', 136.0, 15.0, 2.0, 7.5, '[]'::jsonb, 'PORK', NULL, FALSE, 'Active'),
    ('Thịt heo xay nạc', 'PROTEIN', 210.0, 19.0, 0.0, 14.5, '[]'::jsonb, 'PORK', NULL, FALSE, 'Active'),

    ('Cá hồi phi lê tươi', 'PROTEIN', 208.0, 20.0, 0.0, 13.0, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá ngừ đại dương tươi', 'PROTEIN', 130.0, 28.0, 0.0, 1.0, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá ngừ ngâm nước khoáng (Hộp)', 'PROTEIN', 116.0, 26.0, 0.0, 1.0, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá thu tươi', 'PROTEIN蛋白质', 205.0, 19.0, 0.0, 13.9, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá bass (Cá chẽm)', 'PROTEIN', 124.0, 23.0, 0.0, 3.0, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá lóc đồng', 'PROTEIN', 97.0, 18.2, 0.0, 2.7, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá chép tươi', 'PROTEIN', 127.0, 16.0, 0.0, 5.6, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá rô phi phi lê', 'PROTEIN', 96.0, 20.0, 0.0, 1.7, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),
    ('Cá tuyết (Cod)', 'PROTEIN', 82.0, 18.0, 0.0, 0.7, '["FISH"]'::jsonb, 'FISH', NULL, FALSE, 'Active'),

    ('Tôm sú tươi', 'PROTEIN', 99.0, 24.0, 0.2, 0.3, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),
    ('Mực ống tươi', 'PROTEIN', 92.0, 15.6, 3.1, 1.4, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),
    ('Mực lá hấp', 'PROTEIN', 105.0, 18.0, 2.0, 1.8, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),
    ('Bạch tuộc nướng', 'PROTEIN', 82.0, 14.9, 2.2, 1.0, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),
    ('Cua biển hấp', 'PROTEIN', 97.0, 19.0, 0.0, 1.5, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),
    ('Sò điệp tươi', 'PROTEIN', 88.0, 16.8, 2.4, 0.8, '["SEAFOOD"]'::jsonb, 'SEAFOOD', NULL, FALSE, 'Active'),

    ('Trứng gà tươi (Cả quả)', 'PROTEIN', 155.0, 13.0, 1.1, 11.0, '["EGG"]'::jsonb, 'EGG', NULL, FALSE, 'Active'),
    ('Lòng trắng trứng gà', 'PROTEIN', 52.0, 11.0, 0.7, 0.2, '["EGG"]'::jsonb, 'EGG', NULL, FALSE, 'Active'),
    ('Trứng vịt', 'PROTEIN', 185.0, 12.8, 1.4, 13.8, '["EGG"]'::jsonb, 'EGG', NULL, FALSE, 'Active'),
    ('Trứng cút', 'PROTEIN', 158.0, 13.0, 0.4, 11.0, '["EGG"]'::jsonb, 'EGG', NULL, FALSE, 'Active'),

    ('Đậu phụ mềm tươi', 'PROTEIN', 76.0, 8.0, 1.9, 4.8, '["SOY"]'::jsonb, 'SOY', NULL, FALSE, 'Active'),
    ('Đậu phụ chiên sơ', 'PROTEIN', 156.0, 14.0, 3.0, 9.5, '["SOY"]'::jsonb, 'SOY', NULL, FALSE, 'Active'),
    ('Đậu nành luộc (Edamame)', 'PROTEIN', 122.0, 11.0, 10.0, 5.0, '["SOY"]'::jsonb, 'SOY', NULL, FALSE, 'Active'),
    ('Tempeh đậu nành', 'PROTEIN', 193.0, 19.0, 9.0, 11.0, '["SOY"]'::jsonb, 'SOY', NULL, FALSE, 'Active'),
    ('Seitan (Mì căn đạm lúa mì)', 'PROTEIN', 370.0, 75.0, 14.0, 1.9, '["WHEAT"]'::jsonb, 'WHEAT', NULL, FALSE, 'Active'),

    ('Thịt cừu nạc', 'PROTEIN', 206.0, 20.0, 0.0, 13.5, '[]'::jsonb, 'LAMB', NULL, FALSE, 'Active'),
    ('Thịt vịt bỏ da', 'PROTEIN', 201.0, 23.5, 0.0, 11.2, '[]'::jsonb, 'DUCK', NULL, FALSE, 'Active'),
    ('Sữa chua Hy Lạp không đường', 'PROTEIN', 59.0, 10.0, 3.6, 0.4, '["DAIRY"]'::jsonb, 'DAIRY', NULL, FALSE, 'Active'),
    ('Phô mai Cottage Cheese', 'PROTEIN', 98.0, 11.0, 3.4, 4.3, '["DAIRY"]'::jsonb, 'DAIRY', NULL, FALSE, 'Active'),

    -- =========================================================================
    -- 2. CARBOHYDRATE SOURCES (Nguồn Tinh Bột)
    -- =========================================================================
    ('Cơm gạo lứt luộc', 'CARB', 110.0, 2.6, 23.0, 0.9, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Cơm trắng nấu chín', 'CARB', 130.0, 2.7, 28.0, 0.3, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Cơm gạo đỏ Huyết Rồng', 'CARB', 112.0, 2.5, 24.0, 0.8, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Khoai lang mật luộc', 'CARB', 90.0, 2.0, 21.0, 0.1, '[]'::jsonb, NULL, 'SWEET_POTATO', FALSE, 'Active'),
    ('Khoai lang tím luộc', 'CARB', 86.0, 1.6, 20.0, 0.1, '[]'::jsonb, NULL, 'SWEET_POTATO', FALSE, 'Active'),
    ('Khoai tây luộc nguyên vỏ', 'CARB', 87.0, 1.9, 20.0, 0.1, '[]'::jsonb, NULL, 'POTATO', FALSE, 'Active'),
    ('Khoai tây nướng thảo mộc', 'CARB', 93.0, 2.5, 21.0, 0.2, '[]'::jsonb, NULL, 'POTATO', FALSE, 'Active'),
    ('Khoai môn hấp', 'CARB', 112.0, 1.5, 26.0, 0.2, '[]'::jsonb, NULL, 'ROOT', FALSE, 'Active'),
    ('Khoai mì (Sắn) luộc', 'CARB', 160.0, 1.4, 38.0, 0.3, '[]'::jsonb, NULL, 'ROOT', FALSE, 'Active'),

    ('Yến mạch cán vỡ Quick Oats', 'CARB', 389.0, 16.9, 66.0, 6.9, '[]'::jsonb, NULL, 'OATS', FALSE, 'Active'),
    ('Yến mạch nguyên hạt Rolled Oats', 'CARB', 379.0, 13.0, 67.0, 6.5, '[]'::jsonb, NULL, 'OATS', FALSE, 'Active'),
    ('Hạt diêm mạch (Quinoa) nấu chín', 'CARB', 120.0, 4.4, 21.0, 1.9, '[]'::jsonb, NULL, 'QUINOA', FALSE, 'Active'),
    ('Hạt kiều mạch (Buckwheat)', 'CARB', 343.0, 13.0, 71.0, 3.4, '[]'::jsonb, NULL, 'GRAIN', FALSE, 'Active'),
    ('Bánh mì đen lúa mạch (Rye Bread)', 'CARB', 259.0, 8.5, 48.0, 3.3, '["WHEAT"]'::jsonb, NULL, 'WHEAT', FALSE, 'Active'),
    ('Bánh mì nguyên cám Whole Wheat', 'CARB', 247.0, 13.0, 41.0, 3.4, '["WHEAT"]'::jsonb, NULL, 'WHEAT', FALSE, 'Active'),
    ('Bánh mì sandwich trắng', 'CARB', 265.0, 9.0, 49.0, 3.2, '["WHEAT"]'::jsonb, NULL, 'WHEAT', FALSE, 'Active'),

    ('Bún tươi lứt', 'CARB', 130.0, 2.2, 28.0, 0.4, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Bún trắng tươi', 'CARB', 110.0, 1.7, 25.0, 0.2, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Phở tươi lứt', 'CARB', 135.0, 2.4, 29.0, 0.3, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Mì Ý nguyên cám (Whole Wheat Pasta)', 'CARB', 124.0, 5.3, 26.0, 0.5, '["WHEAT"]'::jsonb, NULL, 'WHEAT', FALSE, 'Active'),
    ('Mì Shirataki (Bún Konjac 0 calo)', 'CARB', 10.0, 0.2, 2.0, 0.0, '[]'::jsonb, NULL, 'KONJAC', FALSE, 'Active'),
    ('Mì chũ Bắc Giang', 'CARB', 350.0, 7.0, 78.0, 0.5, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),
    ('Miến dong riềng', 'CARB', 330.0, 0.7, 82.0, 0.2, '[]'::jsonb, NULL, 'ROOT', FALSE, 'Active'),
    ('Bánh phở khô', 'CARB', 348.0, 6.0, 79.0, 0.4, '[]'::jsonb, NULL, 'RICE', FALSE, 'Active'),

    ('Nổ bắp ngô luộc (Corn)', 'CARB', 96.0, 3.4, 21.0, 1.5, '[]'::jsonb, NULL, 'CORN', FALSE, 'Active'),
    ('Đậu đen luộc', 'CARB', 132.0, 8.9, 23.7, 0.5, '[]'::jsonb, NULL, 'BEANS', FALSE, 'Active'),
    ('Đậu đỏ luộc', 'CARB', 127.0, 8.7, 22.8, 0.5, '[]'::jsonb, NULL, 'BEANS', FALSE, 'Active'),
    ('Đậu xanh luộc', 'CARB', 105.0, 7.0, 19.0, 0.4, '[]'::jsonb, NULL, 'BEANS', FALSE, 'Active'),
    ('Đậu Lăng đỏ (Lentils)', 'CARB', 116.0, 9.0, 20.0, 0.4, '[]'::jsonb, NULL, 'LENTILS', FALSE, 'Active'),
    ('Đậu Gà luộc (Chickpeas)', 'CARB', 164.0, 8.9, 27.0, 2.6, '[]'::jsonb, NULL, 'BEANS', FALSE, 'Active'),

    ('Chuối tiêu chín', 'CARB', 89.0, 1.1, 23.0, 0.3, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Táo đỏ Mỹ', 'CARB', 52.0, 0.3, 14.0, 0.2, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Cam sành Việt Nam', 'CARB', 47.0, 0.9, 12.0, 0.1, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Thơm (Dứa) chín', 'CARB', 50.0, 0.5, 13.0, 0.1, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Xoài chín', 'CARB', 60.0, 0.8, 15.0, 0.4, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Dưa hấu tươi', 'CARB', 30.0, 0.6, 7.5, 0.2, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Dâu tây tươi', 'CARB', 32.0, 0.7, 7.7, 0.3, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Việt quất tươi (Blueberry)', 'CARB', 57.0, 0.7, 14.0, 0.3, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Bưởi da xanh', 'CARB', 38.0, 0.8, 9.5, 0.1, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),
    ('Kiwi xanh', 'CARB', 61.0, 1.1, 15.0, 0.5, '[]'::jsonb, NULL, 'FRUIT', FALSE, 'Active'),

    -- =========================================================================
    -- 3. VEGETABLE SOURCES (Rau Xanh & Nấm)
    -- =========================================================================
    ('Bông cải xanh (Broccoli)', 'VEGGIE', 34.0, 2.8, 7.0, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bông cải trắng (Cauliflower)', 'VEGGIE', 25.0, 1.9, 5.0, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Măng tây tươi', 'VEGGIE', 20.0, 2.2, 3.8, 0.1, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Rau bina (Cải bó xôi)', 'VEGGIE', 23.0, 2.9, 3.6, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cải kale (Xoăn)', 'VEGGIE', 49.0, 4.3, 8.8, 0.9, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cải thìa tươi', 'VEGGIE', 13.0, 1.5, 2.2, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cải ngọt luộc', 'VEGGIE', 16.0, 1.7, 2.8, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cải thảo tươi', 'VEGGIE', 16.0, 1.2, 3.2, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Rau muống luộc', 'VEGGIE', 19.0, 3.2, 2.1, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Rau ngót luộc', 'VEGGIE', 35.0, 5.3, 3.4, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Rau mồng tơi', 'VEGGIE', 14.0, 2.0, 2.4, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Rau dền đỏ', 'VEGGIE', 23.0, 2.5, 4.0, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cần tây tươi', 'VEGGIE', 16.0, 0.7, 3.0, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Xà lách Lô-măng (Romaine)', 'VEGGIE', 17.0, 1.2, 3.3, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Xà lách mỡ tươi', 'VEGGIE', 14.0, 1.3, 2.2, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),

    ('Cà chua ripe tươi', 'VEGGIE', 18.0, 0.9, 3.9, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Dưa leo (Dưa chuột)', 'VEGGIE', 15.0, 0.7, 3.6, 0.1, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cà rốt tươi', 'VEGGIE', 41.0, 0.9, 9.6, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Ớt chuông đỏ', 'VEGGIE', 31.0, 1.0, 6.0, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Ớt chuông vàng', 'VEGGIE', 27.0, 1.0, 6.3, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Ớt chuông xanh', 'VEGGIE', 20.0, 0.9, 4.6, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bí ngòi xanh (Zucchini)', 'VEGGIE', 17.0, 1.2, 3.1, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bí đỏ (Bí rợ) hấp', 'VEGGIE', 26.0, 1.0, 6.5, 0.1, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bắp cải tím', 'VEGGIE', 31.0, 1.4, 7.4, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bắp cải trắng', 'VEGGIE', 25.0, 1.3, 5.8, 0.1, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Cà tím nướng', 'VEGGIE', 25.0, 1.0, 6.0, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Đậu đũa luộc', 'VEGGIE', 47.0, 2.8, 8.0, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Đậu cô ve luộc', 'VEGGIE', 31.0, 1.8, 7.0, 0.1, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),

    ('Nấm đùi gà tươi', 'VEGGIE', 35.0, 3.1, 6.0, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Nấm kim châm', 'VEGGIE', 37.0, 2.7, 7.8, 0.3, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Nấm rơm tươi', 'VEGGIE', 32.0, 3.8, 5.0, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Nấm đông cô (Hương) tươi', 'VEGGIE', 34.0, 2.2, 6.8, 0.5, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Nấm bào ngư xám', 'VEGGIE', 33.0, 3.3, 6.1, 0.4, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Mộc nhĩ (Nấm mèo) ngâm nở', 'VEGGIE', 25.0, 1.0, 6.5, 0.2, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),

    -- =========================================================================
    -- 4. HEALTHY FAT SOURCES (Nguồn Chất Béo Tốt & Hạt)
    -- =========================================================================
    ('Dầu Oliu nguyên chất Extra Virgin', 'FAT', 884.0, 0.0, 0.0, 100.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Dầu bơ (Avocado Oil)', 'FAT', 884.0, 0.0, 0.0, 100.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Dầu dừa tinh khiết', 'FAT', 862.0, 0.0, 0.0, 100.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Dầu mè (Sesame Oil)', 'FAT', 884.0, 0.0, 0.0, 100.0, '["SESAME"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Quả bơ sáp tươi', 'FAT', 160.0, 2.0, 8.5, 14.7, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),

    ('Hạt hạnh nhân (Almond)', 'FAT', 579.0, 21.0, 22.0, 49.9, '["NUTS"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt óc chó (Walnut)', 'FAT', 654.0, 15.0, 13.7, 65.2, '["NUTS"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt điều rang muối', 'FAT', 553.0, 18.0, 30.0, 44.0, '["NUTS"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt Macca Úc', 'FAT', 718.0, 7.9, 13.8, 75.8, '["NUTS"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt chia tươi (Chia Seeds)', 'FAT', 486.0, 16.5, 42.0, 30.7, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt lanh (Flaxseeds)', 'FAT', 534.0, 18.0, 29.0, 42.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt hướng dương tách vỏ', 'FAT', 584.0, 20.0, 20.0, 51.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Hạt bí tách vỏ', 'FAT', 559.0, 30.0, 10.7, 49.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bơ đậu phộng nguyên chất (Mịn)', 'FAT', 588.0, 25.0, 20.0, 50.0, '["PEANUT"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Bơ hạnh nhân (Almond Butter)', 'FAT', 614.0, 21.0, 19.0, 56.0, '["NUTS"]'::jsonb, NULL, NULL, FALSE, 'Active'),
    ('Socola đen 85% Cacao', 'FAT', 600.0, 7.8, 36.0, 46.0, '[]'::jsonb, NULL, NULL, FALSE, 'Active'),

    -- =========================================================================
    -- 5. NUTIFOOD PARTNER PRODUCTS (Sản Phẩm Dinh Dưỡng Cao NutiFood)
    -- =========================================================================
    ('Sữa NutiFood Varna Complete 250ml', 'NUTIFOOD', 80.0, 4.0, 10.0, 2.5, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood Varna Elite Hoàng Gia 237ml', 'NUTIFOOD', 95.0, 5.0, 11.5, 3.0, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood Varna Diabetes 237ml', 'NUTIFOOD', 75.0, 4.2, 8.5, 2.6, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood LeanMax High Protein Shake', 'NUTIFOOD', 100.0, 8.0, 12.0, 2.0, '["WHEY"]'::jsonb, 'WHEY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood Enplus Gold Dinh Dưỡng Cao', 'NUTIFOOD', 100.0, 4.5, 13.0, 3.2, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood GrowPLUS+ Đỏ 110ml', 'NUTIFOOD', 85.0, 3.5, 11.0, 2.8, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood GrowPLUS+ Trắng giảm cân', 'NUTIFOOD', 65.0, 4.5, 8.0, 1.5, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa NutiFood CaloPic Phục Hồi Thể Lực', 'NUTIFOOD', 110.0, 6.0, 14.0, 3.5, '["WHEY"]'::jsonb, 'WHEY', NULL, TRUE, 'Active'),
    ('Sữa Tiệt Trùng NutiFood 100% Sữa Tươi 220ml', 'NUTIFOOD', 62.0, 3.0, 4.8, 3.3, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa Đậu Nành NutiFood Nguyên Chất', 'NUTIFOOD', 45.0, 3.2, 4.0, 1.8, '["SOY"]'::jsonb, 'SOY', NULL, TRUE, 'Active'),
    ('Sữa Hạt Óc Chó NutiFood Varna 200ml', 'NUTIFOOD', 58.0, 2.0, 7.5, 2.2, '["NUTS"]'::jsonb, 'NUTS', NULL, TRUE, 'Active'),
    ('Sữa Hạt Hạnh Nhân NutiFood 200ml', 'NUTIFOOD', 52.0, 1.8, 6.8, 2.0, '["NUTS"]'::jsonb, 'NUTS', NULL, TRUE, 'Active'),

    ('Bánh Dinh Dưỡng NutiFood FitBar Protein Granola', 'NUTIFOOD', 350.0, 15.0, 45.0, 10.0, '["SOY", "OATS"]'::jsonb, 'SOY', 'OATS', TRUE, 'Active'),
    ('Bánh Ngũ Cốc Dinh Dưỡng NutiFood NuVi', 'NUTIFOOD', 380.0, 8.0, 65.0, 9.0, '["WHEAT"]'::jsonb, NULL, 'WHEAT', TRUE, 'Active'),
    ('Bột Ngũ Cốc Dinh Dưỡng NutiFood 20T', 'NUTIFOOD', 400.0, 10.0, 70.0, 8.0, '["SOY"]'::jsonb, 'SOY', 'RICE', TRUE, 'Active'),
    ('Sữa Chua Uống NutiFood NuVi Trái Cây 100ml', 'NUTIFOOD', 60.0, 1.2, 12.5, 0.6, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Sữa Chua Ăn NutiFood Nha Đam 100g', 'NUTIFOOD', 90.0, 3.0, 15.0, 2.0, '["DAIRY"]'::jsonb, 'DAIRY', NULL, TRUE, 'Active'),
    ('Thạch Dinh Dưỡng NutiFood NuVi Jelly Pro', 'NUTIFOOD', 50.0, 0.5, 12.0, 0.1, '[]'::jsonb, NULL, NULL, TRUE, 'Active')
ON CONFLICT DO NOTHING;
