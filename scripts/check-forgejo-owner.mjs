// MUTATING acceptance check: use only an empty disposable matching-native Forgejo.
// Requires the Soda custom tree and active Soda PAM source. No real credentials.
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
const require = createRequire(new URL('../cockpit/package.json', import.meta.url));
const { chromium } = require('playwright');
const [address, output = '.artifacts/branding/owner'] = process.argv.slice(2);
if (!address) throw new Error('Usage: node scripts/check-forgejo-owner.mjs DISPOSABLE_URL [SCREENSHOTS]');
const base = address.replace(/\/$/, '');
await mkdir(output, { recursive: true });
const browser = await chromium.launch();
try {
  for (const colorScheme of ['light', 'dark']) {
    const context = await browser.newContext({ colorScheme, viewport: { width: 1280, height: 1000 } });
    const page = await context.newPage();
    await page.goto(base, { waitUntil: 'networkidle' });
    assert(await page.getByRole('heading', { name: 'Create your Forgejo administrator account' }).isVisible(), 'Requires an empty disposable instance');
    await page.screenshot({ path: `${output}/home-${colorScheme}.png`, fullPage: true });
    await page.getByRole('link', { name: 'Create administrator account', exact: true }).click();
    assert(page.url().endsWith('/user/sign_up'));
    assert(await page.locator('[name=user_name]').isVisible());
    assert(await page.getByText(/Choose independent Forgejo credentials/).isVisible());
    for (const width of [390, 320]) {
      await page.setViewportSize({ width, height: 844 });
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      await page.screenshot({ path: `${output}/signup-${colorScheme}-${width}.png`, fullPage: true });
    }
    await page.goto(`${base}/user/login`, { waitUntil: 'networkidle' });
    assert(page.url().endsWith('/user/sign_up'), 'Early sign-in must lead to owner registration');
    await context.close();
  }
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  for (const path of ['/api/v1/user', '/soda-owner-review/probe.git/info/refs?service=git-upload-pack']) {
    const response = await context.request.get(`${base}${path}`, { headers: { Authorization: `Basic ${Buffer.from('soda-owner-review:not-used').toString('base64')}` } });
    assert.equal(response.status(), 401);
    assert.match(await response.text(), /first-owner registration/);
  }
  await page.goto(`${base}/user/sign_up`);
  const name = 'soda-owner-review';
  const password = randomBytes(24).toString('base64url');
  await page.locator('[name=user_name]').fill(name);
  await page.locator('[name=email]').fill(`${name}@example.test`);
  await page.locator('[name=password]').fill(password);
  await page.locator('[name=retype]').fill(`${password}different`);
  await page.locator('form button').click();
  assert(await page.getByRole('heading', { name: 'Create your Forgejo administrator account' }).isVisible());
  assert(await page.locator('.ui.negative.message').isVisible(), 'Invalid signup must report an error and remain retryable');
  await page.locator('[name=password]').fill(password);
  await page.locator('[name=retype]').fill(password);
  await page.locator('form button').click();
  await page.waitForURL(`${base}/admin`);
  assert(await page.getByText(/Your Forgejo administrator account is ready/).isVisible());
  const userResponse = await context.request.get(`${base}/api/v1/user`, { headers: { Authorization: `Basic ${Buffer.from(`${name}:${password}`).toString('base64')}` } });
  assert.equal(userResponse.status(), 200);
  const user = await userResponse.json();
  assert.equal(user.is_admin, true);
  assert.equal(user.id, 1, 'Earlier requests and failed signup must not consume an account');
  await page.screenshot({ path: `${output}/administrator.png`, fullPage: true });
  const guest = await browser.newContext();
  const guestPage = await guest.newPage();
  await guestPage.goto(base);
  assert.equal(await guestPage.locator('#soda-owner-title').count(), 0);
  await guestPage.goto(`${base}/user/login`);
  assert(guestPage.url().endsWith('/user/login'), 'Established instance retains native sign-in');
  assert.deepEqual(errors, []);
  await guest.close();
  await context.close();
  console.log('Owner browser checks passed: guidance, light/dark/mobile, early web/API/Git rejection, signup retry, native admin confirmation, and established sign-in.');
} finally {
  await browser.close();
}
