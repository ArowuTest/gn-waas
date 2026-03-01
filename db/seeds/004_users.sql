-- ============================================================
-- GN-WAAS Seed Data: 004_users
-- Description: Default system users for all RBAC roles
--              Passwords managed by Keycloak - these are profile records only
-- ============================================================

-- NOTE: Passwords are managed by Keycloak (self-hosted).
-- These records are GN-WAAS profile data only.
-- Keycloak user IDs (keycloak_id) will be populated on first login.

INSERT INTO users (id, email, full_name, phone_number, role, status, organisation, employee_id) VALUES

-- ============================================================
-- SYSTEM ADMINISTRATORS (Your company)
-- ============================================================
('a0000001-0000-0000-0000-000000000001', 'superadmin@gnwaas.gov.gh',      'GN-WAAS Super Admin',        '+233200000001', 'SUPER_ADMIN',      'ACTIVE', 'GN-WAAS Managed Services', 'SYS-001'),
('a0000001-0000-0000-0000-000000000002', 'sysadmin@gnwaas.gov.gh',        'System Administrator',       '+233200000002', 'SYSTEM_ADMIN',     'ACTIVE', 'GN-WAAS Managed Services', 'SYS-002'),

-- ============================================================
-- GOVERNMENT ROLES
-- ============================================================
('a0000001-0000-0000-0000-000000000010', 'minister@mowsanitation.gov.gh', 'Minister of Water Resources', '+233200000010', 'MINISTER_VIEW',   'ACTIVE', 'Ministry of Water Resources & Sanitation', 'MOW-001'),
('a0000001-0000-0000-0000-000000000011', 'auditor1@mof.gov.gh',           'Senior Auditor - MoF',       '+233200000011', 'MOF_AUDITOR',      'ACTIVE', 'Ministry of Finance', 'MOF-001'),
('a0000001-0000-0000-0000-000000000012', 'auditor2@mof.gov.gh',           'Deputy Auditor - MoF',       '+233200000012', 'MOF_AUDITOR',      'ACTIVE', 'Ministry of Finance', 'MOF-002'),
('a0000001-0000-0000-0000-000000000013', 'graofficer1@gra.gov.gh',        'GRA Compliance Officer',     '+233200000013', 'GRA_OFFICER',      'ACTIVE', 'Ghana Revenue Authority', 'GRA-001'),
('a0000001-0000-0000-0000-000000000014', 'graofficer2@gra.gov.gh',        'GRA Senior Officer',         '+233200000014', 'GRA_OFFICER',      'ACTIVE', 'Ghana Revenue Authority', 'GRA-002'),

-- ============================================================
-- GWL OPERATIONAL ROLES
-- ============================================================
('a0000001-0000-0000-0000-000000000020', 'ceo@gwl.com.gh',                'GWL Chief Executive Officer', '+233200000020', 'GWL_EXECUTIVE',   'ACTIVE', 'Ghana Water Limited', 'GWL-EXEC-001'),
('a0000001-0000-0000-0000-000000000021', 'cfo@gwl.com.gh',                'GWL Chief Finance Officer',  '+233200000021', 'GWL_EXECUTIVE',    'ACTIVE', 'Ghana Water Limited', 'GWL-EXEC-002'),
('a0000001-0000-0000-0000-000000000022', 'manager.tema@gwl.com.gh',       'Tema District Manager',      '+233200000022', 'GWL_MANAGER',      'ACTIVE', 'Ghana Water Limited', 'GWL-MGR-001'),
('a0000001-0000-0000-0000-000000000023', 'manager.accrawest@gwl.com.gh',  'Accra West District Manager','+233200000023', 'GWL_MANAGER',      'ACTIVE', 'Ghana Water Limited', 'GWL-MGR-002'),
('a0000001-0000-0000-0000-000000000024', 'analyst1@gwl.com.gh',           'GWL Data Analyst',           '+233200000024', 'GWL_ANALYST',      'ACTIVE', 'Ghana Water Limited', 'GWL-ANA-001'),

-- ============================================================
-- FIELD OPERATIONS
-- ============================================================
('a0000001-0000-0000-0000-000000000030', 'supervisor.tema@gnwaas.gov.gh', 'Tema Field Supervisor',      '+233200000030', 'FIELD_SUPERVISOR', 'ACTIVE', 'GN-WAAS Managed Services', 'FLD-SUP-001'),
('a0000001-0000-0000-0000-000000000031', 'supervisor.accra@gnwaas.gov.gh','Accra Field Supervisor',     '+233200000031', 'FIELD_SUPERVISOR', 'ACTIVE', 'GN-WAAS Managed Services', 'FLD-SUP-002'),
('a0000001-0000-0000-0000-000000000040', 'officer.kwame@gnwaas.gov.gh',   'Kwame Asante',               '+233244000001', 'FIELD_OFFICER',    'ACTIVE', 'GN-WAAS Managed Services', 'FLD-OFF-001'),
('a0000001-0000-0000-0000-000000000041', 'officer.ama@gnwaas.gov.gh',     'Ama Boateng',                '+233244000002', 'FIELD_OFFICER',    'ACTIVE', 'GN-WAAS Managed Services', 'FLD-OFF-002'),
('a0000001-0000-0000-0000-000000000042', 'officer.kofi@gnwaas.gov.gh',    'Kofi Mensah',                '+233244000003', 'FIELD_OFFICER',    'ACTIVE', 'GN-WAAS Managed Services', 'FLD-OFF-003'),
('a0000001-0000-0000-0000-000000000043', 'officer.abena@gnwaas.gov.gh',   'Abena Osei',                 '+233244000004', 'FIELD_OFFICER',    'ACTIVE', 'GN-WAAS Managed Services', 'FLD-OFF-004'),
('a0000001-0000-0000-0000-000000000044', 'officer.yaw@gnwaas.gov.gh',     'Yaw Darko',                  '+233244000005', 'FIELD_OFFICER',    'ACTIVE', 'GN-WAAS Managed Services', 'FLD-OFF-005'),

-- ============================================================
-- MDA USERS (Government Ministries/Departments/Agencies)
-- ============================================================
('a0000001-0000-0000-0000-000000000050', 'water.mda@moh.gov.gh',          'Ministry of Health - Water', '+233200000050', 'MDA_USER',         'ACTIVE', 'Ministry of Health', 'MDA-MOH-001'),
('a0000001-0000-0000-0000-000000000051', 'water.mda@moe.gov.gh',          'Ministry of Education - Water','+233200000051','MDA_USER',        'ACTIVE', 'Ministry of Education', 'MDA-MOE-001');

-- Assign field officers to districts
UPDATE users SET district_id = (SELECT id FROM districts WHERE district_code = 'TEMA-EAST')
WHERE id IN (
    'a0000001-0000-0000-0000-000000000030',
    'a0000001-0000-0000-0000-000000000040',
    'a0000001-0000-0000-0000-000000000041'
);

UPDATE users SET district_id = (SELECT id FROM districts WHERE district_code = 'ACCRA-WEST')
WHERE id IN (
    'a0000001-0000-0000-0000-000000000031',
    'a0000001-0000-0000-0000-000000000042',
    'a0000001-0000-0000-0000-000000000043',
    'a0000001-0000-0000-0000-000000000044'
);

-- Assign managers to districts
UPDATE users SET district_id = (SELECT id FROM districts WHERE district_code = 'TEMA-EAST')
WHERE id = 'a0000001-0000-0000-0000-000000000022';

UPDATE users SET district_id = (SELECT id FROM districts WHERE district_code = 'ACCRA-WEST')
WHERE id = 'a0000001-0000-0000-0000-000000000023';
