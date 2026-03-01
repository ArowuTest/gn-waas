-- ============================================================
-- GN-WAAS Seed Data: 002_tariff_rates
-- Description: PURC 2026 approved tariff rates
--              Source: PURC Ghana Tariff Review January 2026
-- ============================================================

-- VAT Configuration (20% effective rate as of 2026)
-- Breakdown: VAT 12.5% + NHIL 2.5% + GETFund 2.5% + COVID Levy 1.0% + ECOWAS Levy 0.5% = ~19% + rounding = 20%
INSERT INTO vat_config (rate_percentage, components, effective_from, regulatory_ref, is_active) VALUES (
    20.00,
    '{
        "VAT": 12.5,
        "NHIL": 2.5,
        "GETFund": 2.5,
        "COVID_Levy": 1.0,
        "ECOWAS_Levy": 0.5,
        "effective_rate": 20.0,
        "note": "Effective composite rate per GRA Act 1151 (2026)"
    }',
    '2026-01-01',
    'GRA-ACT-1151-2026',
    TRUE
);

-- ============================================================
-- RESIDENTIAL TARIFFS
-- ============================================================
INSERT INTO tariff_rates (category, tier_name, min_volume_m3, max_volume_m3, rate_per_m3, service_charge_ghs, effective_from, regulatory_ref, is_active) VALUES
('RESIDENTIAL', 'Tier 1 - Lifeline',    0,    5,    6.1225,  0.00, '2026-01-01', 'PURC-2026-TARIFF-01', TRUE),
('RESIDENTIAL', 'Tier 2 - Standard',    5,    NULL, 10.8320, 0.00, '2026-01-01', 'PURC-2026-TARIFF-01', TRUE);

-- ============================================================
-- PUBLIC / GOVERNMENT TARIFFS
-- ============================================================
INSERT INTO tariff_rates (category, tier_name, min_volume_m3, max_volume_m3, rate_per_m3, service_charge_ghs, effective_from, regulatory_ref, is_active) VALUES
('PUBLIC_GOVT', 'Flat Rate',            0,    NULL, 15.7372, 2000.00, '2026-01-01', 'PURC-2026-TARIFF-02', TRUE);

-- ============================================================
-- COMMERCIAL TARIFFS
-- ============================================================
INSERT INTO tariff_rates (category, tier_name, min_volume_m3, max_volume_m3, rate_per_m3, service_charge_ghs, effective_from, regulatory_ref, is_active) VALUES
('COMMERCIAL', 'Standard Commercial',   0,    NULL, 18.4500, 500.00,  '2026-01-01', 'PURC-2026-TARIFF-03', TRUE);

-- ============================================================
-- INDUSTRIAL TARIFFS
-- ============================================================
INSERT INTO tariff_rates (category, tier_name, min_volume_m3, max_volume_m3, rate_per_m3, service_charge_ghs, effective_from, regulatory_ref, is_active) VALUES
('INDUSTRIAL', 'Industrial Rate',       0,    NULL, 22.1000, 1500.00, '2026-01-01', 'PURC-2026-TARIFF-04', TRUE);

-- ============================================================
-- BOTTLED WATER PRODUCER TARIFFS
-- ============================================================
INSERT INTO tariff_rates (category, tier_name, min_volume_m3, max_volume_m3, rate_per_m3, service_charge_ghs, effective_from, regulatory_ref, is_active) VALUES
('BOTTLED_WATER', 'Bottled Water Rate', 0,    NULL, 32.7858, 25000.00, '2026-01-01', 'PURC-2026-TARIFF-05', TRUE);
