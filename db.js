const fs = require('fs');
const path = require('path');

const DB_PATH = path.join(__dirname, 'data.json');

const DEFAULT_DATA = {
  config: {
    botName: 'WA Bot Server',
    port: 3000,
    tunnelEnabled: false,
  },
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

function load() {
  try {
    if (!fs.existsSync(DB_PATH)) {
      save(DEFAULT_DATA);
      return { ...DEFAULT_DATA };
    }
    const raw = fs.readFileSync(DB_PATH, 'utf-8');
    return JSON.parse(raw);
  } catch {
    return { ...DEFAULT_DATA };
  }
}

function save(data) {
  fs.writeFileSync(DB_PATH, JSON.stringify(data, null, 2), 'utf-8');
}

function get(key) {
  const data = load();
  return data[key];
}

function set(key, value) {
  const data = load();
  data[key] = value;
  save(data);
}

function pushTo(key, item) {
  const data = load();
  if (!Array.isArray(data[key])) data[key] = [];
  data[key].push(item);
  // Keep message log capped at 500
  if (key === 'messages' && data[key].length > 500) {
    data[key] = data[key].slice(-500);
  }
  save(data);
}

function removeFrom(key, predicate) {
  const data = load();
  if (!Array.isArray(data[key])) return;
  data[key] = data[key].filter((item) => !predicate(item));
  save(data);
}

function incrementStat(statKey) {
  const data = load();
  if (!data.stats) data.stats = { ...DEFAULT_DATA.stats };
  data.stats[statKey] = (data.stats[statKey] || 0) + 1;
  save(data);
}

// Initialize on first require
if (!fs.existsSync(DB_PATH)) {
  save(DEFAULT_DATA);
}

module.exports = { load, save, get, set, pushTo, removeFrom, incrementStat };
