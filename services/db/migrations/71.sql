-- Migration: 071_update_plus_yearly_price_id
-- Description: Update Plus yearly subscription price ID to new Stripe price ID

BEGIN;

-- Update the Plus yearly price ID with conditional logic
DO $$
BEGIN
    -- Check if the old price ID exists and update it
    IF EXISTS (
        SELECT 1 FROM prices 
        WHERE product_key = 'Plus' 
          AND billing_period = 'yearly' 
          AND stripe_price_id_live = 'price_1RhmtGGCLCMUwjFlzcETQoMj'
    ) THEN
        -- Update the old price ID to the new one
        UPDATE prices 
        SET stripe_price_id_live = 'price_1Rl0HeGCLCMUwjFl28hzmCvv',
            updated_at = CURRENT_TIMESTAMP
        WHERE product_key = 'Plus' 
          AND billing_period = 'yearly' 
          AND stripe_price_id_live = 'price_1RhmtGGCLCMUwjFlzcETQoMj';
        
        RAISE NOTICE 'Updated Plus yearly price ID from price_1RhmtGGCLCMUwjFlzcETQoMj to price_1Rl0HeGCLCMUwjFl28hzmCvv';
    
    ELSIF EXISTS (
        SELECT 1 FROM prices 
        WHERE product_key = 'Plus' 
          AND billing_period = 'yearly' 
          AND stripe_price_id_live = 'price_1Rl0HeGCLCMUwjFl28hzmCvv'
    ) THEN
        -- New price ID already exists, no update needed
        RAISE NOTICE 'Plus yearly price ID already set to price_1Rl0HeGCLCMUwjFl28hzmCvv - no update needed';
    
    ELSE
        -- Neither old nor new price ID found, this might indicate a problem
        RAISE NOTICE 'Plus yearly price record not found - this migration may have already been applied or the price structure has changed';
    END IF;
END $$;

-- Update schema version
INSERT INTO schema_versions (version, description)
VALUES (
    71,
    'Update Plus yearly subscription price ID to new Stripe price ID'
) ON CONFLICT (version) DO NOTHING;

COMMIT; 