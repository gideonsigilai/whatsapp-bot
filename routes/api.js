const express = require('express');
const router = express.Router();
const waClient = require('../wa-client');
const db = require('../db');

// GET /api/status — Bot connection status + QR code
router.get('/status', (req, res) => {
  res.json(waClient.getStatus(req.user.id));
});

// GET /api/stats — Dashboard statistics
router.get('/stats', (req, res) => {
  const stats = db.getUser(req.user.id, 'stats') || {};
  const webhooks = db.getUser(req.user.id, 'webhooks') || [];
  res.json({
    ...stats,
    webhookCount: webhooks.length,
  });
});

// GET /api/messages — Recent message log
router.get('/messages', (req, res) => {
  const messages = db.getUser(req.user.id, 'messages') || [];
  const limit = parseInt(req.query.limit) || 50;
  res.json(messages.slice(-limit).reverse());
});

// GET /api/groups — List all groups
router.get('/groups', async (req, res) => {
  try {
    const groups = await waClient.getGroups(req.user.id);
    res.json(groups);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/send-message — Send message to a number
router.post('/send-message', async (req, res) => {
  try {
    const { number, message } = req.body;
    if (!number || !message) {
      return res.status(400).json({ error: 'number and message are required' });
    }
    const result = await waClient.sendMessage(req.user.id, number, message);
    res.json({ success: true, message: result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/send-group-message — Send message to a group
router.post('/send-group-message', async (req, res) => {
  try {
    const { groupId, message } = req.body;
    if (!groupId || !message) {
      return res.status(400).json({ error: 'groupId and message are required' });
    }
    const result = await waClient.sendGroupMessage(req.user.id, groupId, message);
    res.json({ success: true, message: result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/join-group — Join a group via invite link
router.post('/join-group', async (req, res) => {
  try {
    const { inviteLink } = req.body;
    if (!inviteLink) {
      return res.status(400).json({ error: 'inviteLink is required' });
    }
    const result = await waClient.joinGroup(req.user.id, inviteLink);
    res.json({ success: true, result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/leave-group — Leave a group
router.post('/leave-group', async (req, res) => {
  try {
    const { groupId } = req.body;
    if (!groupId) {
      return res.status(400).json({ error: 'groupId is required' });
    }
    const result = await waClient.leaveGroup(req.user.id, groupId);
    res.json({ success: true, result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/add-to-group — Add participants to a group
router.post('/add-to-group', async (req, res) => {
  try {
    const { groupId, participants } = req.body;
    if (!groupId || !participants || !Array.isArray(participants)) {
      return res.status(400).json({
        error: 'groupId and participants (array) are required',
      });
    }
    const result = await waClient.addToGroup(req.user.id, groupId, participants);
    res.json({ success: true, result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/disconnect — Disconnect WhatsApp session
router.post('/disconnect', async (req, res) => {
  try {
    await waClient.disconnect(req.user.id);
    res.json({ success: true, message: 'WhatsApp disconnected' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// POST /api/reconnect — Reconnect WhatsApp (re-initialize client)
router.post('/reconnect', async (req, res) => {
  try {
    // Don't await — initialize is long-running, just kick it off
    waClient.reconnect(req.user.id).catch((err) => console.error('Reconnect error:', err.message));
    res.json({ success: true, message: 'Reconnecting... scan QR if prompted' });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

module.exports = router;
