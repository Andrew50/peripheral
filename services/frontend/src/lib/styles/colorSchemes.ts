// Color schemes definitions
export interface ColorScheme {
	name: string;
	// Base UI Colors
	c1: string; // darker grey
	c2: string; // base dark background
	c3: string; // accent (modern blue)
	c4: string; // separator (medium grey)
	c5: string; // error (modern red)
	c6: string; // success (modern green)

	// Hover states
	c3Hover: string; // blue hover
	c6Hover: string; // green hover

	// Font Colors
	f1: string; // primary text (off white)
	f2: string; // secondary text (light grey)

	// Additional Accents
	accentPurple: string;
	accentIndigo: string;
	accentYellow: string;

	// UI Theme Colors
	uiBgPrimary: string;
	uiBgSecondary: string;
	uiBgHover: string;
	uiBgElement: string;
	uiBgHighlight: string;
	uiBorder: string;
	uiAccent: string;

	// Text Colors
	textPrimary: string;
	textSecondary: string;

	// Up/Down Colors
	colorUp: string;
	colorUpStrong: string;
	colorDown: string;
	colorDownStrong: string;
}

// Default color scheme (current colors)
export const defaultScheme: ColorScheme = {
	name: 'Default',
	c1: '#2e2e2e',
	c2: '#1a1c21',
	c3: '#3b82f6',
	c4: '#374151',
	c5: '#ef4444',
	c6: '#22c55e',

	c3Hover: '#2563eb',
	c6Hover: '#16a34a',

	f1: '#f9fafb',
	f2: '#9ca3af',

	accentPurple: '#8b5cf6',
	accentIndigo: '#6366f1',
	accentYellow: '#eab308',

	uiBgPrimary: 'rgba(0, 0, 0, 0.85)',
	uiBgSecondary: 'rgba(0, 0, 0, 0.3)',
	uiBgHover: 'rgba(255, 255, 255, 0.05)',
	uiBgElement: 'rgba(255, 255, 255, 0.03)',
	uiBgHighlight: '#222',
	uiBorder: 'rgba(255, 255, 255, 0.1)',
	uiAccent: '#3b82f6',

	textPrimary: '#e0e0e0',
	textSecondary: '#999',

	colorUp: '#66bb6a',
	colorUpStrong: '#43a047',
	colorDown: '#ef5350',
	colorDownStrong: '#e53935'
};

// Dark blue theme
export const darkBlueScheme: ColorScheme = {
	name: 'Dark Blue',
	c1: '#1e2433',
	c2: '#121726',
	c3: '#4c63b6',
	c4: '#2a3652',
	c5: '#e53e3e',
	c6: '#38a169',

	c3Hover: '#3849a2',
	c6Hover: '#2f855a',

	f1: '#f7fafc',
	f2: '#a0aec0',

	accentPurple: '#805ad5',
	accentIndigo: '#5a67d8',
	accentYellow: '#d69e2e',

	uiBgPrimary: 'rgba(6, 8, 20, 0.85)',
	uiBgSecondary: 'rgba(6, 8, 20, 0.3)',
	uiBgHover: 'rgba(76, 99, 182, 0.1)',
	uiBgElement: 'rgba(76, 99, 182, 0.05)',
	uiBgHighlight: '#1a202c',
	uiBorder: 'rgba(76, 99, 182, 0.2)',
	uiAccent: '#4c63b6',

	textPrimary: '#e2e8f0',
	textSecondary: '#a0aec0',

	colorUp: '#68d391',
	colorUpStrong: '#48bb78',
	colorDown: '#fc8181',
	colorDownStrong: '#f56565'
};

// Midnight theme
export const midnightScheme: ColorScheme = {
	name: 'Midnight',
	c1: '#171923',
	c2: '#0D1117',
	c3: '#6b46c1',
	c4: '#2d3748',
	c5: '#e53e3e',
	c6: '#38a169',

	c3Hover: '#553c9a',
	c6Hover: '#2f855a',

	f1: '#f7fafc',
	f2: '#a0aec0',

	accentPurple: '#9f7aea',
	accentIndigo: '#7a5af8',
	accentYellow: '#ecc94b',

	uiBgPrimary: 'rgba(0, 0, 0, 0.9)',
	uiBgSecondary: 'rgba(0, 0, 0, 0.4)',
	uiBgHover: 'rgba(107, 70, 193, 0.1)',
	uiBgElement: 'rgba(107, 70, 193, 0.05)',
	uiBgHighlight: '#1a202c',
	uiBorder: 'rgba(107, 70, 193, 0.2)',
	uiAccent: '#6b46c1',

	textPrimary: '#e2e8f0',
	textSecondary: '#a0aec0',

	colorUp: '#68d391',
	colorUpStrong: '#48bb78',
	colorDown: '#fc8181',
	colorDownStrong: '#f56565'
};

// Forest theme
export const forestScheme: ColorScheme = {
	name: 'Forest',
	c1: '#1E2B20',
	c2: '#111a12',
	c3: '#48bb78',
	c4: '#2c392e',
	c5: '#f56565',
	c6: '#38a169',

	c3Hover: '#3da066',
	c6Hover: '#2f855a',

	f1: '#f0fff4',
	f2: '#9ae6b4',

	accentPurple: '#9f7aea',
	accentIndigo: '#667eea',
	accentYellow: '#ecc94b',

	uiBgPrimary: 'rgba(0, 20, 0, 0.85)',
	uiBgSecondary: 'rgba(0, 20, 0, 0.3)',
	uiBgHover: 'rgba(72, 187, 120, 0.1)',
	uiBgElement: 'rgba(72, 187, 120, 0.05)',
	uiBgHighlight: '#1c2a1e',
	uiBorder: 'rgba(72, 187, 120, 0.2)',
	uiAccent: '#48bb78',

	textPrimary: '#e6ffec',
	textSecondary: '#9ae6b4',

	colorUp: '#68d391',
	colorUpStrong: '#48bb78',
	colorDown: '#fc8181',
	colorDownStrong: '#f56565'
};

// Sunset theme
export const sunsetScheme: ColorScheme = {
	name: 'Sunset',
	c1: '#2D2327',
	c2: '#1A1618',
	c3: '#ed8936',
	c4: '#413035',
	c5: '#e53e3e',
	c6: '#38a169',

	c3Hover: '#dd6b20',
	c6Hover: '#2f855a',

	f1: '#fff5f5',
	f2: '#fed7d7',

	accentPurple: '#b794f4',
	accentIndigo: '#7f9cf5',
	accentYellow: '#faf089',

	uiBgPrimary: 'rgba(30, 10, 10, 0.85)',
	uiBgSecondary: 'rgba(30, 10, 10, 0.3)',
	uiBgHover: 'rgba(237, 137, 54, 0.1)',
	uiBgElement: 'rgba(237, 137, 54, 0.05)',
	uiBgHighlight: '#2d2022',
	uiBorder: 'rgba(237, 137, 54, 0.2)',
	uiAccent: '#ed8936',

	textPrimary: '#fffafa',
	textSecondary: '#fed7d7',

	colorUp: '#68d391',
	colorUpStrong: '#48bb78',
	colorDown: '#fc8181',
	colorDownStrong: '#f56565'
};

// Grayscale theme
export const grayscaleScheme: ColorScheme = {
	name: 'Grayscale',
	c1: '#424242',
	c2: '#212121',
	c3: '#757575',
	c4: '#616161',
	c5: '#ef4444', // Use standard red for error indication
	c6: '#22c55e', // Use standard green for success indication

	c3Hover: '#616161',
	c6Hover: '#16a34a', // Use standard green hover

	f1: '#f5f5f5',
	f2: '#bdbdbd',

	accentPurple: '#9e9e9e',
	accentIndigo: '#757575',
	accentYellow: '#eeeeee',

	uiBgPrimary: '#303030',
	uiBgSecondary: '#424242',
	uiBgHover: '#515151',
	uiBgElement: '#353535',
	uiBgHighlight: '#515151',
	uiBorder: '#616161',
	uiAccent: '#9e9e9e',

	textPrimary: '#f5f5f5',
	textSecondary: '#bdbdbd',

	colorUp: '#66bb6a', // Standard green
	colorUpStrong: '#43a047',
	colorDown: '#ef5350', // Standard red
	colorDownStrong: '#e53935'
};

export const colorSchemes: Record<string, ColorScheme> = {
	default: defaultScheme,
	'dark-blue': darkBlueScheme,
	midnight: midnightScheme,
	forest: forestScheme,
	sunset: sunsetScheme,
	grayscale: grayscaleScheme
};

// Function to apply a color scheme to CSS variables
export function applyColorScheme(scheme: ColorScheme): void {
	document.documentElement.style.setProperty('--c1', scheme.c1);
	document.documentElement.style.setProperty('--c2', scheme.c2);
	document.documentElement.style.setProperty('--c3', scheme.c3);
	document.documentElement.style.setProperty('--c4', scheme.c4);
	document.documentElement.style.setProperty('--c5', scheme.c5);
	document.documentElement.style.setProperty('--c6', scheme.c6);

	document.documentElement.style.setProperty('--c3-hover', scheme.c3Hover);
	document.documentElement.style.setProperty('--c6-hover', scheme.c6Hover);

	document.documentElement.style.setProperty('--f1', scheme.f1);
	document.documentElement.style.setProperty('--f2', scheme.f2);

	document.documentElement.style.setProperty('--accent-purple', scheme.accentPurple);
	document.documentElement.style.setProperty('--accent-indigo', scheme.accentIndigo);
	document.documentElement.style.setProperty('--accent-yellow', scheme.accentYellow);

	document.documentElement.style.setProperty('--ui-bg-primary', scheme.uiBgPrimary);
	document.documentElement.style.setProperty('--ui-bg-secondary', scheme.uiBgSecondary);
	document.documentElement.style.setProperty('--ui-bg-hover', scheme.uiBgHover);
	document.documentElement.style.setProperty('--ui-bg-element', scheme.uiBgElement);
	document.documentElement.style.setProperty('--ui-bg-highlight', scheme.uiBgHighlight);
	document.documentElement.style.setProperty('--ui-border', scheme.uiBorder);
	document.documentElement.style.setProperty('--ui-accent', scheme.uiAccent);

	document.documentElement.style.setProperty('--text-primary', scheme.textPrimary);
	document.documentElement.style.setProperty('--text-secondary', scheme.textSecondary);

	document.documentElement.style.setProperty('--color-up', scheme.colorUp);
	document.documentElement.style.setProperty('--color-up-strong', scheme.colorUpStrong);
	document.documentElement.style.setProperty('--color-down', scheme.colorDown);
	document.documentElement.style.setProperty('--color-down-strong', scheme.colorDownStrong);
}
