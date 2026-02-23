const express = require('express');
const router = express.Router();
const db = require('../db');

// GET /api/hooks — List registered webhooks
router.get('/', (req, res) => {
  const webhooks = db.getUser(req.user.id, 'webhooks') || [];
  res.json(webhooks);
});

// POST /api/hooks/register — Register a new webhook
router.post('/register', (req, res) => {
  const { url, name } = req.body;
  if (!url) {
    return res.status(400).json({ error: 'url is required' });
  }

  const webhooks = db.getUser(req.user.id, 'webhooks') || [];
  const exists = webhooks.find((w) => w.url === url);
  if (exists) {
    return res.status(409).json({ error: 'Webhook URL already registered' });
  }
 
  const webhook = {
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 7),
    url,
    name: name || url,
    createdAt: new Date().toISOString(),
  };

  db.pushToUser(req.user.id, 'webhooks', webhook);
  res.status(201).json({ success: true, webhook });
});

// DELETE /api/hooks/unregister — Remove a webhook
router.delete('/unregister', (req, res) => {
  const { id, url } = req.body;
  if (!id && !url) {
    return res.status(400).json({ error: 'id or url is required' });
  }

  db.removeFromUser(req.user.id, 'webhooks', (w) => (id ? w.id === id : w.url === url));
  res.json({ success: true });
});

module.exports = router;
