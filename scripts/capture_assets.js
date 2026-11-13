const { spawn, execSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const http = require('http');
const puppeteer = require('puppeteer');

const PORT = 8097;
const BASE_URL = `http://127.0.0.1:${PORT}`;
const IMAGES_DIR = path.resolve(__dirname, '../docs/images');

function wait(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function checkServerReady() {
  return new Promise((resolve) => {
    const check = () => {
      const req = http.get(`${BASE_URL}/v1/prompts`, (res) => {
        if (res.statusCode === 200) {
          resolve(true);
        } else {
          setTimeout(check, 250);
        }
      });
      req.on('error', () => {
        setTimeout(check, 250);
      });
      req.end();
    };
    check();
  });
}

function postJson(endpoint, data) {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify(data);
    const req = http.request(`${BASE_URL}${endpoint}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
      },
    }, (res) => {
      let body = '';
      res.on('data', chunk => body += chunk);
      res.on('end', () => resolve(body));
    });
    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

async function main() {
  console.log('=== promptledger Authentic Responsive Asset Generator ===');

  if (!fs.existsSync(IMAGES_DIR)) {
    fs.mkdirSync(IMAGES_DIR, { recursive: true });
  }

  const tmpDataDir = path.resolve(__dirname, '../.tmp_capture_data_' + Date.now());
  fs.mkdirSync(tmpDataDir, { recursive: true });

  console.log('1. Starting promptledger daemon on port ' + PORT + '...');
  const daemon = spawn(path.resolve(__dirname, '../bin/promptledger.exe'), [
    'serve',
    '--prompts', path.resolve(__dirname, '../prompts'),
    '--data', tmpDataDir,
    '--port', String(PORT),
  ], {
    cwd: path.resolve(__dirname, '..'),
    stdio: 'ignore',
  });

  try {
    await checkServerReady();
    console.log('✓ Daemon is live at ' + BASE_URL);

    console.log('2. Seeding traffic, cache hits, golden benchmark evals, and audit records...');
    await postJson('/v1/simulate-traffic', { count: 35 });
    await wait(1000);

    // Cache seeding
    const cacheQuery = {
      user_id: 'cache_probe_user',
      variables: {
        UserInput: 'Do you offer international shipping to Germany?',
        Context: 'Standard customer cart query'
      }
    };
    await postJson('/v1/prompts/checkout-intent-classifier/execute', cacheQuery);
    await postJson('/v1/prompts/checkout-intent-classifier/execute', cacheQuery);
    await postJson('/v1/prompts/checkout-intent-classifier/execute', cacheQuery);

    // Golden benchmark run
    await postJson('/v1/prompts/checkout-intent-classifier/eval', {
      version: 'v1.2.0',
      dataset: 'evals/checkout_intent_eval.jsonl'
    });

    // Promotion and rollback to create audit trail
    await postJson('/v1/prompts/checkout-intent-classifier/promote', { version: 'v1.3.0-canary' });
    await wait(300);
    await postJson('/v1/simulate-traffic', { count: 12 });
    await wait(1200);

    console.log('3. Launching Puppeteer browser for pixel-perfect captures...');
    const browser = await puppeteer.launch({
      headless: 'new',
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
    });

    const page = await browser.newPage();

    const tabsToCapture = [
      { tab: 'overview', filename: 'promptledger_overview.png' },
      { tab: 'registry', filename: 'promptledger_prompt_registry.png' },
      { tab: 'routing', filename: 'promptledger_traffic_routing.png' },
      { tab: 'shadow', filename: 'promptledger_shadow_inspector.png' },
      { tab: 'gates', filename: 'promptledger_safety_gates.png' },
      { tab: 'benchmarks', filename: 'promptledger_golden_evals.png' },
      { tab: 'playground', filename: 'promptledger_playground.png' },
    ];

    // Desktop Captures (1440x900)
    console.log('4. Capturing Desktop Views (1440x900)...');
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });

    for (const item of tabsToCapture) {
      const targetUrl = `${BASE_URL}/?tab=${item.tab}`;
      await page.goto(targetUrl, { waitUntil: 'networkidle0' });
      await wait(800);
      const outPath = path.join(IMAGES_DIR, item.filename);
      await page.screenshot({ path: outPath, fullPage: false });
      console.log(`✓ Captured ${item.filename} (Desktop)`);
    }

    // Tablet Capture (768x1024)
    console.log('5. Capturing Tablet View (768x1024)...');
    await page.setViewport({ width: 768, height: 1024, deviceScaleFactor: 2 });
    await page.goto(`${BASE_URL}/?tab=overview`, { waitUntil: 'networkidle0' });
    await wait(800);
    await page.screenshot({ path: path.join(IMAGES_DIR, 'promptledger_tablet.png') });
    console.log('✓ Captured promptledger_tablet.png (Tablet)');

    // Mobile Capture (390x844)
    console.log('6. Capturing Mobile View (390x844)...');
    await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
    await page.goto(`${BASE_URL}/?tab=overview`, { waitUntil: 'networkidle0' });
    await wait(800);
    await page.screenshot({ path: path.join(IMAGES_DIR, 'promptledger_mobile.png') });
    console.log('✓ Captured promptledger_mobile.png (Mobile)');

    // GIF Frames (1280x740)
    console.log('7. Capturing frames for animated demo GIF...');
    const gifFramesDir = path.resolve(__dirname, '../.tmp_gif_frames_' + Date.now());
    fs.mkdirSync(gifFramesDir, { recursive: true });

    await page.setViewport({ width: 1280, height: 740, deviceScaleFactor: 1 });
    const gifTabs = ['overview', 'registry', 'routing', 'shadow', 'gates', 'benchmarks', 'playground'];

    for (let i = 0; i < gifTabs.length; i++) {
      const gTab = gifTabs[i];
      await page.goto(`${BASE_URL}/?tab=${gTab}`, { waitUntil: 'networkidle0' });
      await wait(800);
      const framePath = path.join(gifFramesDir, `frame_${String(i).padStart(2, '0')}.png`);
      await page.screenshot({ path: framePath });
    }

    await browser.close();

    console.log('8. Compiling animated demo GIF with ffmpeg...');
    const gifOut = path.join(IMAGES_DIR, 'promptledger_demo.gif');
    const ffmpegCmd = `ffmpeg -y -framerate 0.75 -i "${path.join(gifFramesDir, 'frame_%02d.png')}" -vf "split[s0][s1];[s0]palettegen=max_colors=128:reserve_transparent=0[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3" "${gifOut}"`;
    execSync(ffmpegCmd, { stdio: 'inherit' });
    console.log('✓ Compiled ' + gifOut);

    // Clean up temporary gif frames
    fs.rmSync(gifFramesDir, { recursive: true, force: true });
  } finally {
    daemon.kill();
    fs.rmSync(tmpDataDir, { recursive: true, force: true });
  }

  console.log('=== All authentic assets generated successfully ===');
}

main().catch(err => {
  console.error('Asset generation failed:', err);
  process.exit(1);
});
