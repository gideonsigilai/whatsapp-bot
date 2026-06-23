// ── Auth State ──
let authToken = localStorage.getItem('wa_token') || null;
let currentUser = null;
let resetEmail = ''; // stored between forgot → reset flow

// ── API Base URL Config ──
function getApiBase() {
  return localStorage.getItem('wa_api_base') || window.location.origin;
}

function setApiBase(url) {
  const clean = url.replace(/\/$/, '');
  localStorage.setItem('wa_api_base', clean);
  return clean;
}

// ── API Helper (sends auth token) ──
async function api(endpoint, options = {}) {
  try {
    const base = getApiBase();
    const headers = { 'Content-Type': 'application/json' };
    if (authToken) {
      headers['Authorization'] = `Bearer ${authToken}`;
    }
    const res = await fetch(`${base}/api${endpoint}`, {
      headers,
      ...options,
    });
    const data = await res.json();
    if (!res.ok) {
      if (res.status === 401) {
        // Token invalid — force logout
        handleLogout();
        throw new Error('Session expired — please log in again');
      }
      throw new Error(data.error || 'Request failed');
    }
    return data;
  } catch (err) {
    throw err;
  }
}

// ── Auth API Helper (no token needed) ──
async function authApi(endpoint, options = {}) {
  const base = getApiBase();
  const res = await fetch(`${base}/auth${endpoint}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Request failed');
  return data;
}

// ── Auth UI Management ──

function showAuthView(view) {
  const views = { login: 'authLogin', register: 'authRegister', forgot: 'authForgot', reset: 'authReset' };
  const subtitles = {
    login: 'Sign in to continue',
    register: 'Create your account',
    forgot: 'Reset your password',
    reset: 'Enter the code from your server console',
  };

  Object.values(views).forEach((id) => {
    document.getElementById(id).style.display = 'none';
  });
  document.getElementById(views[view]).style.display = '';
  document.getElementById('authSubtitle').textContent = subtitles[view] || '';
  hideAuthMessage();
}

function showAuthMessage(msg, type = 'error') {
  const el = document.getElementById('authMessage');
  el.textContent = msg;
  el.className = `mt-4 text-center text-sm ${type === 'error' ? 'text-red-500' : 'text-green-500'}`;
  el.classList.remove('hidden');
}

function hideAuthMessage() {
  document.getElementById('authMessage').classList.add('hidden');
}

function showDashboard(user) {
  currentUser = user;
  authToken = localStorage.getItem('wa_token');
  document.getElementById('authScreen').style.display = 'none';
  document.getElementById('dashboardContent').style.display = '';
  document.getElementById('userEmail').textContent = user.email;
  startPolling();
}

function showAuthScreen() {
  document.getElementById('authScreen').style.display = '';
  document.getElementById('dashboardContent').style.display = 'none';
  showAuthView('login');
}

// ── Auth Handlers ──

async function handleLogin() {
  const email = document.getElementById('loginEmail').value.trim();
  const password = document.getElementById('loginPassword').value;

  if (!email || !password) return showAuthMessage('Please fill in all fields');

  try {
    hideAuthMessage();
    const data = await authApi('/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    localStorage.setItem('wa_token', data.token);
    authToken = data.token;
    showDashboard(data.user);
    toast('Logged in successfully', 'success');
  } catch (err) {
    showAuthMessage(err.message);
  }
}

async function handleRegister() {
  const email = document.getElementById('registerEmail').value.trim();
  const password = document.getElementById('registerPassword').value;
  const confirm = document.getElementById('registerConfirm').value;

  if (!email || !password || !confirm) return showAuthMessage('Please fill in all fields');
  if (password !== confirm) return showAuthMessage('Passwords do not match');
  if (password.length < 6) return showAuthMessage('Password must be at least 6 characters');

  try {
    hideAuthMessage();
    const data = await authApi('/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    localStorage.setItem('wa_token', data.token);
    authToken = data.token;
    showDashboard(data.user);
    toast('Account created!', 'success');
  } catch (err) {
    showAuthMessage(err.message);
  }
}

async function handleForgotPassword() {
  const email = document.getElementById('forgotEmail').value.trim();
  if (!email) return showAuthMessage('Please enter your email');

  try {
    hideAuthMessage();
    await authApi('/forgot-password', {
      method: 'POST',
      body: JSON.stringify({ email }),
    });
    resetEmail = email;
    showAuthView('reset');
    showAuthMessage('Check your server console for the reset code.', 'success');
  } catch (err) {
    showAuthMessage(err.message);
  }
}

async function handleResetPassword() {
  const otp = document.getElementById('resetOtp').value.trim();
  const newPassword = document.getElementById('resetNewPassword').value;

  if (!otp || !newPassword) return showAuthMessage('Please fill in all fields');
  if (newPassword.length < 6) return showAuthMessage('Password must be at least 6 characters');

  try {
    hideAuthMessage();
    const data = await authApi('/reset-password', {
      method: 'POST',
      body: JSON.stringify({ email: resetEmail, otp, newPassword }),
    });
    localStorage.setItem('wa_token', data.token);
    authToken = data.token;
    showDashboard(data.user);
    toast('Password reset! You are now logged in.', 'success');
  } catch (err) {
    showAuthMessage(err.message);
  }
}

function handleLogout() {
  localStorage.removeItem('wa_token');
  authToken = null;
  currentUser = null;
  stopPolling();
  showAuthScreen();
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
let pollingIntervals = [];

async function pollStatus() {
  try {
    const data = await api('/status');
    const dot = document.getElementById('statusDot');
    const ping = document.getElementById('statusPing');
    const text = document.getElementById('statusText');
    const overlay = document.getElementById('qrOverlay');
    const qrImg = document.getElementById('qrImage');
    const qrLoading = document.getElementById('qrLoading');
    const qrReady = document.getElementById('qrReady');

    const btnDisconnect = document.getElementById('btnDisconnect');
    const btnReconnect = document.getElementById('btnReconnect');

    if (data.status === 'ready') {
      dot.style.background = '#31cb00';
      ping.style.background = '#31cb00';
      ping.className = 'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75';
      text.textContent = `Connected — ${data.info?.pushname || 'Bot'}`;
      text.style.color = '#31cb00';
      overlay.classList.remove('visible');
      btnDisconnect.style.display = '';
      btnReconnect.style.display = 'none';

      if (lastStatus !== 'ready') {
        toast('WhatsApp connected!', 'success');
        refreshGroups();
      }
    } else if (data.status === 'qr') {
      dot.style.background = '#f1d302';
      ping.style.background = '#f1d302';
      ping.className = 'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75';
      text.textContent = 'Scan QR Code';
      text.style.color = '#f1d302';
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';
      if (data.qr) {
        document.getElementById('qrImage').src = data.qr;
        
        document.getElementById('connectionMethodForm').style.display = 'none';
        qrLoading.style.display = 'none';
        qrReady.style.display = '';
        
        document.getElementById('readyTitle').textContent = 'Scan QR Code';
        document.getElementById('readyInstructions').textContent = 'Open WhatsApp → Settings → Linked Devices → Link a Device';
        document.getElementById('pairingCodeContainer').style.display = 'none';
        document.getElementById('qrImageContainer').style.display = '';
        document.getElementById('readyStatus').textContent = 'Waiting for scan…';
        
        if (!isQrDismissed) {
          overlay.classList.add('visible');
        }
      }
    } else if (data.status === 'pairing_code') {
      dot.style.background = '#f1d302';
      ping.style.background = '#f1d302';
      ping.className = 'animate-ping absolute inline-flex h-full w-full rounded-full opacity-75';
      text.textContent = 'Pairing Code Active';
      text.style.color = '#f1d302';
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';
      if (data.pairingCode) {
        document.getElementById('pairingCodeDisplay').textContent = data.pairingCode;
        
        document.getElementById('connectionMethodForm').style.display = 'none';
        qrLoading.style.display = 'none';
        qrReady.style.display = '';
        
        document.getElementById('readyTitle').textContent = 'Link with Phone Number';
        document.getElementById('readyInstructions').textContent = 'Open WhatsApp → Settings → Linked Devices → Link a Device → Link with phone number instead';
        document.getElementById('pairingCodeContainer').style.display = '';
        document.getElementById('qrImageContainer').style.display = 'none';
        document.getElementById('readyStatus').textContent = 'Waiting for pairing…';
        
        if (!isQrDismissed) {
          overlay.classList.add('visible');
        }
      }
    } else if (data.status === 'error') {
      dot.style.background = '#ef4444';
      ping.style.background = '#ef4444';
      ping.className = 'absolute inline-flex h-full w-full rounded-full opacity-0';
      text.textContent = 'Connection Failed';
      text.style.color = '#ef4444';
      
      const statusText = document.getElementById('qrStatusText');
      if (statusText && overlay.classList.contains('visible') && !isQrDismissed) {
        statusText.textContent = data.error || 'Initialization failed. Check logs.';
        statusText.style.color = '#ef4444';
        
        document.getElementById('connectionMethodForm').style.display = '';
        qrLoading.style.display = 'none';
        qrReady.style.display = 'none';
      } else if (lastStatus !== 'error') {
        toast('Connection failed: ' + (data.error || 'Check server logs'), 'error');
      }
      
      btnDisconnect.style.display = 'none';
      btnReconnect.style.display = '';

      if (lastStatus === 'ready') {
        cachedGroups = [];
        renderGroups([]);
        populateGroupSelects([]);
      }
    } else {
      dot.style.background = '#737373';
      ping.style.background = '#737373';
      ping.className = 'absolute inline-flex h-full w-full rounded-full opacity-0';
      const statusMsg = data.status === 'initializing' ? 'Initializing…' : 'Disconnected';
      text.textContent = statusMsg;
      text.style.color = data.status === 'initializing' ? '#f1d302' : '#737373';
      
      if (data.status === 'disconnected') {
        if (lastStatus && lastStatus !== 'disconnected' && lastStatus !== 'initializing' && overlay.classList.contains('visible')) {
          document.getElementById('connectionMethodForm').style.display = '';
          document.getElementById('qrLoading').style.display = 'none';
          document.getElementById('qrReady').style.display = 'none';
        }
      } else if (data.status === 'initializing' && !isQrDismissed) {
        overlay.classList.add('visible');
        qrLoading.style.display = '';
        qrReady.style.display = 'none';
        const statusText = document.getElementById('qrStatusText');
        if (statusText) {
          statusText.textContent = 'Initializing…';
          statusText.style.color = '#f1d302';
        }
        const spinner = qrLoading.querySelector('.qr-spinner');
        if (spinner) spinner.style.display = '';
      }

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
    // Server not reachable or auth error (handled in api())
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
    const btn = document.getElementById('btnDisconnect');
    btn.innerHTML = '<span class="qr-spinner-inline"></span> Disconnecting…';
    btn.disabled = true;

    await api('/disconnect', { method: 'POST' });
    toast('WhatsApp disconnected — data cleared', 'success');

    cachedGroups = [];
    renderGroups([]);
    populateGroupSelects([]);
    document.getElementById('statSent').textContent = '0';
    document.getElementById('statReceived').textContent = '0';
    document.getElementById('statGroups').textContent = '0';
    document.getElementById('statWebhooks').textContent = '0';
    refreshMessages();
    refreshHooks();

    btn.innerHTML = '<span class="material-symbols-outlined text-[18px]">power_settings_new</span> Disconnect';
    btn.disabled = false;
    btn.style.display = 'none';
    document.getElementById('btnReconnect').style.display = '';
    pollStatus();
  } catch (err) {
    const btn = document.getElementById('btnDisconnect');
    btn.innerHTML = '<span class="material-symbols-outlined text-[18px]">power_settings_new</span> Disconnect';
    btn.disabled = false;
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

    const overlay = document.getElementById('qrOverlay');
    const qrLoading = document.getElementById('qrLoading');
    const qrReady = document.getElementById('qrReady');
    const connectionMethodForm = document.getElementById('connectionMethodForm');
    const statusText = document.getElementById('qrStatusText');

    connectionMethodForm.style.display = '';
    qrLoading.style.display = 'none';
    qrReady.style.display = 'none';
    overlay.classList.add('visible');

  } catch (err) {
    toast('Failed to open connect dialog: ' + err.message, 'error');
  }
}

async function requestPairing() {
  const phoneNumber = document.getElementById('pairingPhoneNumber').value.trim();
  if (!phoneNumber) return toast('Please enter a phone number', 'error');

  try {
    const btn = document.getElementById('btnRequestPairingCode');
    btn.innerHTML = '<span class="qr-spinner-inline"></span> Requesting…';
    btn.disabled = true;

    const qrLoading = document.getElementById('qrLoading');
    const connectionMethodForm = document.getElementById('connectionMethodForm');
    const statusText = document.getElementById('qrStatusText');
    const spinner = qrLoading.querySelector('.qr-spinner');

    if (spinner) spinner.style.display = '';
    connectionMethodForm.style.display = 'none';
    qrLoading.style.display = '';
    statusText.textContent = 'Preparing…';
    statusText.style.color = '#f1d302';

    document.getElementById('btnReconnect').innerHTML = '<span class="qr-spinner-inline"></span> Connecting…';
    document.getElementById('btnReconnect').disabled = true;

    await api('/reconnect', { 
      method: 'POST',
      body: JSON.stringify({ method: 'pairing_code', phoneNumber })
    });
    
    statusText.textContent = 'Waiting for pairing code…';
    toast('Requesting pairing code…', 'info');

    setTimeout(() => {
      document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
      document.getElementById('btnReconnect').disabled = false;
      btn.innerHTML = 'Generate Pairing Code';
      btn.disabled = false;
      pollStatus();
    }, 3000);
  } catch (err) {
    const btn = document.getElementById('btnRequestPairingCode');
    btn.innerHTML = 'Generate Pairing Code';
    btn.disabled = false;
    document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
    document.getElementById('btnReconnect').disabled = false;
    document.getElementById('qrOverlay').classList.remove('visible');
    toast('Failed to request pairing code: ' + err.message, 'error');
  }
}

async function requestQrCode() {
  try {
    const btn = document.getElementById('btnRequestQrCode');
    btn.innerHTML = '<span class="qr-spinner-inline"></span> Generating…';
    btn.disabled = true;

    const qrLoading = document.getElementById('qrLoading');
    const connectionMethodForm = document.getElementById('connectionMethodForm');
    const statusText = document.getElementById('qrStatusText');
    const spinner = qrLoading.querySelector('.qr-spinner');

    if (spinner) spinner.style.display = '';
    connectionMethodForm.style.display = 'none';
    qrLoading.style.display = '';
    statusText.textContent = 'Preparing…';
    statusText.style.color = '#f1d302';

    document.getElementById('btnReconnect').innerHTML = '<span class="qr-spinner-inline"></span> Connecting…';
    document.getElementById('btnReconnect').disabled = true;

    await api('/reconnect', { 
      method: 'POST',
      body: JSON.stringify({ method: 'qr' })
    });
    
    statusText.textContent = 'Waiting for QR code…';
    toast('Requesting QR code…', 'info');

    setTimeout(() => {
      document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
      document.getElementById('btnReconnect').disabled = false;
      btn.innerHTML = 'Generate QR Code';
      btn.disabled = false;
      pollStatus();
    }, 3000);
  } catch (err) {
    const btn = document.getElementById('btnRequestQrCode');
    btn.innerHTML = 'Generate QR Code';
    btn.disabled = false;
    document.getElementById('btnReconnect').innerHTML = '<span class="material-symbols-outlined text-[18px]">refresh</span> Reconnect';
    document.getElementById('btnReconnect').disabled = false;
    document.getElementById('qrOverlay').classList.remove('visible');
    toast('Failed to request QR code: ' + err.message, 'error');
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

function switchConnTab(btn, tabId) {
  const container = btn.closest('#connectionMethodForm');
  container.querySelectorAll('.tab-btn-new-conn').forEach((b) => {
    b.classList.remove('active', 'border-neutral-900', 'dark:border-white', 'text-neutral-900', 'dark:text-white');
    b.classList.add('border-transparent', 'text-neutral-400');
  });
  container.querySelectorAll('.tab-content-conn').forEach((t) => {
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

// ── Realtime WebSocket ──
let realtimeSocket = null;
let realtimeReconnectTimer = null;
let realtimeShouldRun = false;

function realtimeUrl() {
  const base = getApiBase();
  const wsBase = base.replace(/^http/i, 'ws');
  return `${wsBase}/ws?token=${encodeURIComponent(authToken || '')}`;
}

function connectRealtime() {
  realtimeShouldRun = true;
  if (!authToken) return;
  // Avoid duplicate sockets
  if (realtimeSocket && (realtimeSocket.readyState === WebSocket.OPEN || realtimeSocket.readyState === WebSocket.CONNECTING)) {
    return;
  }

  setWsState('connecting');
  let socket;
  try {
    socket = new WebSocket(realtimeUrl());
  } catch {
    scheduleRealtimeReconnect();
    return;
  }
  realtimeSocket = socket;

  socket.onopen = () => setWsState('connected');

  socket.onmessage = (ev) => {
    let msg;
    try {
      msg = JSON.parse(ev.data);
    } catch {
      return;
    }
    handleRealtimeEvent(msg.event, msg.data);
  };

  socket.onclose = () => {
    if (realtimeSocket === socket) realtimeSocket = null;
    setWsState('disconnected');
    if (realtimeShouldRun) scheduleRealtimeReconnect();
  };

  socket.onerror = () => {
    try { socket.close(); } catch {}
  };
}

function scheduleRealtimeReconnect() {
  if (!realtimeShouldRun || realtimeReconnectTimer) return;
  realtimeReconnectTimer = setTimeout(() => {
    realtimeReconnectTimer = null;
    connectRealtime();
  }, 3000);
}

function disconnectRealtime() {
  realtimeShouldRun = false;
  if (realtimeReconnectTimer) {
    clearTimeout(realtimeReconnectTimer);
    realtimeReconnectTimer = null;
  }
  if (realtimeSocket) {
    try { realtimeSocket.close(); } catch {}
    realtimeSocket = null;
  }
}

function handleRealtimeEvent(event, data) {
  pushActivity(event, data);
  switch (event) {
    case 'status':
    case 'pair_success':
      // Re-render connection status immediately (QR, pairing code, ready, ...)
      pollStatus();
      break;
    case 'message':
      refreshMessages();
      pollStats();
      if (document.getElementById('expInbox')?.classList.contains('active')) {
        loadInbox();
      }
      if (data && data.hasMedia && document.getElementById('expMedia')?.classList.contains('active')) {
        loadMedia();
      }
      if (data && data.type === 'received') {
        const who = data.contactName || data.from || 'Someone';
        toast(`New message from ${who}`, 'info');
      }
      break;
    case 'joined_group':
    case 'group_info':
      refreshGroups();
      break;
    default:
      // receipt / presence / chat_presence / picture / call — shown in Live Activity
      break;
  }
}

// ── Explore: tab switching ──
function switchExploreTab(btn, tabId) {
  const container = document.getElementById('exploreSection');
  container.querySelectorAll('.explore-tab-btn').forEach((b) => {
    b.classList.remove('active', 'border-neutral-900', 'dark:border-white', 'text-neutral-900', 'dark:text-white');
    b.classList.add('border-transparent', 'text-neutral-400');
  });
  container.querySelectorAll('.explore-tab-content').forEach((t) => {
    t.classList.remove('active');
    t.style.display = 'none';
  });
  btn.classList.add('active', 'border-neutral-900', 'dark:border-white', 'text-neutral-900', 'dark:text-white');
  btn.classList.remove('border-transparent', 'text-neutral-400');
  const target = document.getElementById(tabId);
  target.classList.add('active');
  target.style.display = '';

  // Lazy-load content when a tab is first opened
  if (tabId === 'expInbox') loadInbox();
  if (tabId === 'expMedia') loadMedia();
  if (tabId === 'expContacts' && !contactsLoadedOnce) loadContacts();
}

// ── Inbox (conversation list) ──
let inboxCache = [];
let currentConv = null;

function fmtTime(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '';
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString();
}

async function loadInbox() {
  const list = document.getElementById('inboxList');
  if (!list) return;
  list.innerHTML = '<div class="text-neutral-400 py-6 text-center text-sm">Loading…</div>';
  const enrich = document.getElementById('inboxEnrich')?.checked;
  try {
    const data = await api(`/inbox?limit=100${enrich ? '&enrich=true' : ''}`);
    const convs = data.result || data || [];
    inboxCache = convs;
    if (!convs.length) {
      list.innerHTML = '<div class="text-neutral-400 py-8 text-center text-sm">No conversations yet — messages you send or receive will appear here.</div>';
      return;
    }
    list.innerHTML = convs
      .map((c, i) => {
        const lm = c.lastMessage || {};
        const avatar = c.profilePicture
          ? `<img src="${c.profilePicture}" class="w-10 h-10 rounded-full object-cover flex-shrink-0" />`
          : `<div class="w-10 h-10 rounded-full bg-neutral-200 dark:bg-neutral-800 flex items-center justify-center text-neutral-500 flex-shrink-0"><span class="material-symbols-outlined text-[20px]">${c.isGroup ? 'group' : 'person'}</span></div>`;
        return `<div class="flex items-center gap-3 p-2.5 rounded hover:bg-neutral-50 dark:hover:bg-neutral-900 cursor-pointer" onclick="openConversation(${i})">
          ${avatar}
          <div class="flex-1 min-w-0">
            <div class="flex justify-between gap-2">
              <p class="text-sm font-medium text-neutral-900 dark:text-white truncate">${escHtml(c.name)}</p>
              <span class="text-[11px] text-neutral-400 whitespace-nowrap">${fmtTime(lm.timestamp)}</span>
            </div>
            <p class="text-xs text-neutral-400 truncate">${lm.fromMe ? 'You: ' : ''}${escHtml(lm.body || '')}</p>
          </div>
        </div>`;
      })
      .join('');
  } catch (err) {
    list.innerHTML = `<div class="text-red-500 py-6 text-center text-sm">${escHtml(err.message)}</div>`;
  }
}

async function openConversation(idx) {
  const c = inboxCache[idx];
  if (!c) return;
  currentConv = { chat: c.chat, name: c.name, isGroup: c.isGroup, actions: [] };
  const detail = document.getElementById('conversationDetail');
  detail.innerHTML = '<div class="text-neutral-400 py-6 text-center text-sm">Loading…</div>';
  try {
    const [msgsRes, actsRes] = await Promise.all([
      api(`/conversation?chat=${encodeURIComponent(c.chat)}&limit=50`),
      api(`/conversation/actions?chat=${encodeURIComponent(c.chat)}`),
    ]);
    const msgs = msgsRes.result || msgsRes || [];
    currentConv.actions = (actsRes.result || actsRes || {}).actions || [];

    const msgsHtml = msgs
      .map(
        (m) => `<div class="flex ${m.type === 'sent' ? 'justify-end' : 'justify-start'}">
          <div class="max-w-[80%] px-3 py-1.5 rounded text-xs ${m.type === 'sent' ? 'bg-green-100 dark:bg-green-900/40' : 'bg-neutral-100 dark:bg-neutral-800'}">
            ${escHtml(m.body || '')}
            <div class="text-[10px] text-neutral-400 mt-0.5">${fmtTime(m.timestamp)}</div>
          </div>
        </div>`
      )
      .join('');

    const actsHtml = currentConv.actions
      .map(
        (a, i) =>
          `<button class="px-2.5 py-1 rounded border border-neutral-200 dark:border-neutral-700 text-xs hover:bg-neutral-50 dark:hover:bg-neutral-900" onclick="convAction(${i})">${escHtml(a.label)}</button>`
      )
      .join('');

    detail.innerHTML = `<div class="flex items-center justify-between mb-2 gap-2">
        <p class="text-sm font-semibold text-neutral-900 dark:text-white truncate">${escHtml(c.name)}</p>
        <span class="text-[10px] px-2 py-0.5 rounded bg-neutral-100 dark:bg-neutral-800 text-neutral-500 whitespace-nowrap">${c.isGroup ? 'group' : 'contact'}</span>
      </div>
      ${c.about ? `<p class="text-xs text-neutral-400 mb-2">${escHtml(c.about)}</p>` : ''}
      <div class="flex flex-wrap gap-1.5 mb-3">${actsHtml}</div>
      <div class="space-y-1.5 max-h-[300px] overflow-y-auto">${msgsHtml || '<p class="text-xs text-neutral-400 text-center py-4">No messages stored for this conversation</p>'}</div>`;
  } catch (err) {
    detail.innerHTML = `<div class="text-red-500 py-6 text-center text-sm">${escHtml(err.message)}</div>`;
  }
}

async function convAction(idx) {
  if (!currentConv) return;
  const a = currentConv.actions[idx];
  if (!a) return;
  const body = { chat: currentConv.chat, action: a.id };
  for (const p of a.params || []) {
    if (p === 'participants') {
      const v = prompt('Participants (comma-separated phone numbers):');
      if (v === null) return;
      body.participants = v.split(',').map((s) => s.trim()).filter(Boolean);
    } else if (p === 'announce' || p === 'locked' || p === 'reset') {
      body[p] = confirm(`Set "${p}" to true?  (OK = true, Cancel = false)`);
    } else if (p === 'image') {
      const v = prompt('Image URL, data URI or base64:');
      if (v === null) return;
      body.image = v;
    } else {
      const v = prompt(`Enter ${p}:`);
      if (v === null) return;
      body[p] = v;
    }
  }
  if (a.id === 'exit' && !confirm('Leave this group?')) return;
  try {
    const res = await api('/conversation/action', { method: 'POST', body: JSON.stringify(body) });
    toast(`${a.label} ✓`, 'success');
    if (a.id === 'invite_link') {
      const link = (res.result || {}).result?.link || (res.result || {}).link;
      if (link) prompt('Invite link:', link);
    }
    if (a.id === 'exit' || a.id === 'leave') {
      document.getElementById('conversationDetail').innerHTML =
        '<div class="text-neutral-400 py-8 text-center text-sm">Left the group</div>';
      loadInbox();
    }
  } catch (err) {
    toast(err.message, 'error');
  }
}

// ── Live Activity feed ──
let activityEntries = [];
let contactsLoadedOnce = false;

function setWsState(state) {
  const el = document.getElementById('wsState');
  if (!el) return;
  const map = {
    connected: ['live ●', '#31cb00'],
    disconnected: ['offline ●', '#e63946'],
    connecting: ['connecting…', '#f1d302'],
  };
  const [label, color] = map[state] || map.connecting;
  el.textContent = label;
  el.style.color = color;
}

function activitySummary(event, data) {
  try {
    switch (event) {
      case 'message':
        return `${data.type === 'sent' ? '→' : '←'} ${data.contactName || data.from || ''}: ${data.body || ''}`;
      case 'status':
        return `connection: ${data.status}${data.info?.pushname ? ' (' + data.info.pushname + ')' : ''}`;
      case 'receipt':
        return `receipt ${data.type || 'delivered'} from ${data.sender || data.chat}`;
      case 'presence':
        return `${data.from} is ${data.unavailable ? 'offline' : 'online'}`;
      case 'chat_presence':
        return `${data.sender} ${data.state}${data.media === 'audio' ? ' (audio)' : ''}`;
      case 'picture':
        return `${data.jid} ${data.removed ? 'removed' : 'changed'} picture`;
      case 'group_info':
        return `group ${data.jid} updated`;
      case 'joined_group':
        return `joined group ${data.name || data.jid || ''}`;
      case 'call':
        return `call from ${data.from}`;
      case 'pair_success':
        return `paired as ${data.jid}`;
      default:
        return JSON.stringify(data);
    }
  } catch {
    return event;
  }
}

function pushActivity(event, data) {
  const time = new Date().toLocaleTimeString();
  activityEntries.unshift({ time, event, text: activitySummary(event, data || {}) });
  if (activityEntries.length > 100) activityEntries.pop();
  renderActivity();
}

function renderActivity() {
  const box = document.getElementById('activityLog');
  if (!box) return;
  if (!activityEntries.length) {
    box.innerHTML = '<div class="text-neutral-400 dark:text-neutral-600 py-8 text-center">Waiting for live events…</div>';
    return;
  }
  const colors = {
    message: '#31cb00', status: '#3a86ff', receipt: '#8d99ae', presence: '#f1d302',
    chat_presence: '#f1d302', call: '#e63946', picture: '#9b5de5', group_info: '#00bbf9',
    joined_group: '#00bbf9', pair_success: '#31cb00',
  };
  box.innerHTML = activityEntries
    .map(
      (e) => `<div class="flex gap-2 py-1 border-b border-border-light dark:border-border-dark/50">
        <span class="text-neutral-400 dark:text-neutral-600">${escHtml(e.time)}</span>
        <span class="font-semibold" style="color:${colors[e.event] || '#8d99ae'}">${escHtml(e.event)}</span>
        <span class="text-neutral-600 dark:text-neutral-300 truncate">${escHtml(e.text)}</span>
      </div>`
    )
    .join('');
}

function clearActivity() {
  activityEntries = [];
  renderActivity();
}

// ── Media ──
async function loadMedia() {
  const list = document.getElementById('mediaList');
  if (!list) return;
  try {
    const msgs = await api('/messages?limit=200');
    const media = (msgs || []).filter((m) => m.hasMedia);
    if (!media.length) {
      list.innerHTML = '<div class="text-neutral-400 dark:text-neutral-600 py-8 text-center text-sm">No media received yet</div>';
      return;
    }
    const icon = { image: 'image', video: 'movie', audio: 'mic', document: 'description', sticker: 'sentiment_satisfied' };
    list.innerHTML = media
      .map(
        (m) => `<div class="flex items-center gap-3 p-3 rounded border border-border-light dark:border-border-dark">
          <span class="material-symbols-outlined text-neutral-400">${icon[m.mediaType] || 'attachment'}</span>
          <div class="flex-1 min-w-0">
            <p class="text-sm text-neutral-900 dark:text-white truncate">${escHtml(m.body || m.mediaType)}</p>
            <p class="text-xs text-neutral-400 truncate">${escHtml(m.contactName || m.from || '')} · ${escHtml(m.mediaType)}</p>
          </div>
          <button class="px-3 py-1.5 rounded bg-neutral-900 dark:bg-white text-white dark:text-neutral-900 text-xs font-medium" onclick="downloadMedia('${escHtml(m.id)}')">View / Download</button>
        </div>`
      )
      .join('');
  } catch (err) {
    list.innerHTML = `<div class="text-red-500 py-6 text-center text-sm">${escHtml(err.message)}</div>`;
  }
}

async function downloadMedia(messageId) {
  const preview = document.getElementById('mediaPreview');
  preview.innerHTML = '<div class="text-xs text-neutral-400 py-2">Downloading…</div>';
  try {
    const data = await api(`/download-media?messageId=${encodeURIComponent(messageId)}`);
    const r = data.result || data;
    let body = '';
    if (r.mediaType === 'image' || r.mediaType === 'sticker') {
      body = `<img src="${r.dataUri}" class="max-h-64 rounded border border-border-light dark:border-border-dark" />`;
    } else if (r.mediaType === 'video') {
      body = `<video src="${r.dataUri}" controls class="max-h-64 rounded"></video>`;
    } else if (r.mediaType === 'audio') {
      body = `<audio src="${r.dataUri}" controls></audio>`;
    }
    preview.innerHTML = `<div class="p-3 rounded border border-border-light dark:border-border-dark">
      ${body}
      <div class="mt-2 flex items-center gap-3">
        <a href="${r.dataUri}" download="${escHtml(r.filename)}" class="text-xs px-3 py-1.5 rounded bg-neutral-900 dark:bg-white text-white dark:text-neutral-900 font-medium">Download ${escHtml(r.filename)}</a>
        <span class="text-xs text-neutral-400">${escHtml(r.mimetype)} · ${(r.size / 1024).toFixed(1)} KB</span>
      </div>
    </div>`;
  } catch (err) {
    preview.innerHTML = `<div class="text-red-500 text-sm py-2">${escHtml(err.message)}</div>`;
    toast(err.message, 'error');
  }
}

// ── Contacts ──
async function loadContacts() {
  const list = document.getElementById('contactsList');
  if (!list) return;
  list.innerHTML = '<div class="text-neutral-400 py-6 text-center text-sm">Loading…</div>';
  try {
    const data = await api('/contacts');
    const contacts = (data.result || data || []).filter((c) => c.fullName || c.pushName || c.firstName);
    contactsLoadedOnce = true;
    if (!contacts.length) {
      list.innerHTML = '<div class="text-neutral-400 py-6 text-center text-sm">No contacts found</div>';
      return;
    }
    list.innerHTML = contacts
      .map(
        (c) => `<div class="flex items-center justify-between p-2.5 rounded hover:bg-neutral-50 dark:hover:bg-neutral-900">
          <div class="min-w-0">
            <p class="text-sm text-neutral-900 dark:text-white truncate">${escHtml(c.fullName || c.pushName || c.firstName || 'Unknown')}</p>
            <p class="text-xs text-neutral-400 font-mono truncate">${escHtml((c.jid || '').split('@')[0])}</p>
          </div>
          ${c.businessName ? '<span class="text-[10px] px-2 py-0.5 rounded bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-400">business</span>' : ''}
        </div>`
      )
      .join('');
  } catch (err) {
    list.innerHTML = `<div class="text-red-500 py-6 text-center text-sm">${escHtml(err.message)}</div>`;
  }
}

// ── User lookup ──
function renderLookup(title, obj) {
  const box = document.getElementById('lookupResult');
  box.innerHTML = `<div class="p-4 rounded border border-border-light dark:border-border-dark">
    <p class="text-xs font-semibold uppercase tracking-wider text-neutral-500 mb-2">${escHtml(title)}</p>
    <pre class="text-xs overflow-x-auto whitespace-pre-wrap break-words text-neutral-700 dark:text-neutral-300">${escHtml(JSON.stringify(obj, null, 2))}</pre>
  </div>`;
}

function lookupTarget() {
  const v = document.getElementById('lookupInput').value.trim();
  if (!v) toast('Enter a phone number or JID', 'error');
  return v;
}

async function lookupCheck() {
  const t = lookupTarget();
  if (!t) return;
  try {
    const data = await api('/check-number', { method: 'POST', body: JSON.stringify({ numbers: [t] }) });
    renderLookup('On WhatsApp', data.result || data);
  } catch (err) { toast(err.message, 'error'); }
}

async function lookupInfo() {
  const t = lookupTarget();
  if (!t) return;
  try {
    const data = await api(`/user-info?jids=${encodeURIComponent(t)}`);
    renderLookup('User info', data.result || data);
  } catch (err) { toast(err.message, 'error'); }
}

async function lookupPicture() {
  const t = lookupTarget();
  if (!t) return;
  try {
    const data = await api(`/profile-picture?jid=${encodeURIComponent(t)}`);
    const r = data.result || data;
    const box = document.getElementById('lookupResult');
    box.innerHTML = r.url
      ? `<div class="p-4 rounded border border-border-light dark:border-border-dark text-center">
           <img src="${r.url}" class="w-32 h-32 rounded-full mx-auto object-cover" />
           <p class="text-xs text-neutral-400 mt-2">${escHtml(r.jid || '')}</p>
         </div>`
      : '<div class="p-4 text-sm text-neutral-400">No profile picture available (or hidden by privacy settings).</div>';
  } catch (err) { toast(err.message, 'error'); }
}

async function lookupBusiness() {
  const t = lookupTarget();
  if (!t) return;
  try {
    const data = await api(`/business-profile?jid=${encodeURIComponent(t)}`);
    renderLookup('Business profile', data.result || data || 'Not a business account');
  } catch (err) { toast(err.message, 'error'); }
}

// ── Misc ──
function showMiscOutput(obj) {
  const el = document.getElementById('miscOutput');
  el.classList.remove('hidden');
  el.textContent = JSON.stringify(obj, null, 2);
}

async function miscSetStatus() {
  const status = document.getElementById('miscStatus').value;
  try {
    await api('/set-status', { method: 'POST', body: JSON.stringify({ status }) });
    toast('Status updated', 'success');
  } catch (err) { toast(err.message, 'error'); }
}

async function miscPresence(presence) {
  try {
    await api('/presence', { method: 'POST', body: JSON.stringify({ presence }) });
    toast(`Presence set to ${presence}`, 'success');
  } catch (err) { toast(err.message, 'error'); }
}

async function miscBlock(block) {
  const jid = document.getElementById('miscBlockJid').value.trim();
  if (!jid) return toast('Enter a number or JID', 'error');
  try {
    await api(block ? '/block' : '/unblock', { method: 'POST', body: JSON.stringify({ jid }) });
    toast(block ? 'User blocked' : 'User unblocked', 'success');
    miscLoadBlocklist();
  } catch (err) { toast(err.message, 'error'); }
}

async function miscLoadBlocklist() {
  try {
    const data = await api('/blocklist');
    const r = data.result || data;
    const blocked = r.blocked || [];
    document.getElementById('miscBlocklist').textContent = blocked.length
      ? `Blocked: ${blocked.map((j) => j.split('@')[0]).join(', ')}`
      : 'No blocked users';
  } catch (err) { toast(err.message, 'error'); }
}

async function miscPrivacy() {
  try {
    const data = await api('/privacy-settings');
    showMiscOutput(data.result || data);
  } catch (err) { toast(err.message, 'error'); }
}

async function miscNewsletters() {
  try {
    const data = await api('/newsletters');
    showMiscOutput(data.result || data);
  } catch (err) { toast(err.message, 'error'); }
}

// ── Polling Control ──
// Polling stays as a safety net; the websocket above drives instant updates, so
// the intervals can be relaxed.
function startPolling() {
  stopPolling(); // clear any existing intervals
  pollStatus();
  pollStats();
  refreshMessages();
  refreshHooks();
  loadInbox();
  connectRealtime();
  pollingIntervals.push(setInterval(pollStatus, 30000));
  pollingIntervals.push(setInterval(pollStats, 60000));
  pollingIntervals.push(setInterval(refreshMessages, 30000));
  pollingIntervals.push(setInterval(refreshHooks, 60000));
}

function stopPolling() {
  pollingIntervals.forEach((id) => clearInterval(id));
  pollingIntervals = [];
  disconnectRealtime();
}

// ── Init ──
document.addEventListener('DOMContentLoaded', async () => {
  // Check if user has a stored token
  if (authToken) {
    try {
      const base = getApiBase();
      const res = await fetch(`${base}/auth/me`, {
        headers: { 'Authorization': `Bearer ${authToken}` },
      });
      if (res.ok) {
        const user = await res.json();
        showDashboard(user);
        return;
      }
    } catch {}
    // Token invalid — clear and show login
    localStorage.removeItem('wa_token');
    authToken = null;
  }

  // No valid token — check if any users exist
  try {
    const data = await authApi('/check');
    if (!data.hasUsers) {
      showAuthView('register');
    } else {
      showAuthView('login');
    }
  } catch {
    showAuthView('login');
  }
});
