const auth = require('../auth');

/**
 * Express middleware — verifies auth token from header or cookie.
 * On success, sets req.user = { id, email }.
 * On failure, returns 401 (API) or redirects to login (browser).
 */
function requireAuth(req, res, next) {
  // 1. Try Authorization header: "Bearer <token>"
  let token = null;
  const header = req.headers.authorization;
  if (header && header.startsWith('Bearer ')) {
    token = header.slice(7);
  }

  // 2. Fallback to cookie
  if (!token && req.cookies) {
    token = req.cookies.wa_token;
  }

  if (!token) {
    return sendUnauthorized(req, res);
  }

  const user = auth.verifyToken(token);
  if (!user) {
    return sendUnauthorized(req, res);
  }

  req.user = { id: user.id, email: user.email };
  next();
}

function sendUnauthorized(req, res) {
  // API requests get JSON 401; browser requests redirect to login
  const isApi = req.path.startsWith('/api');
  if (isApi) {
    return res.status(401).json({ error: 'Unauthorized — please log in' });
  }
  return res.status(401).json({ error: 'Unauthorized' });
}

module.exports = requireAuth;
