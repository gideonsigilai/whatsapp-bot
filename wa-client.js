const { Client, LocalAuth } = require('whatsapp-web.js');
const qrcode = require('qrcode-terminal');
const QRCode = require('qrcode');
const db = require('./db');
const fetch = require('node-fetch');
const fs = require('fs');
const { execSync } = require('child_process');

/**
 * Find system-installed Chrome/Chromium executable.
 * Priority: env PUPPETEER_EXECUTABLE_PATH > common paths > `which chromium`
 */
function findChrome() {
  // 1. Explicit env var (set by nixpacks.toml on Railway)
  if (process.env.PUPPETEER_EXECUTABLE_PATH) {
    return process.env.PUPPETEER_EXECUTABLE_PATH;
  }

  // 2. Common static paths
  const paths = process.platform === 'win32'
    ? [
        process.env['PROGRAMFILES'] + '\\Google\\Chrome\\Application\\chrome.exe',
        process.env['PROGRAMFILES(X86)'] + '\\Google\\Chrome\\Application\\chrome.exe',
        process.env['LOCALAPPDATA'] + '\\Google\\Chrome\\Application\\chrome.exe',
      ]
    : process.platform === 'darwin'
    ? [
        '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
        '/Applications/Chromium.app/Contents/MacOS/Chromium',
      ]
    : [
        '/usr/bin/google-chrome',
        '/usr/bin/chromium-browser',
        '/usr/bin/chromium',
        '/usr/local/bin/chromium',
        '/nix/var/nix/profiles/default/bin/chromium',
      ];

  for (const p of paths) {
    try { if (fs.existsSync(p)) return p; } catch {}
  }

  // 3. Dynamic lookup via `which`
  try {
    const found = execSync('which chromium || which chromium-browser || which google-chrome', { encoding: 'utf8' }).trim().split('\n')[0];
    if (found) return found;
  } catch {}

  return undefined; // fall back to Puppeteer bundled Chrome
}

let client = null;
let qrCodeData = null;
let connectionStatus = 'disconnected'; // disconnected | qr | ready
let clientInfo = null;

function getStatus() {
  return {
    status: connectionStatus,
    qr: qrCodeData,
    info: clientInfo,
  };
}

async function initialize() {
  if (client) {
    try { await client.destroy(); } catch {}
    client = null;
  }

  connectionStatus = 'initializing';
  qrCodeData = null;
  clientInfo = null;
  client = new Client({
    authStrategy: new LocalAuth(),
    puppeteer: {
      headless: true,
      executablePath: findChrome(),
      args: [
        '--no-sandbox',
        '--disable-setuid-sandbox',
        '--disable-dev-shm-usage',
        '--disable-gpu',
      ],
    },
  });

  client.on('qr', async (qr) => {
    connectionStatus = 'qr';
    qrcode.generate(qr, { small: true });
    try {
      qrCodeData = await QRCode.toDataURL(qr);
    } catch {
      qrCodeData = null;
    }
    console.log('📱 Scan the QR code to connect WhatsApp');
  });

  client.on('ready', () => {
    connectionStatus = 'ready';
    qrCodeData = null;
    clientInfo = {
      pushname: client.info?.pushname || 'Unknown',
      phone: client.info?.wid?.user || 'Unknown',
      platform: client.info?.platform || 'Unknown',
    };
    console.log(`✅ WhatsApp connected as ${clientInfo.pushname} (${clientInfo.phone})`);
  });

  client.on('disconnected', (reason) => {
    connectionStatus = 'disconnected';
    qrCodeData = null;
    clientInfo = null;
    db.clearBotData();
    console.log('❌ WhatsApp disconnected:', reason);
  });

  client.on('message', async (msg) => {
    const contact = await msg.getContact();
    const chat = await msg.getChat();

    const messageData = {
      id: msg.id._serialized,
      from: msg.from,
      to: msg.to,
      body: msg.body,
      timestamp: new Date().toISOString(),
      type: 'received',
      contactName: contact?.pushname || contact?.name || msg.from,
      isGroup: chat.isGroup,
      groupName: chat.isGroup ? chat.name : null,
    };

    db.pushTo('messages', messageData);
    db.incrementStat('messagesReceived');

    // Fire webhooks
    const webhooks = db.get('webhooks') || [];
    for (const hook of webhooks) {
      try {
        await fetch(hook.url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(messageData),
        });
      } catch (err) {
        console.error(`Webhook failed (${hook.url}):`, err.message);
      }
    }
  });

  console.log('🚀 Initializing WhatsApp client...');

  try {
    await client.initialize();
  } catch (err) {
    connectionStatus = 'disconnected';
    console.error('⚠️  WhatsApp client initialization failed:', err.message);
    console.error('   The dashboard is still accessible. Fix the issue and restart.');
  }
}

async function sendMessage(number, message) {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = number.includes('@c.us') ? number : `${number}@c.us`;
  const result = await client.sendMessage(chatId, message);

  const messageData = {
    id: result.id._serialized,
    from: 'me',
    to: chatId,
    body: message,
    timestamp: new Date().toISOString(),
    type: 'sent',
    contactName: number,
    isGroup: false,
    groupName: null,
  };

  db.pushTo('messages', messageData);
  db.incrementStat('messagesSent');

  return messageData;
}

async function sendGroupMessage(groupId, message) {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const result = await client.sendMessage(chatId, message);

  const messageData = {
    id: result.id._serialized,
    from: 'me',
    to: chatId,
    body: message,
    timestamp: new Date().toISOString(),
    type: 'sent',
    contactName: 'Group',
    isGroup: true,
    groupName: groupId,
  };

  db.pushTo('messages', messageData);
  db.incrementStat('messagesSent');

  return messageData;
}

async function getGroups() {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chats = await client.getChats();
  return chats
    .filter((c) => c.isGroup)
    .map((c) => ({
      id: c.id._serialized,
      name: c.name,
      participantCount: c.participants?.length || 0,
      isReadOnly: c.isReadOnly,
    }));
}

async function joinGroup(inviteCode) {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  // Extract invite code from URL if full URL is provided
  const code = inviteCode.replace('https://chat.whatsapp.com/', '');
  const result = await client.acceptInvite(code);
  db.incrementStat('groupsJoined');
  return result;
}

async function leaveGroup(groupId) {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const chat = await client.getChatById(chatId);
  if (!chat.isGroup) throw new Error('Chat is not a group');
  await chat.leave();
  db.incrementStat('groupsLeft');
  return { success: true, groupId: chatId };
}

async function addToGroup(groupId, participants) {
  if (!client || connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const chat = await client.getChatById(chatId);
  if (!chat.isGroup) throw new Error('Chat is not a group');

  const participantIds = participants.map((p) =>
    p.includes('@c.us') ? p : `${p}@c.us`
  );

  const result = await chat.addParticipants(participantIds);
  return result;
}

async function disconnect() {
  if (!client) throw new Error('No active WhatsApp session');
  connectionStatus = 'disconnected';
  qrCodeData = null;
  clientInfo = null;
  db.clearBotData();
  try {
    await client.logout();
  } catch {}
  try {
    await client.destroy();
  } catch {}
  client = null;
  console.log('🔌 WhatsApp disconnected by user');
}

async function reconnect() {
  console.log('🔄 Reconnecting WhatsApp...');
  await initialize();
}

module.exports = {
  initialize,
  disconnect,
  reconnect,
  getStatus,
  sendMessage,
  sendGroupMessage,
  getGroups,
  joinGroup,
  leaveGroup,
  addToGroup,
};
