const express = require('express');
const bodyParser = require('body-parser');
const path = require('path');
const fs = require('fs');
const waClient = require('./wa-client');
const apiRoutes = require('./routes/api');
const hookRoutes = require('./routes/hooks');
const db = require('./db');

const app = express();
const PORT = process.env.PORT || db.get('config')?.port || 3000;

// Middleware
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  if (req.method === 'OPTIONS') return res.sendStatus(204);
  next();
});
app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true }));

// Serve static dashboard
app.use(express.static(path.join(__dirname, 'public')));

// API routes
app.use('/api', apiRoutes);
app.use('/api/hooks', hookRoutes);



// Start server
app.listen(PORT, () => {
  const publicUrl = process.env.RAILWAY_PUBLIC_DOMAIN
    ? `https://${process.env.RAILWAY_PUBLIC_DOMAIN}`
    : `http://localhost:${PORT}`;
  console.log(`\n🌐 Dashboard:  ${publicUrl}`);
  console.log(`📡 API:        ${publicUrl}/api\n`);

  // Initialize WhatsApp client (async, won't block server)
  waClient.initialize().catch((err) => {
    console.error('⚠️  WhatsApp init failed:', err.message);
  });
});



// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down...');
  process.exit(0);
});
