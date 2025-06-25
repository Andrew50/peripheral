export const exchangeMicToName: Record<string, string> = {
	// US Equities
	XNAS: 'NASDAQ',
	XNYS: 'NYSE',
	ARCX: 'NYSE Arca',
	BATS: 'CBOE',
	IEXG: 'IEX',
	EDGX: 'CBOE',
	EDGA: 'CBOE'
	// Add other common exchanges as needed

	// US Options (Example - adjust if needed)
	// 'OPRA': 'OPRA', // Consolidated Tape
	// 'AMXO': 'NYSE American Options',
	// 'BOXO': 'BOX Options',
	// 'CBOE': 'Cboe Options',
	// ... more option exchanges

	// Other markets (Examples)
	// 'XTSX': 'TSX Venture Exchange',
	// 'XTSE': 'Toronto Stock Exchange',
	// 'XLON': 'London Stock Exchange',
};

export function getExchangeName(micCode: string | null | undefined): string {
	if (!micCode) {
		return 'N/A';
	}
	return exchangeMicToName[micCode.toUpperCase()] || micCode; // Return code if not found
}
