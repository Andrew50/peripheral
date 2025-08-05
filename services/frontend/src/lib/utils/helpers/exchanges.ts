export const exchangeMicToName: Record<string, string> = {
	// US Equities
	XNAS: 'NASDAQ',
	XNYS: 'NYSE',
	ARCX: 'NYSE Arca',
	BATS: 'CBOE',
	IEXG: 'IEX',
	EDGX: 'CBOE',
	EDGA: 'CBOE'
};

export function getExchangeName(micCode: string | null | undefined): string {
	if (!micCode) {
		return 'N/A';
	}
	return exchangeMicToName[micCode] || micCode; // Return code if not found
}
