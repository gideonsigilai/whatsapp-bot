// ── API Base URL Config ──
function getApiBase() {
  return localStorage.getItem('wa_api_base') || window.location.origin;
}

function setApiBase(url) {
  const clean = url.replace(/\/$/, '');
  localStorage.setItem('wa_api_base', clean);
  return clean;
}

// ── API Helper ──
async function api(endpoint, options = {}) {
  try {
    const base = getApiBase();
    const res = await fetch(`${base}/api${endpoint}`, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Request failed');
    return data;
  } catch (err) {
    throw err;
  }
}

// ── Toast Notifications ──
function toast(message, type = 'info') {
  const container = document.getElementById('toastContainer');
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.innerHTML = `<span>${type === 'success' ? '✅' : type === 'error' ? '❌' : 'ℹ️'}</span> ${message}`;
  container.appendChild(el);
  setTimeout(() => {
    el.style.opacity = '0';
    el.style.transform = 'translateX(100px)';
    el.style.transition = 'all 0.3s ease';
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

// ── Status Polling ──
let lastStatus = null;

async function pollStatus() {
  try {
    const data = await api('/status');
    const badge = document.getElementById('statusBadge');
    const text = document.getElementById('statusText');
    const overlay = document.getElementById('qrOverlay');
    const qrImg = document.getElementById('qrImage');

    badge.className = 'status-badge';

    const btnDisconnect = document.getElementById('btnDisconnect');
    const btnReconnect = document.getElementById('btnReconnect');

    if (data.status === 'ready') {
      badge.classList.add('ready');
      text.textContent = `Connected — ${data.info?.pushname || 'Bot'}`;
      overlay.classList.remove('visible');
      btnDisconnect.style.display = '';
      btnReconnect.style.display = 'none';

      if (lastStatus !== 'ready') {
        toast('WhatsApp connected!', 'success');
        refreshGroups();
      }
    } else if (data.status === 'qr') {
      badge.classList.add('qr');
      text.textContent = 'Scan QR Code';
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';
      if (data.qr) {
        qrImg.src = data.qr;
        overlay.classList.add('visible');
      }
    } else {
      badge.classList.add('disconnected');
      text.textContent = data.status === 'initializing' ? 'Connecting...' : 'Disconnected';
      overlay.classList.remove('visible');
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';
    }

    lastStatus = data.status;
  } catch (err) {
    // Server not reachable
  }
}

// ── Stats Polling ──
async function pollStats() {
  try {
    const data = await api('/stats');
    document.getElementById('statSent').textContent = data.messagesSent || 0;
    document.getElementById('statReceived').textContent = data.messagesReceived || 0;
    document.getElementById('statWebhooks').textContent = data.webhookCount || 0;
  } catch (err) {
    // ignore
  }
}


// ── Messages ──
async function refreshMessages() {
  try {
    const messages = await api('/messages?limit=50');
    const log = document.getElementById('messageLog');

    if (!messages.length) {
      log.innerHTML = '<div class="empty-state"><div class="icon">💬</div><p>No messages yet</p></div>';
      return;
    }

    log.innerHTML = messages
      .map((m) => {
        const time = new Date(m.timestamp).toLocaleTimeString();
        const typeClass = m.type === 'sent' ? 'sent' : 'received';
        const avatar = m.type === 'sent' ? '📤' : '📥';
        const badge = m.type === 'sent' ? 'SENT' : 'RECV';
        const groupTag = m.isGroup ? ` <span style="opacity:0.5">· ${escHtml(m.groupName || '')}</span>` : '';

        return `
          <div class="msg-item ${typeClass}">
            <div class="msg-avatar">${avatar}</div>
            <div class="msg-content">
              <div class="msg-header">
                <span class="msg-name">${escHtml(m.contactName || m.from)}</span>
                <span class="msg-badge">${badge}</span>
                <span class="msg-time">${time}</span>
                ${groupTag}
              </div>
              <div class="msg-body">${escHtml(m.body)}</div>
            </div>
          </div>`;
      })
      .join('');
  } catch (err) {
    // ignore
  }
}

// ── Groups ──
let cachedGroups = [];

async function refreshGroups() {
  try {
    const groups = await api('/groups');
    cachedGroups = groups;
    renderGroups(groups);
    populateGroupSelects(groups);
    document.getElementById('statGroups').textContent = groups.length;
  } catch (err) {
    // ignore
  }
}

function renderGroups(groups) {
  const list = document.getElementById('groupList');

  if (!groups.length) {
    list.innerHTML = '<div class="empty-state"><div class="icon">👥</div><p>No groups found</p></div>';
    return;
  }

  list.innerHTML = groups
    .map(
      (g) => `
    <div class="group-item">
      <div class="group-info">
        <div class="group-name">${escHtml(g.name)}</div>
        <div class="group-meta">${g.participantCount} members · ${g.id}</div>
      </div>
      <div class="group-actions">
        <button class="btn btn-danger btn-sm" onclick="leaveGroup('${g.id}')">Leave</button>
      </div>
    </div>`
    )
    .join('');
}

function populateGroupSelects(groups) {
  const selects = [document.getElementById('groupSelect'), document.getElementById('addGroupSelect')];
  selects.forEach((sel) => {
    if (!sel) return;
    sel.innerHTML =
      '<option value="">— Select a group —</option>' +
      groups.map((g) => `<option value="${g.id}">${escHtml(g.name)}</option>`).join('');
  });
}

// ── Webhooks ──
async function refreshHooks() {
  try {
    const hooks = await api('/hooks');
    const list = document.getElementById('hookList');

    if (!hooks.length) {
      list.innerHTML = '<div class="empty-state" style="padding:16px"><div class="icon">🔔</div><p>No webhooks registered</p></div>';
      return;
    }

    list.innerHTML = hooks
      .map(
        (h) => `
      <div class="hook-item">
        <div style="flex:1">
          <div class="hook-name">${escHtml(h.name)}</div>
          <div class="hook-url">${escHtml(h.url)}</div>
        </div>
        <button class="btn btn-danger btn-sm" onclick="removeHook('${h.id}')">Remove</button>
      </div>`
      )
      .join('');
  } catch (err) {
    // ignore
  }
}

// ── Actions ──
async function sendMessage() {
  const number = document.getElementById('msgNumber').value.trim();
  const message = document.getElementById('msgBody').value.trim();
  if (!number || !message) return toast('Please fill in all fields', 'error');

  try {
    await api('/send-message', {
      method: 'POST',
      body: JSON.stringify({ number, message }),
    });
    toast('Message sent!', 'success');
    document.getElementById('msgBody').value = '';
    refreshMessages();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function sendGroupMessage() {
  const groupId = document.getElementById('groupSelect').value;
  const message = document.getElementById('groupMsgBody').value.trim();
  if (!groupId || !message) return toast('Please select a group and type a message', 'error');

  try {
    await api('/send-group-message', {
      method: 'POST',
      body: JSON.stringify({ groupId, message }),
    });
    toast('Group message sent!', 'success');
    document.getElementById('groupMsgBody').value = '';
    refreshMessages();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function joinGroup() {
  const inviteLink = document.getElementById('inviteLink').value.trim();
  if (!inviteLink) return toast('Please enter an invite link', 'error');

  try {
    await api('/join-group', {
      method: 'POST',
      body: JSON.stringify({ inviteLink }),
    });
    toast('Joined the group!', 'success');
    document.getElementById('inviteLink').value = '';
    refreshGroups();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function leaveGroup(groupId) {
  if (!confirm('Are you sure you want to leave this group?')) return;

  try {
    await api('/leave-group', {
      method: 'POST',
      body: JSON.stringify({ groupId }),
    });
    toast('Left the group', 'success');
    refreshGroups();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function addToGroup() {
  const groupId = document.getElementById('addGroupSelect').value;
  const raw = document.getElementById('addParticipants').value.trim();
  if (!groupId || !raw) return toast('Please fill in all fields', 'error');

  const participants = raw.split(',').map((s) => s.trim()).filter(Boolean);
  if (!participants.length) return toast('Enter at least one phone number', 'error');

  try {
    await api('/add-to-group', {
      method: 'POST',
      body: JSON.stringify({ groupId, participants }),
    });
    toast('Members added!', 'success');
    document.getElementById('addParticipants').value = '';
    refreshGroups();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function registerHook() {
  const url = document.getElementById('hookUrl').value.trim();
  if (!url) return toast('Please enter a webhook URL', 'error');

  try {
    await api('/hooks/register', {
      method: 'POST',
      body: JSON.stringify({ url, name: url }),
    });
    toast('Webhook registered!', 'success');
    document.getElementById('hookUrl').value = '';
    refreshHooks();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function removeHook(id) {
  try {
    await api('/hooks/unregister', {
      method: 'DELETE',
      body: JSON.stringify({ id }),
    });
    toast('Webhook removed', 'success');
    refreshHooks();
    pollStats();
  } catch (err) {
    toast(err.message, 'error');
  }
}

// ── WhatsApp Connection Controls ──
async function disconnectWA() {
  if (!confirm('Disconnect WhatsApp? You will need to scan a QR code to reconnect.')) return;
  try {
    await api('/disconnect', { method: 'POST' });
    toast('WhatsApp disconnected', 'success');
    document.getElementById('btnDisconnect').style.display = 'none';
    document.getElementById('btnReconnect').style.display = '';
    pollStatus();
  } catch (err) {
    toast('Failed to disconnect: ' + err.message, 'error');
  }
}

async function reconnectWA() {
  try {
    document.getElementById('btnReconnect').textContent = '⏳ Connecting...';
    document.getElementById('btnReconnect').disabled = true;
    await api('/reconnect', { method: 'POST' });
    toast('Reconnecting… scan QR if prompted', 'info');
    setTimeout(() => {
      document.getElementById('btnReconnect').textContent = '🔄 Reconnect';
      document.getElementById('btnReconnect').disabled = false;
      pollStatus();
    }, 3000);
  } catch (err) {
    document.getElementById('btnReconnect').textContent = '🔄 Reconnect';
    document.getElementById('btnReconnect').disabled = false;
    toast('Failed to reconnect: ' + err.message, 'error');
  }
}

// ── Tabs ──
function switchTab(btn, tabId) {
  const parent = btn.closest('.card-body');
  parent.querySelectorAll('.tab-btn').forEach((b) => b.classList.remove('active'));
  parent.querySelectorAll('.tab-content').forEach((t) => t.classList.remove('active'));
  btn.classList.add('active');
  document.getElementById(tabId).classList.add('active');
}

// ── Helpers ──
function escHtml(str) {
  const div = document.createElement('div');
  div.textContent = str || '';
  return div.innerHTML;
}

// ── Init ──
function startPolling() {
  pollStatus();
  pollStats();
  refreshMessages();
  refreshHooks();
  setInterval(pollStatus, 3000);
  setInterval(pollStats, 5000);
  setInterval(refreshMessages, 5000);
  setInterval(refreshHooks, 15000);
}

document.addEventListener('DOMContentLoaded', () => {
  const isLocal = location.hostname === 'localhost' || location.hostname === '127.0.0.1';
  const hasSavedBase = !!localStorage.getItem('wa_api_base');

  document.getElementById('apiBaseForm').addEventListener('submit', (e) => {
    e.preventDefault();
    const val = document.getElementById('apiBaseInput').value.trim();
    if (!val) return;
    setApiBase(val);
    document.getElementById('apiConfigBanner').style.display = 'none';
    toast('Server URL saved!', 'success');
    startPolling();
  });

  document.getElementById('apiBaseEditBtn').addEventListener('click', () => {
    document.getElementById('apiBaseInput').value = localStorage.getItem('wa_api_base') || '';
    showApiConfigBanner();
  });

  if (!isLocal && !hasSavedBase) {
    // Show banner and wait — don't poll until URL is saved
    showApiConfigBanner();
  } else {
    // localhost or already configured — start immediately
    startPolling();
  }
});

function showApiConfigBanner() {
  const banner = document.getElementById('apiConfigBanner');
  banner.style.display = 'flex';
  document.getElementById('apiBaseInput').value = localStorage.getItem('wa_api_base') || '';
}
