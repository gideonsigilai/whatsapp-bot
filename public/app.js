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
  const icon = type === 'success' ? '✓' : type === 'error' ? '✗' : 'ℹ';
  el.innerHTML = `<span>${icon}</span> ${message}`;
  container.appendChild(el);
  setTimeout(() => {
    el.style.opacity = '0';
    el.style.transform = 'translateX(100px)';
    el.style.transition = 'all 0.3s ease';
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

// ── Dark Mode ──
function toggleDarkMode() {
  document.documentElement.classList.toggle('dark');
  const icon = document.getElementById('themeIcon');
  if (document.documentElement.classList.contains('dark')) {
    icon.textContent = 'light_mode';
  } else {
    icon.textContent = 'dark_mode';
  }
}

// ── Status Polling ──
let lastStatus = null;
let isQrDismissed = false;

async function pollStatus() {
  try {
    const data = await api('/status');
    const dot = document.getElementById('statusDot');
    const ping = document.getElementById('statusPing');
    const text = document.getElementById('statusText');
    const overlay = document.getElementById('qrOverlay');
    const qrImg = document.getElementById('qrImage');

    const btnDisconnect = document.getElementById('btnDisconnect');
    const btnReconnect = document.getElementById('btnReconnect');

    if (data.status === 'ready') {
      dot.className = 'relative inline-flex rounded-full h-2 w-2 bg-green-500';
      ping.className = 'animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75';
      text.textContent = `Connected — ${data.info?.pushname || 'Bot'}`;
      text.className = 'text-xs font-mono text-green-600 dark:text-green-400';
      overlay.classList.remove('visible');
      btnDisconnect.style.display = '';
      btnReconnect.style.display = 'none';

      if (lastStatus !== 'ready') {
        toast('WhatsApp connected!', 'success');
        refreshGroups();
      }
    } else if (data.status === 'qr') {
      dot.className = 'relative inline-flex rounded-full h-2 w-2 bg-amber-500';
      ping.className = 'animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75';
      text.textContent = 'Scan QR Code';
      text.className = 'text-xs font-mono text-amber-600 dark:text-amber-400';
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';
      if (data.qr) {
        qrImg.src = data.qr;
        qrImg.alt = 'QR Code';
        if (!isQrDismissed) {
          overlay.classList.add('visible');
        }
      }
    } else {
      dot.className = 'relative inline-flex rounded-full h-2 w-2 bg-neutral-400';
      ping.className = 'absolute inline-flex h-full w-full rounded-full bg-neutral-400 opacity-0';
      text.textContent = data.status === 'initializing' ? 'Connecting...' : 'Disconnected';
      text.className = 'text-xs font-mono text-neutral-500 dark:text-neutral-400';
      overlay.classList.remove('visible');
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';

      if (lastStatus === 'ready') {
        cachedGroups = [];
        renderGroups([]);
        populateGroupSelects([]);
      }
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
    const sent = data.messagesSent || 0;
    const recv = data.messagesReceived || 0;
    document.getElementById('statSent').textContent = sent;
    document.getElementById('statReceived').textContent = recv;
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
      log.innerHTML = `
        <div class="flex-1 p-8 flex flex-col items-center justify-center text-center">
          <div class="w-16 h-16 rounded-full border border-dashed border-neutral-300 dark:border-neutral-700 flex items-center justify-center mb-4">
            <span class="material-symbols-outlined text-2xl text-neutral-400 dark:text-neutral-600">history</span>
          </div>
          <h3 class="text-sm font-medium text-neutral-900 dark:text-neutral-200">No logs available</h3>
          <p class="text-xs text-neutral-500 mt-2 max-w-xs">System events and message statuses will appear here in real-time.</p>
        </div>`;
      return;
    }

    log.innerHTML = messages
      .map((m) => {
        const time = new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        const typeLabel = m.type === 'sent' ? 'OUT' : 'IN';
        const typeColor = m.type === 'sent'
          ? 'bg-neutral-900 dark:bg-neutral-200 text-white dark:text-neutral-900'
          : 'bg-neutral-200 dark:bg-neutral-700 text-neutral-700 dark:text-neutral-300';
        const contact = escHtml(m.contactName || m.from);
        const body = escHtml(m.body).substring(0, 60) + (m.body.length > 60 ? '…' : '');

        return `
          <div class="grid grid-cols-12 gap-4 px-6 py-3 border-b border-border-light dark:border-border-dark text-sm hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors">
            <div class="col-span-2 text-neutral-500 dark:text-neutral-400 font-mono text-xs">${time}</div>
            <div class="col-span-2"><span class="px-2 py-0.5 rounded text-[10px] font-bold ${typeColor}">${typeLabel}</span></div>
            <div class="col-span-5 text-neutral-900 dark:text-neutral-200 truncate">${contact}</div>
            <div class="col-span-3 text-right text-neutral-500 dark:text-neutral-400 truncate">${body}</div>
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
    list.innerHTML = `
      <div class="flex flex-col items-center justify-center text-center py-8">
        <span class="material-symbols-outlined text-3xl text-neutral-300 dark:text-neutral-700 mb-2">group_off</span>
        <h3 class="text-sm font-medium text-neutral-500 dark:text-neutral-500">No groups found</h3>
      </div>`;
    return;
  }

  list.innerHTML = groups
    .map(
      (g) => `
    <div class="flex items-center justify-between px-5 py-3.5 border-b border-border-light dark:border-border-dark hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors last:border-b-0">
      <div class="flex-1 min-w-0">
        <div class="text-sm font-medium text-neutral-900 dark:text-neutral-200 truncate">${escHtml(g.name)}</div>
        <div class="text-xs text-neutral-500 dark:text-neutral-400 font-mono mt-0.5">${g.participantCount} members · ${g.id}</div>
      </div>
      <button class="ml-3 px-3 py-1.5 rounded text-xs font-medium border border-red-200 dark:border-red-800 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors" onclick="leaveGroup('${g.id}')">Leave</button>
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
      list.innerHTML = `
        <div class="flex flex-col items-center justify-center text-center py-8 border border-dashed border-neutral-200 dark:border-neutral-800 rounded bg-neutral-50/50 dark:bg-neutral-900/20">
          <span class="material-symbols-outlined text-3xl text-neutral-300 dark:text-neutral-700 mb-2">cloud_off</span>
          <h3 class="text-sm font-medium text-neutral-500 dark:text-neutral-500">No active webhooks</h3>
        </div>`;
      return;
    }

    list.innerHTML = hooks
      .map(
        (h) => `
      <div class="flex items-center justify-between px-4 py-3 border-b border-border-light dark:border-border-dark last:border-b-0 hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors">
        <div class="flex-1 min-w-0">
          <div class="text-sm font-medium text-neutral-900 dark:text-neutral-200">${escHtml(h.name)}</div>
          <div class="text-xs text-neutral-500 dark:text-neutral-400 font-mono truncate">${escHtml(h.url)}</div>
        </div>
        <button class="ml-3 px-3 py-1.5 rounded text-xs font-medium border border-red-200 dark:border-red-800 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors" onclick="removeHook('${h.id}')">Remove</button>
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

function closeQrOverlay() {
  isQrDismissed = true;
  document.getElementById('qrOverlay').classList.remove('visible');
}

async function reconnectWA() {
  try {
    isQrDismissed = false;
    document.getElementById('qrOverlay').classList.add('visible');
    const qrImg = document.getElementById('qrImage');
    qrImg.removeAttribute('src');
    qrImg.alt = 'Generating new QR Code... Please wait.';

    document.getElementById('btnReconnect').textContent = '⏳ Connecting...';
    document.getElementById('btnReconnect').disabled = true;
    await api('/reconnect', { method: 'POST' });
    toast('Reconnecting… scan QR if prompted', 'info');
    setTimeout(() => {
      document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
      document.getElementById('btnReconnect').disabled = false;
      pollStatus();
    }, 3000);
  } catch (err) {
    document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
    document.getElementById('btnReconnect').disabled = false;
    toast('Failed to reconnect: ' + err.message, 'error');
  }
}

// ── Tabs ──
function switchTab(btn, tabId) {
  const parent = btn.closest('.p-5') || btn.closest('div');
  const container = parent.parentElement;
  container.querySelectorAll('.tab-btn-new').forEach((b) => {
    b.classList.remove('active', 'border-neutral-900', 'dark:border-white', 'text-neutral-900', 'dark:text-white');
    b.classList.add('border-transparent', 'text-neutral-400');
  });
  container.querySelectorAll('.tab-content').forEach((t) => {
    t.classList.remove('active');
    t.style.display = 'none';
  });
  btn.classList.add('active', 'border-neutral-900', 'dark:border-white', 'text-neutral-900', 'dark:text-white');
  btn.classList.remove('border-transparent', 'text-neutral-400');
  const target = document.getElementById(tabId);
  target.classList.add('active');
  target.style.display = '';
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
  startPolling();
});
