-- Seed sample water accounts for testing
-- Using Ablekuma district (our test district)
DO $$
DECLARE
    ablekuma_id UUID;
    accra_central_id UUID;
BEGIN
    SELECT id INTO ablekuma_id FROM districts WHERE district_code = 'ABLEKUMA';
    SELECT id INTO accra_central_id FROM districts WHERE district_code = 'ACCRA-CENTRAL';

    IF ablekuma_id IS NULL THEN
        RAISE NOTICE 'Ablekuma district not found, skipping seed';
        RETURN;
    END IF;

    -- Normal residential accounts
    INSERT INTO water_accounts (gwl_account_number, account_holder_name, category, status, district_id, meter_number, meter_serial, address_line1, gps_latitude, gps_longitude, is_within_network, monthly_avg_consumption)
    VALUES
    ('GWL-AB-001001', 'Kwame Asante Household', 'RESIDENTIAL', 'ACTIVE', ablekuma_id, 'MTR-AB1001', 'SN-AB1001', '12 Ablekuma Road, Accra', 5.6200, -0.2500, TRUE, 8.5),
    ('GWL-AB-001002', 'Ama Boateng Family', 'RESIDENTIAL', 'ACTIVE', ablekuma_id, 'MTR-AB1002', 'SN-AB1002', '45 Community 1, Ablekuma', 5.6215, -0.2515, TRUE, 12.3),
    ('GWL-AB-001003', 'Kofi Mensah Residence', 'RESIDENTIAL', 'ACTIVE', ablekuma_id, 'MTR-AB1003', 'SN-AB1003', '8 Fishing Harbour, Ablekuma', 5.6185, -0.2490, TRUE, 6.2),
    -- Commercial accounts
    ('GWL-AB-002001', 'Ablekuma Cold Store Ltd', 'COMMERCIAL', 'FLAGGED', ablekuma_id, 'MTR-AB2001', 'SN-AB2001', '22 Industrial Area, Ablekuma', 5.6300, -0.2450, TRUE, 285.0),
    ('GWL-AB-002002', 'Harbour View Hotel', 'COMMERCIAL', 'ACTIVE', ablekuma_id, 'MTR-AB2002', 'SN-AB2002', '5 Beach Road, Ablekuma', 5.6190, -0.2480, TRUE, 420.0),
    -- Phantom/Ghost accounts
    ('GWL-AB-003001', 'Phantom Account Alpha', 'RESIDENTIAL', 'GHOST', ablekuma_id, 'MTR-AB3001', 'SN-AB3001', '99 Unknown Street, Ablekuma', 5.6200, -0.2510, TRUE, 5.0),
    ('GWL-AB-003002', 'Phantom Account Beta', 'RESIDENTIAL', 'GHOST', ablekuma_id, 'MTR-AB3002', 'SN-AB3002', '101 Unknown Street, Ablekuma', 5.6201, -0.2511, TRUE, 5.0),
    -- Industrial
    ('GWL-AB-004001', 'AquaPure Ghana Ltd', 'INDUSTRIAL', 'FLAGGED', ablekuma_id, 'MTR-AB4001', 'SN-AB4001', '8 Industrial Zone, Ablekuma', 5.6310, -0.2440, TRUE, 3200.0),
    -- Government
    ('GWL-AB-005001', 'Ablekuma District Assembly', 'PUBLIC_GOVT', 'ACTIVE', ablekuma_id, 'MTR-AB5001', 'SN-AB5001', 'District Assembly, Ablekuma', 5.6220, -0.2520, TRUE, 320.0)
    ON CONFLICT (gwl_account_number) DO NOTHING;

    RAISE NOTICE 'Seeded water accounts for Ablekuma district';
END $$;
