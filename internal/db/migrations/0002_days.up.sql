CREATE TABLE IF NOT EXISTS days (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    day_no      INTEGER NOT NULL UNIQUE CHECK (day_no BETWEEN 1 AND 21),
    title       TEXT    NOT NULL,
    bullets     TEXT    NOT NULL DEFAULT '[]', -- JSON array
    description TEXT    NOT NULL DEFAULT '',
    video_url   TEXT    NOT NULL DEFAULT '',
    published   INTEGER NOT NULL DEFAULT 1,    -- 0/1
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_days_day_no ON days(day_no);

-- Seed: ilk üç eğitim
INSERT OR IGNORE INTO days (day_no, title, bullets, description) VALUES
(1, 'Programa Giriş',
 '["Neden 21 Eğitim?","Hedef Belirleme","Marka Yolculuğu"]',
 '21 eğitimlik yolculuğun ilk adımı: programın ruhunu, hedeflerini ve sana katacaklarını konuşuyoruz.'),
(2, 'Markanın Doğuşu',
 '["Marka Nedir?","Tarihsel Süreç","Modern Marka"]',
 'Markanın tarihsel kökenleri ve bugünkü anlamı üzerine sağlam bir temel atıyoruz.'),
(3, 'Bizden Neden Marka Çıkmıyor?',
 '["Markanın Gerçek Tanımı","Marka Fikrine Giriş","3C Formülü"]',
 '100 puanlık uzmanlık sorusu: Bu topraklardan neden dünya markası çıkmıyor? Üçüncü eğitimde markanın gerçek tanımına odaklanıp, marka fikrine giriş yapıyoruz. 3C formülünü açıyoruz.');
