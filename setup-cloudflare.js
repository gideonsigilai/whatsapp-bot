#!/usr/bin/env node

/**
 * Cloudflare Domain Setup Script
 *
 * This script configures a custom Cloudflare domain for the WA Bot Server.
 * It uses Cloudflare's API to create a named tunnel and DNS route.
 *
 * Prerequisites:
 *   1. A Cloudflare account with a domain added
 *   2. A Cloudflare API Token with the following permissions:
 *      - Account: Cloudflare Tunnel (Edit)
 *      - Zone: DNS (Edit)
 *   3. Your Account ID (found on the Cloudflare dashboard overview page)
 *
 * Usage:
 *   bun run setup-cloudflare.js
 *
 * Or with arguments:
 *   bun run setup-cloudflare.js --token YOUR_API_TOKEN --account YOUR_ACCOUNT_ID --domain bot.yourdomain.com
 */

const fs = require('fs');
const path = require('path');
const readline = require('readline');

const CONFIG_FILE = path.join(__dirname, 'cloudflare-config.json');
const CLOUDFLARE_API = 'https://api.cloudflare.com/client/v4';

// ── Helpers ──

function loadConfig() {
  try {
    if (fs.existsSync(CONFIG_FILE)) {
      return JSON.parse(fs.readFileSync(CONFIG_FILE, 'utf-8'));
    }
  } catch {}
  return {};
}

function saveConfig(config) {
  fs.writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2), 'utf-8');
}

function ask(question) {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer.trim());
    });
  });
}

async function cfFetch(endpoint, token, options = {}) {
  const res = await fetch(`${CLOUDFLARE_API}${endpoint}`, {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    ...options,
  });
  const data = await res.json();
  if (!data.success) {
    const errors = data.errors?.map((e) => e.message).join(', ') || 'Unknown error';
    throw new Error(`Cloudflare API error: ${errors}`);
  }
  return data;
}

function parseArgs() {
  const args = {};
  const argv = process.argv.slice(2);
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i].replace(/^--/, '');
    args[key] = argv[i + 1];
  }
  return args;
}

// ── Main ──

async function main() {
  console.log('\n┌───────────────────────────────────────────┐');
  console.log('│   ☁️  Cloudflare Domain Setup for WA Bot   │');
  console.log('└───────────────────────────────────────────┘\n');

  const args = parseArgs();
  const config = loadConfig();

  // Step 1: Gather credentials
  const token =
    args.token ||
    config.apiToken ||
    (await ask('🔑 Cloudflare API Token: '));

  const accountId =
    args.account ||
    config.accountId ||
    (await ask('🆔 Cloudflare Account ID: '));

  const domain =
    args.domain ||
    config.domain ||
    (await ask('🌐 Custom domain (e.g. bot.yourdomain.com): '));

  const localPort = args.port || config.port || '3000';

  // Validate token
  console.log('\n⏳ Verifying API token...');
  try {
    await cfFetch('/user/tokens/verify', token);
    console.log('✅ API token is valid.\n');
  } catch (err) {
    console.error('❌ Invalid API token:', err.message);
    process.exit(1);
  }

  // Step 2: Create a named tunnel
  const tunnelName = `wa-bot-${domain.replace(/\./g, '-')}`;
  console.log(`⏳ Creating tunnel "${tunnelName}"...`);

  let tunnelId;
  let tunnelToken;

  try {
    // Check if tunnel already exists
    const existingTunnels = await cfFetch(
      `/accounts/${accountId}/cfd_tunnel?name=${encodeURIComponent(tunnelName)}&is_deleted=false`,
      token
    );

    if (existingTunnels.result?.length > 0) {
      tunnelId = existingTunnels.result[0].id;
      console.log(`✅ Tunnel already exists: ${tunnelId}`);

      // Get a fresh token for the existing tunnel
      const tokenRes = await cfFetch(
        `/accounts/${accountId}/cfd_tunnel/${tunnelId}/token`,
        token
      );
      tunnelToken = tokenRes.result;
    } else {
      // Generate a tunnel secret
      const secret = Buffer.from(
        Array.from({ length: 32 }, () => Math.floor(Math.random() * 256))
      ).toString('base64');

      const createRes = await cfFetch(
        `/accounts/${accountId}/cfd_tunnel`,
        token,
        {
          method: 'POST',
          body: JSON.stringify({
            name: tunnelName,
            tunnel_secret: secret,
            config_src: 'cloudflare',
          }),
        }
      );

      tunnelId = createRes.result.id;
      console.log(`✅ Tunnel created: ${tunnelId}`);

      // Get tunnel token
      const tokenRes = await cfFetch(
        `/accounts/${accountId}/cfd_tunnel/${tunnelId}/token`,
        token
      );
      tunnelToken = tokenRes.result;
    }
  } catch (err) {
    console.error('❌ Failed to create tunnel:', err.message);
    process.exit(1);
  }

  // Step 3: Configure tunnel ingress
  console.log('⏳ Configuring tunnel ingress rules...');
  try {
    await cfFetch(
      `/accounts/${accountId}/cfd_tunnel/${tunnelId}/configurations`,
      token,
      {
        method: 'PUT',
        body: JSON.stringify({
          config: {
            ingress: [
              {
                hostname: domain,
                service: `http://localhost:${localPort}`,
              },
              {
                service: 'http_status:404',
              },
            ],
          },
        }),
      }
    );
    console.log('✅ Ingress rules configured.\n');
  } catch (err) {
    console.error('❌ Failed to configure ingress:', err.message);
    process.exit(1);
  }

  // Step 4: Create DNS CNAME Record
  console.log('⏳ Setting up DNS...');
  const zoneDomain = domain.split('.').slice(-2).join('.');
  const subdomain = domain.replace(`.${zoneDomain}`, '') || '@';

  try {
    // Get Zone ID
    const zoneRes = await cfFetch(`/zones?name=${zoneDomain}`, token);
    if (!zoneRes.result?.length) {
      throw new Error(`Zone "${zoneDomain}" not found in your Cloudflare account.`);
    }
    const zoneId = zoneRes.result[0].id;

    // Check if CNAME already exists
    const dnsRes = await cfFetch(
      `/zones/${zoneId}/dns_records?type=CNAME&name=${domain}`,
      token
    );

    const cnameTarget = `${tunnelId}.cfargotunnel.com`;

    if (dnsRes.result?.length > 0) {
      // Update existing record
      await cfFetch(`/zones/${zoneId}/dns_records/${dnsRes.result[0].id}`, token, {
        method: 'PUT',
        body: JSON.stringify({
          type: 'CNAME',
          name: subdomain,
          content: cnameTarget,
          proxied: true,
        }),
      });
      console.log(`✅ DNS record updated: ${domain} → ${cnameTarget}`);
    } else {
      // Create new record
      await cfFetch(`/zones/${zoneId}/dns_records`, token, {
        method: 'POST',
        body: JSON.stringify({
          type: 'CNAME',
          name: subdomain,
          content: cnameTarget,
          proxied: true,
        }),
      });
      console.log(`✅ DNS record created: ${domain} → ${cnameTarget}`);
    }
  } catch (err) {
    console.error('❌ Failed to create DNS record:', err.message);
    console.log('   You may need to manually add a CNAME record:');
    console.log(`   ${domain} → ${tunnelId}.cfargotunnel.com\n`);
  }

  // Step 5: Save config
  const finalConfig = {
    apiToken: token,
    accountId,
    domain,
    port: localPort,
    tunnelId,
    tunnelName,
    tunnelToken,
  };
  saveConfig(finalConfig);

  // Step 6: Update index.js instructions
  console.log('\n┌───────────────────────────────────────────┐');
  console.log('│   ✅  Setup Complete!                      │');
  console.log('└───────────────────────────────────────────┘\n');
  console.log(`🌐 Your domain: https://${domain}`);
  console.log(`🔧 Tunnel ID:   ${tunnelId}`);
  console.log(`📄 Config saved: ${CONFIG_FILE}\n`);
  console.log('To start the server with your custom domain, run:\n');
  console.log('  bun run start:tunnel\n');
  console.log('This will start both the Express server and the Cloudflare tunnel.\n');
}

main().catch((err) => {
  console.error('💥 Unexpected error:', err);
  process.exit(1);
});
