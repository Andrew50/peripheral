export const load = async ({ fetch }: { fetch: any }) => {
	try {
		// Get backend URL - handle both development and production
		const backendUrl = process.env.BACKEND_URL || 'http://localhost:5058';
		
		// 1. Fetch SPY security ID
		const securityResponse = await fetch(`${backendUrl}/public`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				func: 'getSecurityIDFromTickerTimestamp',
				args: {
					ticker: 'SPY',
					timestampMs: 0
				}
			})
		});

		if (!securityResponse.ok) {
			throw new Error(`Security ID fetch failed: ${securityResponse.status}`);
		}

		const securityData = await securityResponse.json();
		const securityId = securityData?.securityId ?? 0;

		if (securityId === 0) {
			throw new Error('Could not get SPY security ID');
		}

		// 2. Fetch chart data for SPY
		const chartResponse = await fetch(`${backendUrl}/public`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				func: 'getChartData',
				args: {
					securityId: securityId,
					timeframe: '1d',
					timestamp: 0,
					direction: 'backward',
					bars: 400,
					extendedhours: false,
					isreplay: false,
					includeSECFilings: true
				}
			})
		});

		if (!chartResponse.ok) {
			throw new Error(`Chart data fetch failed: ${chartResponse.status}`);
		}

		const chartData = await chartResponse.json();
		// 3. Return preloaded chart data
		const defaultChartData = {
			ticker: 'SPY',
			timeframe: '1d', 
			timestamp: 0,
			securityId: securityId,
			price: 0,
			chartData: chartData,
			bars: chartData.bars?.length || 400
		};

		return {
			defaultChartData
		};

	} catch (error) {
		console.error('Chart preloading failed:', error instanceof Error ? error.message : String(error));
		
		// Return null so client-side loading can take over as fallback
		return {
			defaultChartData: null
		};
	}
}; 