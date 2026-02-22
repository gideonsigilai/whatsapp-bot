const express = require('express');
const bodyParser = require('body-parser');
const path = require('path');
const fs = require('fs');
const waClient = require('./wa-client');
const apiRoutes = require('./routes/api');
const hookRoutes = require('./routes/hooks');
const db = require('./db');

const app = express();
const PORT = db.get('config')?.port || 3000;

// Middleware
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  if (req.method === 'OPTIONS') return res.sendStatus(204);
  next();
});
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Serve static dashboard
app.use(express.static(path.join(__dirname, 'public')));

// API routes
app.use('/api', apiRoutes);
app.use('/api/hooks', hookRoutes);

// ── Tunnel State ──
let tunnelUrl = null;
let tunnelProcess = null;
let tunnelStarting = false;

app.get('/api/tunnel', (req, res) => {
  const config = db.get('config') || {};
  res.json({
    url: tunnelUrl,
    enabled: !!config.tunnelEnabled,
    starting: tunnelStarting,
  });
});

app.post('/api/tunnel/toggle', async (req, res) => {
  const config = db.get('config') || {};
  const enable = !config.tunnelEnabled;

  // Persist the new state
  config.tunnelEnabled = enable;
  db.set('config', config);

  if (enable) {
    if (!tunnelProcess && !tunnelStarting) {
      startTunnel();
    }
    res.json({ success: true, enabled: true, message: 'Tunnel starting...' });
  } else {
    stopTunnel();
    res.json({ success: true, enabled: false, message: 'Tunnel stopped' });
  }
});

// Start server
app.listen(PORT, () => {
  console.log(`\n🌐 Dashboard:  http://localhost:${PORT}`);
  console.log(`📡 API:        http://localhost:${PORT}/api\n`);

  // Initialize WhatsApp client (async, won't block server)
  waClient.initialize().catch((err) => {
    console.error('⚠️  WhatsApp init failed:', err.message);
  });

  // Only start tunnel if enabled in config
  const config = db.get('config') || {};
  if (config.tunnelEnabled) {
    startTunnel();
  } else {
    console.log('☁️  Cloudflare Tunnel is disabled. Enable via the dashboard toggle.\n');
  }
});

async function ensureBinary() {
  const { install } = require('cloudflared/lib/install');
  const { bin } = require('cloudflared/lib/constants');
  if (!fs.existsSync(bin)) {
    console.log('☁️  Downloading cloudflared binary...');
    await install(bin);
    console.log('☁️  cloudflared installed.');
  }
}

async function startTunnel() {
  if (tunnelProcess || tunnelStarting) return;
  tunnelStarting = true;

  try {
    const { Tunnel } = require('cloudflared/lib/tunnel');
    await ensureBinary();

    // Check if named tunnel mode is requested
    const useNamed = process.argv.includes('--tunnel') &&
      process.argv[process.argv.indexOf('--tunnel') + 1] === 'named';

    if (useNamed) {
      const cfConfigPath = path.join(__dirname, 'cloudflare-config.json');
      if (!fs.existsSync(cfConfigPath)) {
        console.log('⚠️  No cloudflare-config.json found. Run: bun run setup:cloudflare');
        console.log('   Falling back to quick tunnel...\n');
        tunnelProcess = Tunnel.quick(`http://localhost:${PORT}`);
      } else {
        const cfConfig = JSON.parse(fs.readFileSync(cfConfigPath, 'utf-8'));
        console.log(`☁️  Starting named tunnel for ${cfConfig.domain}...`);
        tunnelProcess = Tunnel.withToken(cfConfig.tunnelToken);
        tunnelUrl = `https://${cfConfig.domain}`;
        console.log(`\n🌍 Domain:  https://${cfConfig.domain}\n`);
      }
    } else {
      console.log('☁️  Starting Cloudflare Quick Tunnel...');
      tunnelProcess = Tunnel.quick(`http://localhost:${PORT}`);
    }

    tunnelProcess.on('url', (url) => {
      if (!tunnelUrl) {
        tunnelUrl = url;
        console.log(`\n🌍 Tunnel URL:  ${url}\n`);
      }
    });

    tunnelProcess.on('error', (err) => {
      console.error('☁️  Tunnel error:', err.message);
    });

    tunnelStarting = false;
  } catch (err) {
    tunnelStarting = false;
    console.log('⚠️  Cloudflare Tunnel not available:', err.message);
    console.log('   The server is still accessible locally.\n');
  }
}

function stopTunnel() {
  if (tunnelProcess) {
    try { tunnelProcess.stop(); } catch {}
    tunnelProcess = null;
    tunnelUrl = null;
    console.log('☁️  Tunnel stopped.');
  }
}

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down...');
  stopTunnel();
  process.exit(0);
});
