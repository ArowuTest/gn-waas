-- ============================================================
-- GN-WAAS Seed Data: 007_password_hashes
-- Description: bcrypt password hashes for all demo/staging users
--              Enables native email+password login without Keycloak
--              Cost factor: 12 (production-grade)
-- ============================================================
-- Password reference (for demo use only):
--   SUPER_ADMIN / SYSTEM_ADMIN : Admin@GN2026!
--   MINISTER_VIEW              : Minister@2026!
--   MOF_AUDITOR                : MoF@Audit2026!
--   GRA_OFFICER                : GRA@Officer2026!
--   GWL_EXECUTIVE              : GWL@Exec2026!
--   GWL_MANAGER                : GWL@Manager2026!
--   GWL_ANALYST                : GWL@Analyst2026!
--   GWL_SUPERVISOR             : GWL@Super2026!
--   FIELD_SUPERVISOR           : Field@Super2026!
--   FIELD_OFFICER              : Field@Officer2026!
--   MDA_USER                   : MDA@User2026!
-- ============================================================

SET LOCAL app.bypass_rls = 'true';

-- SUPER_ADMIN / SYSTEM_ADMIN
UPDATE users SET password_hash = '$2b$12$wwFQC0bcCOJmuWd2T.F8EuALuNR2Zq6bVLwa1ongn1kbB8nFXHYDe' WHERE email = 'superadmin@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$/XLTavlnCQqq/2KUdYTH7elk6Aw3vMuufYhISQ4ysK5dEeHWvK.Hq' WHERE email = 'sysadmin@gnwaas.gov.gh';

-- GOVERNMENT
UPDATE users SET password_hash = '$2b$12$aBd.ei3r8Ehyam8o8e2FrO0kuyXdWKHak45Mx9f4CMMurwP.t7bfq' WHERE email = 'minister@mowsanitation.gov.gh';
UPDATE users SET password_hash = '$2b$12$xepClk0zZQFCQurfIs44GO0bVvTPA9HAfMiGSl.0./Oyiv9q5ArYq' WHERE email = 'auditor1@mof.gov.gh';
UPDATE users SET password_hash = '$2b$12$wr8NJRdg9jpdhrYM1ty2e.pfZV9.gU7jG2UDbWYMaz0fgIM03Irdy' WHERE email = 'auditor2@mof.gov.gh';
UPDATE users SET password_hash = '$2b$12$h3YNvR88SQsmnTn8PcA.N.elbS5.N4Mi4J1gqL.8CfMdkGlGvxK62' WHERE email = 'graofficer1@gra.gov.gh';
UPDATE users SET password_hash = '$2b$12$QnkP71byDq7NyxJ9hjrkguSTvuJfpQ2K2VSxb2H4j3HirMpiHgaWu' WHERE email = 'graofficer2@gra.gov.gh';

-- GWL
UPDATE users SET password_hash = '$2b$12$a9FjNxjz3NtihbuxztGH/eTqi.DZ/x4jIF4s6VcyrEM/Nd0qY8eq.' WHERE email = 'ceo@gwl.com.gh';
UPDATE users SET password_hash = '$2b$12$Tyqv.cUR6nLZkcfpqHlq0.VA9VmWiJ6H648nj69eOJ2O8AL9IyHl.' WHERE email = 'cfo@gwl.com.gh';
UPDATE users SET password_hash = '$2b$12$Tl1OQCLBGTtH/C2X5wp8uO3XxPmfFBIep0dlrQKJmfxhztwSCa306' WHERE email = 'manager.tema@gwl.com.gh';
UPDATE users SET password_hash = '$2b$12$YzaTiNiFJkWqbd4h1B2wZ.nFNyUW7ffcAJ1Os4j8nn7s4JB1dwZ7u' WHERE email = 'manager.accrawest@gwl.com.gh';
UPDATE users SET password_hash = '$2b$12$Jd06HE35S/MjdkEwYi2k0.CDbT/1v6Homn1eXK43FP67NsXpCo2RS' WHERE email = 'analyst1@gwl.com.gh';
UPDATE users SET password_hash = '$2b$12$Hj/To/niR/.H56BMM2P0JOQhwhqyNrBwZFUHYj45JYTiG.YvY/KgO' WHERE email = 'supervisor@gwl.com.gh';

-- FIELD
UPDATE users SET password_hash = '$2b$12$b9EYBFE/zol3mHWbDitsAOHH.kwe8Q1BEe/oCSPb04.JN2qd/NoNm' WHERE email = 'supervisor.tema@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$KV8yfMYllmIWh66v9P7oFuivJyzNyVMAbWjcn/fQRKaSoKLnSKHeO' WHERE email = 'supervisor.accra@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$p2RgoXd31yBDJUxyDGEBIe8powlwC9G7/FvnoFXoEW5HpBrRuP2v.' WHERE email = 'officer.kwame@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$Bk5DYjbfk./anG38XzvQ3e2f8Rlymr4SLU1mBKXuzO5sdE7ThOH1a' WHERE email = 'officer.ama@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$KAJr3v4ElGI9Yquf20NGK.UUcKb9UzdTNRsscXTG0.UClqA.IehcS' WHERE email = 'officer.kofi@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$7.vU6q19FRZ99jk.rF1jNeAo17IbaxGHKUybum0.WlwsTBL.LEmgq' WHERE email = 'officer.abena@gnwaas.gov.gh';
UPDATE users SET password_hash = '$2b$12$XKrFtgrkojIh43iRysg6legTHR3YWIXCxUO452ERbpWmK7r./YNBe' WHERE email = 'officer.yaw@gnwaas.gov.gh';

-- MDA
UPDATE users SET password_hash = '$2b$12$.1Z01zO.Kq9Uhenb51CWee7D0vjzf7xxLXxiGSEXJ25Kw3CS4bOoS' WHERE email = 'water.mda@moh.gov.gh';
UPDATE users SET password_hash = '$2b$12$x5FGVq.uadGIL2Eowdt7J.XiJJTiNn1fnPUfEHirs2fS/4rtIASU2' WHERE email = 'water.mda@moe.gov.gh';
