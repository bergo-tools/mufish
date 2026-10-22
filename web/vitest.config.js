import { defineConfig } from 'vitest/config';

export default defineConfig({
  oxc: {
    jsx: {
      runtime: 'automatic',
      importSource: 'preact',
      development: false,
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    css: false,
    include: ['test/components/**/*.test.jsx'],
    setupFiles: ['./test/components/setup.js'],
  },
});
