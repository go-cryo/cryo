import { test, expect } from '@playwright/test';
import { NAMESPACE, fill, login, select } from './helpers';

// End-to-end UI flow against a live controller + RustFS:
//   login -> create host -> repository -> S3 backup job -> run it ->
//   watch the run succeed -> browse the resulting snapshot -> logout.
//
// Runs as a single ordered test because each step depends on the previous one's
// server-side state.
test('full backup lifecycle through the UI', async ({ page }) => {
  const hostName = 'ui-host';
  const repoName = 'ui-repo';
  const jobName = 'ui-job';

  test.setTimeout(180_000);

  await test.step('log in', async () => {
    await login(page);
  });

  await test.step('create a RustFS host', async () => {
    await page.goto('/hosts/add');
    await fill(page, 'Secret Name', hostName);
    await fill(page, 'Namespace', NAMESPACE);
    await select(page, 'Host Type', 'S3');
    await fill(page, 'Base URL', 's3:http://rustfs:9000/cryo-repo');
    await fill(page, 'AWS Access Key ID', 'rustfsadmin');
    await fill(page, 'AWS Secret Access Key', 'rustfsadmin');
    await page.getByRole('button', { name: 'Create Host' }).click();
    await expect(page).toHaveURL(new RegExp(`/hosts/${NAMESPACE}/${hostName}$`));
  });

  await test.step('create a repository on the host', async () => {
    await page.goto('/repositories/add');
    await fill(page, 'Secret Name', repoName);
    await fill(page, 'Namespace', NAMESPACE);
    await select(page, 'Host', hostName);
    await fill(page, 'Repository Path', 'ui-e2e');
    await fill(page, 'Restic Password', 'ui-e2e-password');
    await page.getByRole('button', { name: 'Create Repository' }).click();
    await expect(page).toHaveURL(new RegExp(`/repositories/${NAMESPACE}/${repoName}$`));
  });

  await test.step('create an S3 backup job', async () => {
    await page.goto('/backupjobs/add');
    await fill(page, 'Name', jobName);
    await fill(page, 'Namespace', NAMESPACE);
    await select(page, 'Backup Type', 'S3');
    await fill(page, 'Schedule', '0 0 1 1 *');
    await select(page, 'Repository', repoName);
    await fill(page, 'Endpoint', 'http://rustfs:9000');
    await fill(page, 'Bucket', 'test-source');
    await fill(page, 'Access Key', 'rustfsadmin');
    await fill(page, 'Secret Key', 'rustfsadmin');
    await page.getByRole('button', { name: 'Create Backup Job' }).click();
    await expect(page).toHaveURL(new RegExp(`/backupjobs/${NAMESPACE}/${jobName}$`));
  });

  await test.step('trigger the backup and watch it succeed', async () => {
    await page.getByRole('button', { name: 'Run Now' }).click();

    // The run executes asynchronously in-cluster and the runs table does not
    // auto-refresh, so reload-poll the status badge until it reports success.
    // The brief wait after reload lets the page's onMounted fetchRuns() resolve
    // before we read the table.
    await expect
      .poll(
        async () => {
          await page.reload();
          await page.waitForTimeout(1_000);
          if (await page.getByText('Failed', { exact: true }).count()) {
            throw new Error('backup run reported Failed');
          }
          return page.getByText('Succeeded', { exact: true }).count();
        },
        { timeout: 150_000, intervals: [2_500] },
      )
      .toBeGreaterThan(0);
  });

  await test.step('browse the snapshot created by the backup', async () => {
    await page.goto(`/repositories/${NAMESPACE}/${repoName}`);

    // The controller lists snapshots via restic against RustFS. Wait for the
    // backup's snapshot to appear, reloading until restic reports it.
    const browseButton = page.locator('button').filter({ has: page.locator('.q-icon', { hasText: 'folder_open' }) });
    await expect
      .poll(
        async () => {
          await page.reload();
          await page.waitForTimeout(1_000);
          return browseButton.count();
        },
        { timeout: 60_000, intervals: [2_500] },
      )
      .toBeGreaterThan(0);

    await browseButton.first().click();
    await expect(page).toHaveURL(/\/snapshots\//);
    // The snapshot file tree renders at least one entry.
    await expect(page.locator('.q-table tbody tr').first()).toBeVisible();
  });

  await test.step('log out', async () => {
    await page.locator('button').filter({ has: page.locator('.q-icon', { hasText: 'logout' }) }).click();
    await expect(page).toHaveURL(/\/login$/);
  });
});
