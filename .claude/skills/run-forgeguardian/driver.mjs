#!/usr/bin/env node
// Playwright driver for the ForgeGuardian dashboard + API.
//
// Usage:
//   node .claude/skills/run-forgeguardian/driver.mjs nav /welcome
//   node .claude/skills/run-forgeguardian/driver.mjs nav /scan --login
//   node .claude/skills/run-forgeguardian/driver.mjs click "Scan Now"
//
// Commands (space-separated args, run one at a time — each invocation
// launches a fresh browser):
//   nav <path> [--login] [--mobile|--tablet]
//       Navigate to http://localhost:3000<path>, wait for network idle,
//       screenshot to /tmp/fg-driver-shot.png, print console errors.
//       --login: fill+submit the login form first (admin@test.com /
//                testpass123 — see SKILL.md prerequisites) and skip
//                onboarding before navigating.
//       --mobile / --tablet: set viewport to 390x844 / 768x1024 instead
//                of the 1440x950 default.
//   click <text> [<path>] [--login]
//       Navigate to <path> (default "/"), click the first button
//       containing <text>, wait, screenshot, print console errors.
//
// Requires: playwright installed somewhere resolvable (see SKILL.md —
// this project's dashboard/node_modules does NOT include it; install
// separately, e.g. `npm install playwright` in /tmp and run this driver
// with `NODE_PATH=/tmp/node_modules node driver.mjs ...`).

import { chromium } from 'playwright';

const [cmd, ...rest] = process.argv.slice(2);
const flags = new Set(rest.filter(a => a.startsWith('--')));
const args = rest.filter(a => !a.startsWith('--'));

const viewport = flags.has('--mobile') ? { width: 390, height: 844 }
  : flags.has('--tablet') ? { width: 768, height: 1024 }
  : { width: 1440, height: 950 };

const BASE = 'http://localhost:3000';
const SHOT = '/tmp/fg-driver-shot.png';

async function maybeLogin(page) {
  if (!flags.has('--login')) return;
  await page.goto(BASE + '/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(500);
  const emailInput = page.locator('input[type="email"]');
  if (await emailInput.count() === 0) return; // auth disabled — nothing to do
  await emailInput.fill('admin@test.com');
  await page.fill('input[type="password"]', 'testpass123');
  await page.click('button[type="submit"]');
  await page.waitForTimeout(1000);
  await page.evaluate(() => localStorage.setItem('fg_onboarded', 'true'));
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForTimeout(500);
}

const browser = await chromium.launch();
const page = await browser.newPage({ viewport });
const errors = [];
page.on('pageerror', e => errors.push('pageerror: ' + e.message));
page.on('console', msg => {
  if (msg.type() === 'error') errors.push('console: ' + msg.text());
});

try {
  if (cmd === 'nav') {
    const path = args[0] ?? '/';
    await maybeLogin(page);
    await page.goto(BASE + path, { waitUntil: 'networkidle' });
    await page.waitForTimeout(600);
    await page.screenshot({ path: SHOT, fullPage: true });
    console.log('navigated to', path, '| screenshot:', SHOT);
  } else if (cmd === 'click') {
    const text = args[0];
    const path = args[1] ?? '/';
    await maybeLogin(page);
    await page.goto(BASE + path, { waitUntil: 'networkidle' });
    await page.waitForTimeout(500);
    await page.click(`button:has-text("${text}")`);
    await page.waitForTimeout(700);
    await page.screenshot({ path: SHOT, fullPage: true });
    console.log('clicked', JSON.stringify(text), '| screenshot:', SHOT);
  } else {
    console.error('unknown command:', cmd, '\nSee header comment for usage.');
    process.exitCode = 1;
  }
} finally {
  console.log('console/page errors:', errors.length ? JSON.stringify(errors, null, 2) : 'none');
  await browser.close();
}
