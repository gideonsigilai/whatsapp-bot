const { Client, LocalAuth } = require('whatsapp-web.js');
const qrcode = require('qrcode-terminal');
const QRCode = require('qrcode');
const db = require('./db');
const fetch = require('node-fetch');
const fs = require('fs');
const { execSync } = require('child_process');

/**
 * Find system-installed Chrome/Chromium executable.
 */
function findChrome() {
  if (process.env.PUPPETEER_EXECUTABLE_PATH) {
    return process.env.PUPPETEER_EXECUTABLE_PATH;
  }

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

  try {
    const found = execSync('command -v chromium || command -v chromium-browser || command -v google-chrome || command -v chrome', { encoding: 'utf8' }).trim().split('\n')[0];
    if (found) return found;
  } catch {}

  return undefined;
}

// ── Per-user client instances ──
// Map<userId, { client, qrCodeData, connectionStatus, clientInfo }>
const userClients = new Map();

function getUserClient(userId) {
  if (!userClients.has(userId)) {
    userClients.set(userId, {
      client: null,
      qrCodeData: null,
      connectionStatus: 'disconnected',
      clientInfo: null,
      lastError: null,
    });
  }
  return userClients.get(userId);
}

function getStatus(userId) {
  const uc = getUserClient(userId);
  return {
    status: uc.connectionStatus,
    qr: uc.qrCodeData,
    info: uc.clientInfo,
    error: uc.lastError,
  };
}

async function initialize(userId) {
  const uc = getUserClient(userId);

  if (uc.client) {
    try { await uc.client.destroy(); } catch {}
    uc.client = null;
  }

  uc.connectionStatus = 'initializing';
  uc.qrCodeData = null;
  uc.clientInfo = null;
  uc.lastError = null;

  const client = new Client({
    authStrategy: new LocalAuth({ clientId: userId }),
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

  uc.client = client;

  client.on('qr', async (qr) => {
    uc.connectionStatus = 'qr';
    qrcode.generate(qr, { small: true });
    try {
      uc.qrCodeData = await QRCode.toDataURL(qr);
    } catch {
      uc.qrCodeData = null;
    }
    console.log(`📱 [${userId.slice(0, 8)}] Scan the QR code to connect WhatsApp`);
  });

  client.on('ready', () => {
    uc.connectionStatus = 'ready';
    uc.qrCodeData = null;
    uc.clientInfo = {
      pushname: client.info?.pushname || 'Unknown',
      phone: client.info?.wid?.user || 'Unknown',
      platform: client.info?.platform || 'Unknown',
    };
    console.log(`✅ [${userId.slice(0, 8)}] WhatsApp connected as ${uc.clientInfo.pushname} (${uc.clientInfo.phone})`);
  });

  client.on('disconnected', (reason) => {
    uc.connectionStatus = 'disconnected';
    uc.qrCodeData = null;
    uc.clientInfo = null;
    db.clearUserBotData(userId);
    console.log(`❌ [${userId.slice(0, 8)}] WhatsApp disconnected:`, reason);
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

    db.pushToUser(userId, 'messages', messageData);
    db.incrementStatUser(userId, 'messagesReceived');

    // Fire webhooks for this user
    const webhooks = db.getUser(userId, 'webhooks') || [];
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

  console.log(`🚀 [${userId.slice(0, 8)}] Initializing WhatsApp client...`);

  try {
    await client.initialize();
  } catch (err) {
    uc.connectionStatus = 'error';
    uc.lastError = err.message;
    console.error(`⚠️  [${userId.slice(0, 8)}] WhatsApp client initialization failed:`, err.message);
    console.error('   The dashboard is still accessible. Fix the issue and restart.');
  }
}

async function sendMessage(userId, number, message) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = number.includes('@c.us') ? number : `${number}@c.us`;
  const result = await uc.client.sendMessage(chatId, message);

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

  db.pushToUser(userId, 'messages', messageData);
  db.incrementStatUser(userId, 'messagesSent');

  return messageData;
}

async function sendGroupMessage(userId, groupId, message) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const result = await uc.client.sendMessage(chatId, message);

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

  db.pushToUser(userId, 'messages', messageData);
  db.incrementStatUser(userId, 'messagesSent');

  return messageData;
}

async function getGroups(userId) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chats = await uc.client.getChats();
  return chats
    .filter((c) => c.isGroup)
    .map((c) => ({
      id: c.id._serialized,
      name: c.name,
      participantCount: c.participants?.length || 0,
      isReadOnly: c.isReadOnly,
    }));
}

async function joinGroup(userId, inviteCode) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const code = inviteCode.replace('https://chat.whatsapp.com/', '');
  const result = await uc.client.acceptInvite(code);
  db.incrementStatUser(userId, 'groupsJoined');
  return result;
}

async function leaveGroup(userId, groupId) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const chat = await uc.client.getChatById(chatId);
  if (!chat.isGroup) throw new Error('Chat is not a group');
  await chat.leave();
  db.incrementStatUser(userId, 'groupsLeft');
  return { success: true, groupId: chatId };
}

async function addToGroup(userId, groupId, participants) {
  const uc = getUserClient(userId);
  if (!uc.client || uc.connectionStatus !== 'ready') {
    throw new Error('WhatsApp client is not connected');
  }

  const chatId = groupId.includes('@g.us') ? groupId : `${groupId}@g.us`;
  const chat = await uc.client.getChatById(chatId);
  if (!chat.isGroup) throw new Error('Chat is not a group');

  const participantIds = participants.map((p) =>
    p.includes('@c.us') ? p : `${p}@c.us`
  );

  const result = await chat.addParticipants(participantIds);
  return result;
}

async function disconnect(userId) {
  const uc = getUserClient(userId);
  if (!uc.client) throw new Error('No active WhatsApp session');
  uc.connectionStatus = 'disconnected';
  uc.qrCodeData = null;
  uc.clientInfo = null;
  uc.lastError = null;
  db.clearUserBotData(userId);
  try {
    await uc.client.logout();
  } catch {}
  try {
    await uc.client.destroy();
  } catch {}
  uc.client = null;
  console.log(`🔌 [${userId.slice(0, 8)}] WhatsApp disconnected by user`);
}

async function reconnect(userId) {
  console.log(`🔄 [${userId.slice(0, 8)}] Reconnecting WhatsApp...`);
  await initialize(userId);
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
