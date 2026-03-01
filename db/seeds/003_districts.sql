-- ============================================================
-- GN-WAAS Seed Data: 003_districts
-- Description: Ghana districts with pilot focus on Tema East
--              and Accra West (high-loss, high-revenue targets)
-- ============================================================

INSERT INTO districts (district_code, district_name, region, population_estimate, total_connections, supply_status, zone_type, is_pilot_district, is_active) VALUES

-- ============================================================
-- GREATER ACCRA REGION (Pilot Districts)
-- ============================================================
('TEMA-EAST',    'Tema East',           'Greater Accra', 285000, 42000, 'NORMAL',  'RED',    TRUE,  TRUE),
('TEMA-WEST',    'Tema West',           'Greater Accra', 198000, 31000, 'NORMAL',  'RED',    FALSE, TRUE),
('ACCRA-WEST',   'Accra West',          'Greater Accra', 412000, 68000, 'NORMAL',  'RED',    TRUE,  TRUE),
('ACCRA-CENTRAL','Accra Central',       'Greater Accra', 380000, 62000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('ACCRA-EAST',   'Accra East',          'Greater Accra', 295000, 48000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('ASHAIMAN',     'Ashaiman',            'Greater Accra', 225000, 35000, 'REDUCED', 'RED',    FALSE, TRUE),
('LEDZOKUKU',    'Ledzokuku-Krowor',    'Greater Accra', 175000, 28000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('ADENTAN',      'Adentan',             'Greater Accra', 145000, 22000, 'NORMAL',  'GREEN',  FALSE, TRUE),
('AYAWASO',      'Ayawaso',             'Greater Accra', 320000, 52000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('ABLEKUMA',     'Ablekuma',            'Greater Accra', 265000, 43000, 'NORMAL',  'RED',    FALSE, TRUE),

-- ============================================================
-- ASHANTI REGION
-- ============================================================
('KUMASI-CENTRAL','Kumasi Central',     'Ashanti',       520000, 85000, 'NORMAL',  'RED',    FALSE, TRUE),
('KUMASI-NORTH', 'Kumasi North',        'Ashanti',       285000, 46000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('KUMASI-SOUTH', 'Kumasi South',        'Ashanti',       198000, 32000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('OFORIKROM',    'Oforikrom',           'Ashanti',       165000, 27000, 'NORMAL',  'GREEN',  FALSE, TRUE),
('ASOKWA',       'Asokwa',              'Ashanti',       142000, 23000, 'NORMAL',  'GREEN',  FALSE, TRUE),

-- ============================================================
-- WESTERN REGION
-- ============================================================
('SEKONDI-TAKORADI','Sekondi-Takoradi', 'Western',       445000, 72000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('EFFIA-KWESIMINTSIM','Effia-Kwesimintsim','Western',    185000, 30000, 'NORMAL',  'GREY',   FALSE, TRUE),

-- ============================================================
-- CENTRAL REGION
-- ============================================================
('CAPE-COAST',   'Cape Coast',          'Central',       280000, 45000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('ELMINA',       'Elmina',              'Central',        95000, 15000, 'NORMAL',  'GREY',   FALSE, TRUE),

-- ============================================================
-- EASTERN REGION
-- ============================================================
('KOFORIDUA',    'Koforidua',           'Eastern',       185000, 30000, 'NORMAL',  'YELLOW', FALSE, TRUE),
('NSAWAM',       'Nsawam',              'Eastern',        95000, 15000, 'REDUCED', 'RED',    FALSE, TRUE),

-- ============================================================
-- VOLTA REGION
-- ============================================================
('HO',           'Ho',                  'Volta',         185000, 28000, 'NORMAL',  'GREY',   FALSE, TRUE),
('KETA',         'Keta',                'Volta',          85000, 12000, 'REDUCED', 'GREY',   FALSE, TRUE),

-- ============================================================
-- NORTHERN REGION
-- ============================================================
('TAMALE',       'Tamale',              'Northern',      420000, 58000, 'REDUCED', 'RED',    FALSE, TRUE),
('SAGNARIGU',    'Sagnarigu',           'Northern',      185000, 25000, 'REDUCED', 'GREY',   FALSE, TRUE);

-- Update pilot districts with realistic loss ratios
UPDATE districts SET loss_ratio_pct = 58.3, data_confidence_grade = 4 WHERE district_code = 'TEMA-EAST';
UPDATE districts SET loss_ratio_pct = 62.1, data_confidence_grade = 3 WHERE district_code = 'ACCRA-WEST';
UPDATE districts SET loss_ratio_pct = 71.4, data_confidence_grade = 2 WHERE district_code = 'ASHAIMAN';
UPDATE districts SET loss_ratio_pct = 45.2, data_confidence_grade = 5 WHERE district_code = 'ACCRA-CENTRAL';
UPDATE districts SET loss_ratio_pct = 38.7, data_confidence_grade = 6 WHERE district_code = 'ADENTAN';
UPDATE districts SET loss_ratio_pct = 55.9, data_confidence_grade = 4 WHERE district_code = 'KUMASI-CENTRAL';
