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
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Serve static dashboard
app.use(express.static(path.join(__dirname, 'public')));

// API routes
app.use('/api', apiRoutes);
app.use('/api/hooks', hookRoutes);

// Tunnel URL endpoint
let tunnelUrl = null;
app.get('/api/tunnel', (req, res) => {
  res.json({ url: tunnelUrl });
});

// Start server
app.listen(PORT, () => {
  console.log(`\n🌐 Dashboard:  http://localhost:${PORT}`);
  console.log(`📡 API:        http://localhost:${PORT}/api\n`);

  // Initialize WhatsApp client (async, won't block server)
  waClient.initialize().catch((err) => {
    console.error('⚠️  WhatsApp init failed:', err.message);
  });

  // Start Cloudflare Tunnel
  startTunnel();
});

async function startTunnel() {
  try {
    const { Tunnel } = require('cloudflared/lib/tunnel');
    const { install } = require('cloudflared/lib/install');
    const { bin } = require('cloudflared/lib/constants');

    // Auto-install cloudflared binary if missing
    if (!fs.existsSync(bin)) {
      console.log('☁️  Downloading cloudflared binary...');
      await install(bin);
      console.log('☁️  cloudflared installed.');
    }

    // Check if named tunnel mode is requested
    const useNamed = process.argv.includes('--tunnel') &&
      process.argv[process.argv.indexOf('--tunnel') + 1] === 'named';

    let tunnel;

    if (useNamed) {
      // Use named tunnel from cloudflare-config.json
      const cfConfigPath = path.join(__dirname, 'cloudflare-config.json');
      if (!fs.existsSync(cfConfigPath)) {
        console.log('⚠️  No cloudflare-config.json found. Run: bun run setup:cloudflare');
        console.log('   Falling back to quick tunnel...\n');
        tunnel = Tunnel.quick(`http://localhost:${PORT}`);
      } else {
        const cfConfig = JSON.parse(fs.readFileSync(cfConfigPath, 'utf-8'));
        console.log(`☁️  Starting named tunnel for ${cfConfig.domain}...`);
        tunnel = Tunnel.withToken(cfConfig.tunnelToken);
        tunnelUrl = `https://${cfConfig.domain}`;
        console.log(`\n🌍 Domain:  https://${cfConfig.domain}\n`);
      }
    } else {
      // Quick tunnel (default)
      console.log('☁️  Starting Cloudflare Quick Tunnel...');
      tunnel = Tunnel.quick(`http://localhost:${PORT}`);
    }

    tunnel.on('url', (url) => {
      if (!tunnelUrl) {
        tunnelUrl = url;
        console.log(`\n🌍 Tunnel URL:  ${url}\n`);
      }
    });

    tunnel.on('error', (err) => {
      console.error('☁️  Tunnel error:', err.message);
    });

    // Graceful shutdown
    process.on('SIGINT', () => {
      console.log('\n🛑 Shutting down...');
      tunnel.stop();
      process.exit(0);
    });
  } catch (err) {
    console.log('⚠️  Cloudflare Tunnel not available:', err.message);
    console.log('   The server is still accessible locally.\n');
  }
}
