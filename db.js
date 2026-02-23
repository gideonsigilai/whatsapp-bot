const fs = require('fs');
const path = require('path');

const DATA_DIR = path.join(__dirname, 'data');
const USERS_DIR = path.join(DATA_DIR, 'users');

const DEFAULT_USER_DATA = {
  messages: [],
  groups: [],
  webhooks: [],
  stats: {
    messagesSent: 0,
    messagesReceived: 0,
    groupsJoined: 0,
    groupsLeft: 0,
  },
};

const DEFAULT_GLOBAL = {
  config: {
    botName: 'WA Bot Server',
    port: 3000,
    tunnelEnabled: false,
  },
};

// ── Directory helpers ──

function ensureDir(dir) {
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
}

// ── Global config (shared, not per-user) ──

const GLOBAL_PATH = path.join(DATA_DIR, 'global.json');

function loadGlobal() {
  try {
    if (!fs.existsSync(GLOBAL_PATH)) {
      saveGlobal(DEFAULT_GLOBAL);
      return { ...DEFAULT_GLOBAL };
    }
    return JSON.parse(fs.readFileSync(GLOBAL_PATH, 'utf-8'));
  } catch {
    return { ...DEFAULT_GLOBAL };
  }
}

function saveGlobal(data) {
  ensureDir(DATA_DIR);
  fs.writeFileSync(GLOBAL_PATH, JSON.stringify(data, null, 2), 'utf-8');
}

function getGlobal(key) {
  const data = loadGlobal();
  return data[key];
}

function setGlobal(key, value) {
  const data = loadGlobal();
  data[key] = value;
  saveGlobal(data);
}

// ── Per-user data ──

// Validate userId to prevent path traversal attacks
function sanitizeUserId(userId) {
  if (!userId || typeof userId !== 'string') {
    throw new Error('Invalid user ID');
  }
  // Only allow UUID-format characters (alphanumeric + hyphens)
  const clean = userId.replace(/[^a-zA-Z0-9\-]/g, '');
  if (clean !== userId || clean.length === 0) {
    throw new Error('Invalid user ID format');
  }
  return clean;
}

function userDataPath(userId) {
  const safeId = sanitizeUserId(userId);
  return path.join(USERS_DIR, safeId, 'data.json');
}

function initUser(userId) {
  const p = userDataPath(userId);
  ensureDir(path.dirname(p));
  if (!fs.existsSync(p)) {
    fs.writeFileSync(p, JSON.stringify(DEFAULT_USER_DATA, null, 2), 'utf-8');
  }
}

function loadUser(userId) {
  try {
    const p = userDataPath(userId);
    if (!fs.existsSync(p)) {
      initUser(userId);
      return { ...DEFAULT_USER_DATA };
    }
    return JSON.parse(fs.readFileSync(p, 'utf-8'));
  } catch {
    return { ...DEFAULT_USER_DATA };
  }
}

function saveUser(userId, data) {
  const p = userDataPath(userId);
  ensureDir(path.dirname(p));
  fs.writeFileSync(p, JSON.stringify(data, null, 2), 'utf-8');
}

function getUser(userId, key) {
  const data = loadUser(userId);
  return data[key];
}

function setUser(userId, key, value) {
  const data = loadUser(userId);
  data[key] = value;
  saveUser(userId, data);
}

function pushToUser(userId, key, item) {
  const data = loadUser(userId);
  if (!Array.isArray(data[key])) data[key] = [];
  data[key].push(item);
  // Cap messages at 500
  if (key === 'messages' && data[key].length > 500) {
    data[key] = data[key].slice(-500);
  }
  saveUser(userId, data);
}

function removeFromUser(userId, key, predicate) {
  const data = loadUser(userId);
  if (!Array.isArray(data[key])) return;
  data[key] = data[key].filter((item) => !predicate(item));
  saveUser(userId, data);
}

function incrementStatUser(userId, statKey) {
  const data = loadUser(userId);
  if (!data.stats) data.stats = { ...DEFAULT_USER_DATA.stats };
  data.stats[statKey] = (data.stats[statKey] || 0) + 1;
  saveUser(userId, data);
}

function clearUserBotData(userId) {
  const data = loadUser(userId);
  data.messages = [];
  data.webhooks = [];
  data.stats = { ...DEFAULT_USER_DATA.stats };
  saveUser(userId, data);
}

// ── Legacy global compat (for config access during startup) ──

function get(key) {
  return getGlobal(key);
}

function set(key, value) {
  return setGlobal(key, value);
}

// Initialize global config on first require
ensureDir(DATA_DIR);
if (!fs.existsSync(GLOBAL_PATH)) {
  saveGlobal(DEFAULT_GLOBAL);
}

module.exports = {
  // Global
  loadGlobal,
  saveGlobal,
  getGlobal,
  setGlobal,
  get,
  set,

  // Per-user
  initUser,
  loadUser,
  saveUser,
  getUser,
  setUser,
  pushToUser,
  removeFromUser,
  incrementStatUser,
  clearUserBotData,
};
