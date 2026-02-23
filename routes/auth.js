const express = require('express');
const router = express.Router();
const auth = require('../auth');
const db = require('../db');

// GET /auth/check — Does at least one user exist?
router.get('/check', (req, res) => {
  res.json({ hasUsers: auth.hasAnyUsers() });
});

// POST /auth/register — Create a new account
router.post('/register', async (req, res) => {
  try {
    const { email, password } = req.body;
    const result = await auth.register(email, password);

    // Initialize per-user data container
    db.initUser(result.id);

    res.status(201).json({
      success: true,
      token: result.token,
      user: { id: result.id, email: result.email },
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// POST /auth/login — Authenticate
router.post('/login', async (req, res) => {
  try {
    const { email, password } = req.body;
    const result = await auth.login(email, password);
    res.json({
      success: true,
      token: result.token,
      user: { id: result.id, email: result.email },
    });
  } catch (err) {
    res.status(401).json({ error: err.message });
  }
});

// GET /auth/me — Current user info (requires valid token)
router.get('/me', (req, res) => {
  const header = req.headers.authorization;
  let token = null;
  if (header && header.startsWith('Bearer ')) {
    token = header.slice(7);
  }
  if (!token && req.cookies) {
    token = req.cookies.wa_token;
  }

  if (!token) {
    return res.status(401).json({ error: 'No token provided' });
  }

  const user = auth.verifyToken(token);
  if (!user) {
    return res.status(401).json({ error: 'Invalid or expired token' });
  }

  res.json({ id: user.id, email: user.email });
});

// POST /auth/forgot-password — Request a reset OTP
router.post('/forgot-password', async (req, res) => {
  try {
    const { email } = req.body;
    if (!email) return res.status(400).json({ error: 'Email is required' });
    await auth.forgotPassword(email);
    // Always respond with success (don't reveal if email exists)
    res.json({ success: true, message: 'If the email exists, a reset code has been sent.' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /auth/reset-password — Reset password with OTP
router.post('/reset-password', async (req, res) => {
  try {
    const { email, otp, newPassword } = req.body;
    const result = await auth.resetPassword(email, otp, newPassword);
    res.json({
      success: true,
      token: result.token,
      user: { id: result.id, email: result.email },
      message: 'Password reset successfully. You have been logged in with a new session.',
    });
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

module.exports = router;
