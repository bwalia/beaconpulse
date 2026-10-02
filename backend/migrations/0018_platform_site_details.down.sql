ALTER TABLE platform_settings
    DROP COLUMN IF EXISTS legal_address,
    DROP COLUMN IF EXISTS legal_entity,
    DROP COLUMN IF EXISTS support_email;
