-- ============================================================
-- GN-WAAS Seed Data: 005_sample_accounts
-- Description: Realistic sample water accounts for Tema East
--              pilot district covering all fraud scenarios
-- ============================================================

-- Helper: Get Tema East district ID
DO $$
DECLARE
    tema_east_id UUID;
    accra_west_id UUID;
BEGIN
    SELECT id INTO tema_east_id FROM districts WHERE district_code = 'TEMA-EAST';
    SELECT id INTO accra_west_id FROM districts WHERE district_code = 'ACCRA-WEST';

    -- ============================================================
    -- NORMAL ACCOUNTS (Baseline - no fraud)
    -- ============================================================
    INSERT INTO water_accounts (gwl_account_number, account_holder_name, account_holder_tin, category, status, district_id, meter_number, meter_serial, meter_install_date, address_line1, gps_latitude, gps_longitude, is_within_network, monthly_avg_consumption) VALUES
    ('GWL-TE-001001', 'Kwame Asante Household',      'C0012345678', 'RESIDENTIAL',   'ACTIVE', tema_east_id, 'MTR-001001', 'SN-A1001', '2020-03-15', '12 Harbour Road, Tema', 5.6698, -0.0166, TRUE, 8.5),
    ('GWL-TE-001002', 'Ama Boateng Family',           'C0012345679', 'RESIDENTIAL',   'ACTIVE', tema_east_id, 'MTR-001002', 'SN-A1002', '2019-07-22', '45 Community 1, Tema', 5.6712, -0.0178, TRUE, 12.3),
    ('GWL-TE-001003', 'Kofi Mensah Residence',        'C0012345680', 'RESIDENTIAL',   'ACTIVE', tema_east_id, 'MTR-001003', 'SN-A1003', '2021-01-10', '8 Fishing Harbour, Tema', 5.6685, -0.0155, TRUE, 6.2),
    ('GWL-TE-002001', 'Tema Port Authority',           'C0098765432', 'PUBLIC_GOVT',   'ACTIVE', tema_east_id, 'MTR-002001', 'SN-B2001', '2018-05-20', 'Port Access Road, Tema', 5.6750, -0.0100, TRUE, 485.0),
    ('GWL-TE-002002', 'Tema Metropolitan Assembly',   'C0098765433', 'PUBLIC_GOVT',   'ACTIVE', tema_east_id, 'MTR-002002', 'SN-B2002', '2017-11-15', 'TMA Offices, Tema', 5.6720, -0.0145, TRUE, 320.0),

    -- ============================================================
    -- FRAUD SCENARIO 1: Category Mismatch (Commercial billed as Residential)
    -- ============================================================
    ('GWL-TE-003001', 'Tema Cold Store Ltd',          'C0055512345', 'RESIDENTIAL',   'FLAGGED', tema_east_id, 'MTR-003001', 'SN-C3001', '2019-03-10', '22 Industrial Area, Tema', 5.6800, -0.0120, TRUE, 285.0),
    ('GWL-TE-003002', 'Harbour View Hotel',            'C0055512346', 'RESIDENTIAL',   'FLAGGED', tema_east_id, 'MTR-003002', 'SN-C3002', '2020-08-15', '5 Beach Road, Tema', 5.6690, -0.0130, TRUE, 420.0),
    ('GWL-TE-003003', 'Tema Sachet Water Factory',    'C0055512347', 'COMMERCIAL',    'FLAGGED', tema_east_id, 'MTR-003003', 'SN-C3003', '2018-12-01', '15 Factory Lane, Tema', 5.6820, -0.0110, TRUE, 1850.0),

    -- ============================================================
    -- FRAUD SCENARIO 2: Phantom Meters (Identical readings for 8+ months)
    -- ============================================================
    ('GWL-TE-004001', 'Phantom Account Alpha',        NULL,          'RESIDENTIAL',   'GHOST',   tema_east_id, 'MTR-004001', 'SN-D4001', '2015-06-01', '99 Unknown Street, Tema', 5.6700, -0.0160, TRUE, 5.0),
    ('GWL-TE-004002', 'Phantom Account Beta',         NULL,          'RESIDENTIAL',   'GHOST',   tema_east_id, 'MTR-004002', 'SN-D4002', '2016-03-15', '101 Unknown Street, Tema', 5.6701, -0.0161, TRUE, 5.0),
    ('GWL-TE-004003', 'Phantom Account Gamma',        NULL,          'RESIDENTIAL',   'GHOST',   tema_east_id, 'MTR-004003', 'SN-D4003', '2014-09-20', '103 Unknown Street, Tema', 5.6702, -0.0162, TRUE, 5.0),

    -- ============================================================
    -- FRAUD SCENARIO 3: Ghost Accounts (Outside network boundary)
    -- ============================================================
    ('GWL-TE-005001', 'Outside Network Account 1',   NULL,          'RESIDENTIAL',   'GHOST',   tema_east_id, 'MTR-005001', 'SN-E5001', '2020-01-01', 'Remote Location 1', 5.7200, -0.0500, FALSE, 8.0),
    ('GWL-TE-005002', 'Outside Network Account 2',   NULL,          'RESIDENTIAL',   'GHOST',   tema_east_id, 'MTR-005002', 'SN-E5002', '2021-06-15', 'Remote Location 2', 5.7350, -0.0620, FALSE, 6.5),

    -- ============================================================
    -- FRAUD SCENARIO 4: Bottled Water Producer (Underpaying)
    -- ============================================================
    ('GWL-TE-006001', 'AquaPure Ghana Ltd',           'C0077788899', 'COMMERCIAL',    'FLAGGED', tema_east_id, 'MTR-006001', 'SN-F6001', '2019-04-10', '8 Industrial Zone, Tema', 5.6810, -0.0115, TRUE, 3200.0),
    ('GWL-TE-006002', 'Crystal Waters Factory',       'C0077788900', 'RESIDENTIAL',   'FLAGGED', tema_east_id, 'MTR-006002', 'SN-F6002', '2020-02-20', '12 Factory Road, Tema', 5.6815, -0.0118, TRUE, 2800.0),

    -- ============================================================
    -- NORMAL ACCOUNTS - Accra West
    -- ============================================================
    ('GWL-AW-001001', 'Abena Osei Household',         'C0033344455', 'RESIDENTIAL',   'ACTIVE', accra_west_id, 'MTR-AW1001', 'SN-AW1001', '2021-05-10', '34 Kaneshie Road, Accra', 5.5502, -0.2340, TRUE, 9.8),
    ('GWL-AW-001002', 'Yaw Darko Family',              'C0033344456', 'RESIDENTIAL',   'ACTIVE', accra_west_id, 'MTR-AW1002', 'SN-AW1002', '2020-11-25', '67 Dansoman Estate, Accra', 5.5480, -0.2380, TRUE, 14.2),
    ('GWL-AW-002001', 'Accra West Hospital',           'C0099900011', 'PUBLIC_GOVT',   'ACTIVE', accra_west_id, 'MTR-AW2001', 'SN-AW2001', '2016-08-15', 'Hospital Road, Accra West', 5.5520, -0.2300, TRUE, 680.0),
    ('GWL-AW-003001', 'Kaneshie Market Complex',       'C0066677788', 'COMMERCIAL',    'ACTIVE', accra_west_id, 'MTR-AW3001', 'SN-AW3001', '2018-03-20', 'Kaneshie Market, Accra', 5.5510, -0.2320, TRUE, 520.0);

    -- Flag phantom accounts
    UPDATE water_accounts
    SET is_phantom_flagged = TRUE,
        phantom_flag_reason = 'Identical meter readings for 8+ consecutive months',
        phantom_flag_date = NOW()
    WHERE gwl_account_number IN ('GWL-TE-004001', 'GWL-TE-004002', 'GWL-TE-004003');

    -- Flag outside-network accounts
    UPDATE water_accounts
    SET is_phantom_flagged = TRUE,
        phantom_flag_reason = 'Account GPS coordinates outside GWL pipe network boundary',
        phantom_flag_date = NOW()
    WHERE gwl_account_number IN ('GWL-TE-005001', 'GWL-TE-005002');

END $$;
