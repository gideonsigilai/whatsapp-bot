const express = require('express');
const bodyParser = require('body-parser');
const cookieParser = require('cookie-parser');
const path = require('path');
const fs = require('fs');
const apiRoutes = require('./routes/api');
const hookRoutes = require('./routes/hooks');
const authRoutes = require('./routes/auth');
const requireAuth = require('./middleware/auth');
const db = require('./db');

const app = express();
const PORT = process.env.PORT || db.get('config')?.port || 3000;

// Middleware
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');
  // Security headers
  res.setHeader('X-Content-Type-Options', 'nosniff');
  res.setHeader('X-Frame-Options', 'DENY');
  res.setHeader('X-XSS-Protection', '1; mode=block');
  if (req.method === 'OPTIONS') return res.sendStatus(204);
  next();
});
app.use(bodyParser.json({ limit: '1mb' }));
app.use(bodyParser.urlencoded({ extended: true, limit: '1mb' }));
app.use(cookieParser());

// Auth routes (public — no token needed)
app.use('/auth', authRoutes);

// Serve static dashboard (public files, auth handled client-side)
app.use(express.static(path.join(__dirname, 'public')));

// Protected API routes
app.use('/api', requireAuth, apiRoutes);
app.use('/api/hooks', requireAuth, hookRoutes);

// Start server
app.listen(PORT, () => {
  const publicUrl = process.env.RAILWAY_PUBLIC_DOMAIN
    ? `https://${process.env.RAILWAY_PUBLIC_DOMAIN}`
    : `http://localhost:${PORT}`;
  console.log(`\n🌐 Dashboard:  ${publicUrl}`);
  console.log(`📡 API:        ${publicUrl}/api`);
  console.log(`🔐 Auth:       ${publicUrl}/auth\n`);
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down...');
  process.exit(0);
});
