-- Migration 084: Update queries limit for Free and Plus subscription products
-- Description: Increase Free plan queries_limit from 5 to 15 and Plus plan queries_limit from 250 to 300

BEGIN;

-- Update Free plan (id=1) queries_limit from 5 to 15
UPDATE subscription_products 
SET queries_limit = 15, updated_at = CURRENT_TIMESTAMP
WHERE id = 1 AND product_key = 'Free';

-- Update Plus plan (id=2) queries_limit from 250 to 300
UPDATE subscription_products 
SET queries_limit = 300, updated_at = CURRENT_TIMESTAMP
WHERE id = 2 AND product_key = 'Plus';

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    84,
    'Update queries_limit for Free (5->15) and Plus (250->300) subscription products'
) ON CONFLICT (version) DO NOTHING;


COMMIT; 