const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const bcrypt = require('bcryptjs');

const AUTH_PATH = path.join(__dirname, 'data', 'auth.json');
const BCRYPT_ROUNDS = 12;
const MAX_OTP_ATTEMPTS = 5;
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

// ── Helpers ──

function ensureDir(dir) {
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
}

function loadAuth() {
  try {
    if (!fs.existsSync(AUTH_PATH)) return { users: [] };
    return JSON.parse(fs.readFileSync(AUTH_PATH, 'utf-8'));
  } catch {
    return { users: [] };
  }
}

function saveAuth(data) {
  ensureDir(path.dirname(AUTH_PATH));
  fs.writeFileSync(AUTH_PATH, JSON.stringify(data, null, 2), 'utf-8');
}

function generateToken() {
  return crypto.randomBytes(48).toString('hex');
}

function generateOtp() {
  return Math.floor(100000 + Math.random() * 900000).toString();
}

// ── Public API ──

function hasAnyUsers() {
  const auth = loadAuth();
  return auth.users.length > 0;
}

function findUserByEmail(email) {
  const auth = loadAuth();
  return auth.users.find((u) => u.email === email.toLowerCase().trim()) || null;
}

function findUserByToken(token) {
  if (!token || typeof token !== 'string') return null;
  const auth = loadAuth();
  // Use timing-safe comparison to prevent timing attacks
  const tokenBuf = Buffer.from(token);
  return auth.users.find((u) => {
    const storedBuf = Buffer.from(u.token);
    if (tokenBuf.length !== storedBuf.length) return false;
    return crypto.timingSafeEqual(tokenBuf, storedBuf);
  }) || null;
}

async function register(email, password) {
  if (!email || !password) {
    throw new Error('Email and password are required');
  }
  const normalized = email.toLowerCase().trim();

  if (!EMAIL_REGEX.test(normalized)) {
    throw new Error('Please enter a valid email address');
  }
  if (password.length < 6) {
    throw new Error('Password must be at least 6 characters');
  }

  const auth = loadAuth();
  if (auth.users.find((u) => u.email === normalized)) {
    throw new Error('An account with this email already exists');
  }

  const passwordHash = await bcrypt.hash(password, BCRYPT_ROUNDS);
  const token = generateToken();
  const id = crypto.randomUUID();

  const user = {
    id,
    email: normalized,
    passwordHash,
    token,
    resetOtp: null,
    resetOtpExpires: null,
    createdAt: new Date().toISOString(),
  };

  auth.users.push(user);
  saveAuth(auth);

  return { id, email: normalized, token };
}

async function login(email, password) {
  const normalized = email.toLowerCase().trim();

  if (!normalized || !password) {
    throw new Error('Email and password are required');
  }

  const auth = loadAuth();
  const user = auth.users.find((u) => u.email === normalized);
  if (!user) {
    throw new Error('Invalid email or password');
  }

  const valid = await bcrypt.compare(password, user.passwordHash);
  if (!valid) {
    throw new Error('Invalid email or password');
  }

  return { id: user.id, email: user.email, token: user.token };
}

async function forgotPassword(email) {
  const normalized = email.toLowerCase().trim();

  const auth = loadAuth();
  const user = auth.users.find((u) => u.email === normalized);
  if (!user) {
    // Don't reveal whether email exists — silently succeed
    return;
  }

  const otp = generateOtp();
  // Store OTP as a hash so it's not plaintext in auth.json
  user.resetOtpHash = crypto.createHash('sha256').update(otp).digest('hex');
  user.resetOtpExpires = Date.now() + 15 * 60 * 1000; // 15 minutes
  user.resetOtpAttempts = 0; // brute-force counter
  saveAuth(auth);

  // Log OTP to server console (no email service configured)
  console.log(`\n🔑 Password reset OTP for ${normalized}: ${otp}`);
  console.log(`   Valid for 15 minutes.\n`);
}

async function resetPassword(email, otp, newPassword) {
  const normalized = email.toLowerCase().trim();

  if (!otp || !newPassword) {
    throw new Error('OTP and new password are required');
  }
  if (newPassword.length < 6) {
    throw new Error('Password must be at least 6 characters');
  }

  const auth = loadAuth();
  const user = auth.users.find((u) => u.email === normalized);
  if (!user) {
    throw new Error('Invalid email or OTP');
  }

  // Check brute-force lockout
  if ((user.resetOtpAttempts || 0) >= MAX_OTP_ATTEMPTS) {
    user.resetOtpHash = null;
    user.resetOtpExpires = null;
    user.resetOtpAttempts = 0;
    saveAuth(auth);
    throw new Error('Too many attempts — please request a new reset code');
  }

  // Verify OTP via hash comparison
  const otpHash = crypto.createHash('sha256').update(otp).digest('hex');
  if (!user.resetOtpHash || user.resetOtpHash !== otpHash) {
    user.resetOtpAttempts = (user.resetOtpAttempts || 0) + 1;
    saveAuth(auth);
    throw new Error('Invalid email or OTP');
  }

  if (Date.now() > user.resetOtpExpires) {
    user.resetOtpHash = null;
    user.resetOtpExpires = null;
    user.resetOtpAttempts = 0;
    saveAuth(auth);
    throw new Error('OTP has expired — please request a new one');
  }

  // Re-hash + regenerate token (invalidates all existing sessions)
  user.passwordHash = await bcrypt.hash(newPassword, BCRYPT_ROUNDS);
  user.token = generateToken();
  user.resetOtpHash = null;
  user.resetOtpExpires = null;
  user.resetOtpAttempts = 0;
  saveAuth(auth);

  return { id: user.id, email: user.email, token: user.token };
}

function verifyToken(token) {
  return findUserByToken(token);
}

module.exports = {
  hasAnyUsers,
  findUserByEmail,
  findUserByToken,
  register,
  login,
  forgotPassword,
  resetPassword,
  verifyToken,
};
