import { defineConfig, devices } from '@playwright/test';

// The suite drives the controller-served UI against a live KIND cluster + RustFS.
// hack/playwright-up.sh deploys the stack and port-forwards it to CRYO_BASE_URL.
const baseURL = process.env.CRYO_BASE_URL || 'http://localhost:8080';

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 150_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
