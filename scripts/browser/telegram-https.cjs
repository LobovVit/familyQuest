// Invoked by TestTelegramCrossSiteHTTPSBrowser. Every value received on stdin is
// synthetic. No persistent browser profile, production URL, cookie or credential.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const config = JSON.parse(fs.readFileSync(0, 'utf8'));
  for (const [key, host] of Object.entries({ origin: 'product.test', containerOrigin: 'container.test', otherContainerOrigin: 'other-container.test' })) {
    const url = new URL(config[key]);
    assert.equal(url.protocol, 'https:');
    assert.equal(url.hostname, host);
    assert.ok(url.port);
  }
  // Playwright disables this feature by default. Restore it, preserving all
  // other defaults. Pin Playwright and fail if its argument layout changes.
  const bundle = fs.readFileSync(path.join(path.dirname(require.resolve('playwright-core/package.json')), 'lib/coreBundle.js'), 'utf8');
  const declaration = bundle.match(/disabledFeatures = \[([\s\S]*?)\]\.filter\(Boolean\)/);
  assert.ok(declaration, 'Playwright Chromium defaults changed; review the harness');
  const features = [...declaration[1].matchAll(/^\s*"([^"]+)"/gm)].map(match => match[1]);
  assert.ok(features.includes('ThirdPartyStoragePartitioning'));
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.FAMILYQUEST_BROWSER_EXECUTABLE || undefined,
    ignoreDefaultArgs: ['--disable-features=' + features.join(',')],
    args: [
      '--disable-features=' + features.filter(feature => feature !== 'ThirdPartyStoragePartitioning').join(','),
      '--enable-features=ThirdPartyStoragePartitioning,ForceThirdPartyCookieBlocking',
      '--test-third-party-cookie-phaseout', '--no-proxy-server',
      '--host-resolver-rules=EXCLUDE 127.0.0.1,MAP product.test 127.0.0.1,MAP container.test 127.0.0.1,MAP other-container.test 127.0.0.1,MAP * ~NOTFOUND',
    ],
  });
  const report = {
    scope: 'isolated_familyquest_access_core_https_browser',
    browser: browser.version(), checkedAt: new Date().toISOString(),
    real: ['Chromium cookie policy', 'loopback HTTPS', 'FamilyQuest Access', 'Core API', 'disposable PostgreSQL'],
    fixtures: ['HTML shell', 'Portal HMAC/replay/session boundary', 'OIDC authorization server'],
    thirdPartyCookiesBlocked: true, productionUsed: false, checks: [], passed: false,
  };
  const record = (name, details = {}) => report.checks.push({ name, passed: true, ...details });
  const request = (surface, route, { body = {}, token, telegram = true, negative = false, method = 'POST' } = {}) => surface.evaluate(async ({ route, body, token, telegram, negative, method }) => {
    const headers = { 'Content-Type': 'application/json', 'X-FamilyQuest': '1' };
    if (telegram) headers['X-FamilyQuest-Client'] = 'telegram';
    if (negative) headers['X-Fixture-Without-Partitioned'] = '1';
    if (token) headers.Authorization = 'Bearer ' + token;
    const response = await fetch(route, { method, headers, credentials: 'same-origin', ...(method === 'GET' ? {} : { body: JSON.stringify(body) }) });
    return { status: response.status, body: await response.json() };
  }, { route, body, token, telegram, negative, method });
  const expectStatus = (result, expected, label) => {
    assert.equal(result.status, expected, label);
    return result.body;
  };
  const restrict = async (context, target, blocked = []) => {
    let cdp;
    try {
      cdp = await context.newCDPSession(target);
    } catch (error) {
      // Bundled Headless Shell may share the parent's CDP target. Page controls
      // are already installed; the observed negative control remains mandatory.
      if (target.parentFrame?.() && error.message.includes("part of the parent frame's session")) return;
      throw error;
    }
    await cdp.send('Network.enable');
    cdp.on('Network.responseReceivedExtraInfo', event => {
      for (const cookie of event.blockedCookies || []) blocked.push(...cookie.blockedReasons);
    });
    await cdp.send('Network.setCookieControls', { enableThirdPartyCookieRestriction: true });
  };
  const embed = async (context, page, origin, blocked = []) => {
    await restrict(context, page, blocked);
    await page.goto(origin);
    const frame = page.frames().find(frame => frame.url() === config.origin + '/telegram');
    assert.ok(frame, 'cross-site product iframe loaded from real HTTPS server');
    await restrict(context, frame, blocked);
    return frame;
  };
  try {
    // Negative control: same real Access exchange and same browser restrictions;
    // a test-only server writer removes ONLY Partitioned from Set-Cookie.
    const negative = await browser.newContext({ ignoreHTTPSErrors: true, serviceWorkers: 'block' });
    const blocked = [];
    const negativePage = await negative.newPage();
    const negativeFrame = await embed(negative, negativePage, config.containerOrigin, blocked);
    expectStatus(await request(negativeFrame, '/api/telegram/exchange', { body: { initData: config.negativeInitData }, negative: true }), 200, 'unpartitioned exchange');
    expectStatus(await request(negativeFrame, '/api/telegram/session'), 401, 'unpartitioned session blocked by browser');
    assert.ok(blocked.length > 0, 'Chromium must report a blocked Set-Cookie');
    record('negative_control_without_partitioned', { exchange: 200, session: 401, blockedReasons: [...new Set(blocked)] });
    await negative.close();

    const context = await browser.newContext({ ignoreHTTPSErrors: true, serviceWorkers: 'block' });
    const page = await context.newPage();
    await restrict(context, page);
    await page.goto(config.origin + '/api/account/authorize');
    assert.equal(page.url(), config.origin + '/?sso=complete', 'real OIDC redirects completed');
    expectStatus(await request(page, '/api/account/exchange', { telegram: false }), 200, 'OIDC handoff');
    const ssoBefore = (await context.cookies()).find(cookie => cookie.name === '__Host-familyquest-sso');
    assert.ok(ssoBefore && ssoBefore.secure && ssoBefore.httpOnly);
    assert.equal(ssoBefore.sameSite, 'Lax');
    assert.equal(ssoBefore.partitionKey, undefined);
    const normal = expectStatus(await request(page, '/api/account/profile', { telegram: false, body: { participantId: config.participantId, pin: '739281' } }), 200, 'ordinary OIDC profile PIN');
    assert.ok(normal.token);
    expectStatus(await request(page, '/api/participants', { telegram: false, token: normal.token, method: 'GET' }), 200, 'ordinary API before Telegram');
    record('ordinary_oidc_signed_pkce_pin_api', { sameSite: 'Lax', partitioned: false });

    let frame = await embed(context, page, config.containerOrigin);
    const tampered = new URLSearchParams(config.positiveInitData);
    tampered.set('user', '{"id":999999,"first_name":"Tampered"}');
    expectStatus(await request(frame, '/api/telegram/exchange', { body: { initData: tampered.toString() } }), 401, 'invalid signed initData');
    expectStatus(await request(frame, '/api/telegram/session'), 401, 'failed exchange creates no session');
    record('invalid_signature_rejected', { exchange: 401, session: 401 });

    const exchange = expectStatus(await request(frame, '/api/telegram/exchange', { body: { initData: config.positiveInitData } }), 200, 'signed exchange');
    assert.equal(exchange.linked, true);
    assert.equal(exchange.families[0].id, config.familyId);
    const telegramCookie = (await context.cookies()).find(cookie => cookie.name === 'familyquest_telegram');
    assert.ok(telegramCookie && telegramCookie.secure && telegramCookie.httpOnly);
    assert.equal(telegramCookie.sameSite, 'None');
    assert.equal(telegramCookie.path, '/api');
    assert.equal(telegramCookie.partitionKey, 'https://container.test');
    expectStatus(await request(frame, '/api/telegram/exchange', { body: { initData: config.positiveInitData } }), 409, 'signed exchange replay');
    expectStatus(await request(frame, '/api/telegram/session'), 200, 'replay does not remove existing session');
    record('signed_exchange_and_replay', { exchange: 200, replay: 409, sameSite: 'None', partitioned: true });

    expectStatus(await request(frame, '/api/telegram/family', { body: { familyId: config.familyId } }), 200, 'choose family');
    const profile = expectStatus(await request(frame, '/api/telegram/profile', { body: { participantId: config.participantId, pin: '739281' } }), 200, 'Telegram profile PIN');
    assert.ok(profile.token);
    expectStatus(await request(frame, '/api/participants', { token: profile.token, method: 'GET' }), 200, 'protected Core API');
    record('family_pin_and_real_core_api');

    await page.reload();
    frame = page.frames().find(frame => frame.url() === config.origin + '/telegram');
    await restrict(context, frame);
    expectStatus(await request(frame, '/api/telegram/session'), 200, 'Telegram session after reload');
    expectStatus(await request(frame, '/api/participants', { token: profile.token, method: 'GET' }), 200, 'Core API after reload');
    record('reload_retains_session_and_api_access');

    frame = await embed(context, page, config.otherContainerOrigin);
    expectStatus(await request(frame, '/api/telegram/session'), 401, 'another top-level site cannot use partition');
    expectStatus(await request(frame, '/api/participants', { token: profile.token, method: 'GET' }), 401, 'JWT alone cannot cross partitions');
    frame = await embed(context, page, config.containerOrigin);
    expectStatus(await request(frame, '/api/telegram/session'), 200, 'original partition remains usable');
    record('partition_isolation');

    expectStatus(await request(frame, '/api/session/logout', { token: profile.token }), 200, 'actual client logout route');
    expectStatus(await request(frame, '/api/telegram/session'), 401, 'Telegram session revoked');
    expectStatus(await request(frame, '/api/participants', { token: profile.token, method: 'GET' }), 401, 'old JWT denied after logout');
    assert.ok(!(await context.cookies()).some(cookie => cookie.name === 'familyquest_telegram'), 'partitioned cookie deleted');
    const ssoAfter = (await context.cookies()).find(cookie => cookie.name === '__Host-familyquest-sso');
    assert.ok(ssoAfter && ssoBefore.value === ssoAfter.value, 'Telegram logout preserves ordinary OIDC cookie');
    assert.equal(ssoAfter.sameSite, 'Lax');
    assert.equal(ssoAfter.partitionKey, undefined);
    record('telegram_logout_revokes_session_and_api');

    await page.goto(config.origin);
    expectStatus(await request(page, '/api/account/restore', { telegram: false }), 200, 'ordinary OIDC cookie still works');
    expectStatus(await request(page, '/api/participants', { token: normal.token, method: 'GET', telegram: false }), 200, 'ordinary API still works');
    record('ordinary_oidc_and_api_unaffected_by_telegram_logout');
    await context.close();
    report.passed = true;
    process.stdout.write(JSON.stringify(report) + '\n');
  } finally {
    await browser.close();
  }
})().catch(error => {
  // Assertions contain only fixed labels/statuses; never dump responses/cookies.
  console.error(error.name + ': ' + error.message);
  process.exitCode = 1;
});
