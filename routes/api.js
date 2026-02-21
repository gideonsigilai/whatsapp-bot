const express = require('express');
const router = express.Router();
const waClient = require('../wa-client');
const db = require('../db');

// GET /api/status — Bot connection status + QR code
router.get('/status', (req, res) => {
  res.json(waClient.getStatus());
});

// GET /api/stats — Dashboard statistics
router.get('/stats', (req, res) => {
  const stats = db.get('stats') || {};
  const webhooks = db.get('webhooks') || [];
  res.json({
    ...stats,
    webhookCount: webhooks.length,
  });
});

// GET /api/messages — Recent message log
router.get('/messages', (req, res) => {
  const messages = db.get('messages') || [];
  const limit = parseInt(req.query.limit) || 50;
  res.json(messages.slice(-limit).reverse());
});

// GET /api/groups — List all groups
router.get('/groups', async (req, res) => {
  try {
    const groups = await waClient.getGroups();
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
    const result = await waClient.sendMessage(number, message);
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
    const result = await waClient.sendGroupMessage(groupId, message);
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
    const result = await waClient.joinGroup(inviteLink);
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
    const result = await waClient.leaveGroup(groupId);
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
    const result = await waClient.addToGroup(groupId, participants);
    res.json({ success: true, result });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

module.exports = router;
