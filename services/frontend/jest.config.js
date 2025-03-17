export default {
    transform: {
        "^.+\\.svelte$": "svelte-jester",
        "^.+\\.ts$": "ts-jest"
    },
    moduleFileExtensions: ["js", "ts", "svelte"],
    testEnvironment: "jsdom",
    setupFilesAfterEnv: ["@testing-library/jest-dom/extend-expect"],
    testMatch: ["**/*.test.ts", "**/*.test.js"],
    moduleNameMapper: {
        "^\\$lib/(.*)$": "<rootDir>/src/lib/$1"
    }
}; 