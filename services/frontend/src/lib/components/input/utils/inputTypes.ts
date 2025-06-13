

import type { Instance } from '$lib/utils/types/types';

export const allKeys = ['ticker', 'timeframe', 'extendedHours', 'timestamp'] as const;
export type InstanceAttributes = (typeof allKeys)[number];

export interface InputQuery {
    // 'inactive': no UI shown
    // 'initializing' setting up event handlers
    // 'active': window is open waiting for input
    // 'complete': one field completed (may still be active if more required)
    // 'cancelled': user cancelled via Escape
    // 'shutdown': about to close and reset to inactive
    status: 'inactive' | 'initializing' | 'active' | 'complete' | 'cancelled' | 'shutdown';
    inputString: string;
    inputType: string;
    inputValid: boolean;
    instance: Instance;
    requiredKeys: InstanceAttributes[] | 'any';
    possibleKeys: InstanceAttributes[];
    securities?: Instance[];
}