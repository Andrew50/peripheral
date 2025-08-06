-- Migration 084: Update queries limit for Free and Plus subscription products
-- Description: Increase Free plan queries_limit from 5 to 15 and Plus plan queries_limit from 250 to 300

BEGIN;

-- Update Free plan (id=1) queries_limit from 5 to 15
UPDATE subscription_products 
SET queries_limit = 10, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND product_key = 'Free';
-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    87,
    'Update queries_limit for Free (5->10)'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 