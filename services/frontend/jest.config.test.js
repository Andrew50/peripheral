// Sample Jest configuration that properly handles setup files
export default {
    transform: {
        "^.+\\.svelte$": "svelte-jester",
        "^.+\\.ts$": "ts-jest",
        "^.+\\.js$": "ts-jest"
    },
    moduleFileExtensions: ["js", "ts", "svelte"],
    testEnvironment: "jsdom",
    setupFilesAfterEnv: ["<rootDir>/jest.setup.test.js"],
    testMatch: ["**/*.test.ts", "**/*.test.js"],
    moduleNameMapper: {
        "^\\$lib/(.*)$": "<rootDir>/src/lib/$1"
    },
    extensionsToTreatAsEsm: [".ts", ".svelte"],
    globals: {
        "ts-jest": {
            useESM: true
        }
    }
}; 