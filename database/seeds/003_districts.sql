-- ============================================================
-- GN-WAAS Seed Data: 003_districts
-- Description: Ghana districts with pilot focus on Tema East
--              and Accra West (high-loss, high-revenue targets)
-- GPS coordinates are real Ghana district centroids for DMA map rendering.
-- ============================================================

INSERT INTO districts (district_code, district_name, region, population_estimate, total_connections,
                        supply_status, zone_type, is_pilot_district, is_active,
                        gps_latitude, gps_longitude) VALUES

-- ============================================================
-- GREATER ACCRA REGION (Pilot Districts)
-- ============================================================
('TEMA-EAST', 'Tema East', 'Greater Accra', 285000, 42000, 'NORMAL', 'RED', TRUE, TRUE, 5.6698, -0.0166),
('TEMA-WEST', 'Tema West', 'Greater Accra', 198000, 31000, 'NORMAL', 'RED', FALSE, TRUE, 5.65, -0.03),
('ACCRA-WEST', 'Accra West', 'Greater Accra', 412000, 68000, 'NORMAL', 'RED', TRUE, TRUE, 5.55, -0.25),
('ACCRA-CENTRAL', 'Accra Central', 'Greater Accra', 380000, 62000, 'NORMAL', 'YELLOW', FALSE, TRUE, 5.556, -0.1969),
('ACCRA-EAST', 'Accra East', 'Greater Accra', 295000, 48000, 'NORMAL', 'YELLOW', FALSE, TRUE, 5.58, -0.15),
('ASHAIMAN', 'Ashaiman', 'Greater Accra', 225000, 35000, 'REDUCED', 'RED', FALSE, TRUE, 5.696, -0.033),
('LEDZOKUKU', 'Ledzokuku-Krowor', 'Greater Accra', 175000, 28000, 'NORMAL', 'YELLOW', FALSE, TRUE, 5.62, -0.12),
('ADENTAN', 'Adentan', 'Greater Accra', 145000, 22000, 'NORMAL', 'GREEN', FALSE, TRUE, 5.68, -0.16),
('AYAWASO', 'Ayawaso', 'Greater Accra', 320000, 52000, 'NORMAL', 'YELLOW', FALSE, TRUE, 5.61, -0.18),
('ABLEKUMA', 'Ablekuma', 'Greater Accra', 265000, 43000, 'NORMAL', 'RED', FALSE, TRUE, 5.57, -0.22),

-- ============================================================
-- ASHANTI REGION
-- ============================================================
('KUMASI-CENTRAL', 'Kumasi Central', 'Ashanti', 520000, 85000, 'NORMAL', 'RED', FALSE, TRUE, 6.6885, -1.6244),
('KUMASI-NORTH', 'Kumasi North', 'Ashanti', 285000, 46000, 'NORMAL', 'YELLOW', FALSE, TRUE, 6.72, -1.61),
('KUMASI-SOUTH', 'Kumasi South', 'Ashanti', 198000, 32000, 'NORMAL', 'YELLOW', FALSE, TRUE, 6.66, -1.63),
('OFORIKROM', 'Oforikrom', 'Ashanti', 165000, 27000, 'NORMAL', 'GREEN', FALSE, TRUE, 6.67, -1.59),
('ASOKWA', 'Asokwa', 'Ashanti', 142000, 23000, 'NORMAL', 'GREEN', FALSE, TRUE, 6.68, -1.65),

-- ============================================================
-- WESTERN REGION
-- ============================================================
('TAKORADI', 'Takoradi', 'Western', 310000, 51000, 'NORMAL', 'YELLOW', FALSE, TRUE, 4.8845, -1.7554),
('SEKONDI', 'Sekondi', 'Western', 185000, 30000, 'NORMAL', 'YELLOW', FALSE, TRUE, 4.94, -1.71),

-- ============================================================
-- CENTRAL REGION
-- ============================================================
('CAPE-COAST', 'Cape Coast', 'Central', 270000, 44000, 'NORMAL', 'YELLOW', FALSE, TRUE, 5.1053, -1.2466),

-- ============================================================
-- NORTHERN REGION
-- ============================================================
('TAMALE-CENTRAL', 'Tamale Central', 'Northern', 420000, 68000, 'NORMAL', 'RED', FALSE, TRUE, 9.4008, -0.8393),
('TAMALE-SOUTH', 'Tamale South', 'Northern', 195000, 32000, 'NORMAL', 'RED', FALSE, TRUE, 9.38, -0.82),

-- ============================================================
-- BONO REGION
-- ============================================================
('SUNYANI', 'Sunyani', 'Bono', 165000, 27000, 'NORMAL', 'YELLOW', FALSE, TRUE, 7.3349, -2.3123),

-- ============================================================
-- VOLTA REGION
-- ============================================================
('HO', 'Ho', 'Volta', 185000, 30000, 'NORMAL', 'GREEN', FALSE, TRUE, 6.6011, 0.4712),

-- ============================================================
-- EASTERN REGION
-- ============================================================
('KOFORIDUA', 'Koforidua', 'Eastern', 210000, 34000, 'NORMAL', 'YELLOW', FALSE, TRUE, 6.094, -0.259),

-- ============================================================
-- UPPER WEST REGION
-- ============================================================
('WA', 'Wa', 'Upper West', 120000, 19000, 'REDUCED', 'RED', FALSE, TRUE, 10.0601, -2.5099),

-- ============================================================
-- UPPER EAST REGION
-- ============================================================
('BOLGATANGA', 'Bolgatanga', 'Upper East', 135000, 22000, 'REDUCED', 'RED', FALSE, TRUE, 10.7856, -0.8514);
