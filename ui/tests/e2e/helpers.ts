import { type Page, type Locator, expect } from '@playwright/test';

export const ADMIN_USERNAME = process.env.CRYO_ADMIN_USERNAME || 'admin';
export const ADMIN_PASSWORD = process.env.CRYO_ADMIN_PASSWORD || '';
export const NAMESPACE = process.env.CRYO_NAMESPACE || 'cryo-ui';

// The UI has no data-test attributes, so fields are located through their
// Quasar wrapper (.q-field) by exact label text. This keeps selectors stable
// against re-ordering and avoids matching substrings (e.g. "Name" vs "Secret
// Name").
export function field(page: Page, label: string): Locator {
  return page.locator('.q-field').filter({ has: page.getByText(label, { exact: true }) });
}

export async function fill(page: Page, label: string, value: string): Promise<void> {
  await field(page, label).locator('input, textarea').first().fill(value);
}

// Open a Quasar QSelect by its label and pick the option containing `optionText`.
export async function select(page: Page, label: string, optionText: string): Promise<void> {
  await field(page, label).click();
  await page.locator('.q-menu .q-item').filter({ hasText: optionText }).first().click();
}

export async function login(page: Page): Promise<void> {
  if (!ADMIN_PASSWORD) {
    throw new Error('CRYO_ADMIN_PASSWORD is not set — run hack/playwright-up.sh first');
  }
  await page.goto('/login');
  await fill(page, 'Username', ADMIN_USERNAME);
  await fill(page, 'Password', ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  // Successful login lands on the home page (no /login in the URL).
  await expect(page).toHaveURL(/\/$/);
}
