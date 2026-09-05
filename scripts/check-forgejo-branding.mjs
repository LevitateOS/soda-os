#!/usr/bin/env node
// Read-only browser checks against the review sheet on a disposable Forgejo.
// Prerequisites: cockpit's pinned Playwright and a matching-native browser.
import assert from 'node:assert/strict';
import { mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';

const require = createRequire(new URL('../cockpit/package.json', import.meta.url));
const { chromium } = require('playwright');
const [preview, evidence] = process.argv.slice(2);
assert(preview && evidence && process.argv.length === 4,
  'usage: node scripts/check-forgejo-branding.mjs PREVIEW_URL EVIDENCE_DIRECTORY');
const url = new URL(preview);
await mkdir(evidence, { recursive: true });
const browser = await chromium.launch({ headless: true });
const semanticNames = [
  '--color-diff-added-row-bg', '--color-diff-removed-row-bg',
  '--color-diff-added-word-bg', '--color-diff-removed-word-bg',
  '--color-ansi-red', '--color-ansi-green', '--color-console-bg',
  '--color-error-bg', '--color-error-text',
];

function contrast(left, right) {
  function luminance(rgb) {
    const values = rgb.match(/[\d.]+/g).slice(0, 3).map(Number).map(value => {
      const channel = value / 255;
      return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    });
    return values.reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
  }
  const a = luminance(left), b = luminance(right);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

async function colors(locator) {
  return locator.evaluate(async element => {
    // Flush style, then sample the end of native CSS transitions, not a frame
    // between normal/hover/active colors.
    getComputedStyle(element).backgroundColor;
    await Promise.all(element.getAnimations().map(animation => animation.finished));
    const style = getComputedStyle(element);
    return { color: style.color, background: style.backgroundColor };
  });
}

async function visit(page, theme) {
  url.searchParams.set('theme', theme);
  await page.goto(url.href);
  await page.evaluate(() => document.fonts.ready);
  return page.evaluate(names => {
    const style = getComputedStyle(document.documentElement);
    return Object.fromEntries(names.map(name => [name, style.getPropertyValue(name).trim()]));
  }, semanticNames);
}

async function checkButtons(page) {
  let activeBackground;
  for (const selector of ['#primary', '.button.primary:not(.ui)', '.ui.primary.buttons .button:not(.active)']) {
    const button = page.locator(selector);
    await page.mouse.move(0, 0);
    const normal = await colors(button);
    await button.hover();
    const hover = await colors(button);
    await page.mouse.down();
    const active = await colors(button);
    activeBackground = active.background;
    await page.mouse.up();
    for (const [state, value] of Object.entries({ normal, hover, active })) {
      assert(contrast(value.color, value.background) >= 4.5, `${selector} ${state}: ${JSON.stringify(value)}`);
    }
    assert.notEqual(normal.background, hover.background, `${selector} hover feedback`);
    assert.notEqual(hover.background, active.background, `${selector} active feedback`);
  }
  for (const selector of ['.ui.primary.button.active', '.ui.primary.buttons .button.active']) {
    const selected = await colors(page.locator(selector));
    assert.equal(selected.background, activeBackground, `${selector} selected feedback`);
    assert(contrast(selected.color, selected.background) >= 4.5);
  }
  assert(await page.locator('button[disabled]').isDisabled());
}

async function checkFocusAndImages(page) {
  await page.locator('#repository').focus();
  const focus = await page.locator('#repository').evaluate(element => {
    const style = getComputedStyle(element);
    return { width: style.outlineWidth, style: style.outlineStyle };
  });
  assert.deepEqual(focus, { width: '2px', style: 'solid' });
  const problems = await page.evaluate(() => ({
    overflow: document.documentElement.scrollWidth > innerWidth,
    broken: [...document.images].filter(image => !image.complete || !image.naturalWidth).map(image => image.src),
  }));
  assert.deepEqual(problems, { overflow: false, broken: [] });
}

try {
  for (const scheme of ['light', 'dark']) {
    const context = await browser.newContext({ colorScheme: scheme, viewport: { width: 1280, height: 1000 } });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => { if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`); });
    const baseline = await visit(page, `forgejo-${scheme}`);
    for (const [name, value] of Object.entries(baseline)) assert(value, `Missing native semantic color: ${name}`);
    for (const theme of [`soda-${scheme}`, 'soda-auto']) {
      assert.deepEqual(await visit(page, theme), baseline, `${theme} must preserve native diff, error and ANSI colors`);
      const body = await colors(page.locator('body'));
      assert.equal(body.background, scheme === 'light' ? 'rgb(255, 253, 248)' : 'rgb(12, 16, 23)');
      assert(contrast(body.color, body.background) >= 4.5);
      await checkButtons(page);
      await checkFocusAndImages(page);
      await page.screenshot({ path: path.join(evidence, `${theme}-${scheme}.png`), fullPage: true });
    }
    // Automatic mode responds to a preference change without a page reload.
    await page.emulateMedia({ colorScheme: scheme === 'light' ? 'dark' : 'light' });
    assert.equal((await colors(page.locator('body'))).background,
      scheme === 'light' ? 'rgb(12, 16, 23)' : 'rgb(255, 253, 248)');
    // Explicit user choice must not follow the opposite OS preference.
    await visit(page, `soda-${scheme}`);
    assert.equal((await colors(page.locator('body'))).background,
      scheme === 'light' ? 'rgb(255, 253, 248)' : 'rgb(12, 16, 23)');
    assert.deepEqual(errors, []);
    await context.close();
  }
  for (const [width, scale] of [[320, 1], [390, 1], [1280, 2]]) {
    const context = await browser.newContext({ viewport: { width, height: 1000 }, deviceScaleFactor: scale });
    const page = await context.newPage();
    for (const mode of ['light', 'dark']) {
      await visit(page, `soda-${mode}`);
      await checkFocusAndImages(page);
      await page.screenshot({ path: path.join(evidence, `soda-${mode}-${width}-${scale}x.png`), fullPage: true });
    }
    await context.close();
  }
  console.log('Forgejo theme checks passed: native CSS, control states/contrast, semantic colors, focus, automatic/explicit modes, mobile and 2x.');
} finally {
  await browser.close();
}
