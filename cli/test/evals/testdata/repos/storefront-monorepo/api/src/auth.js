// Resolves the session cookie to an account id.
function authenticate(pool) {
  return async (req, res, next) => {
    const sid = /(?:^|;\s*)sid=([^;]+)/.exec(req.headers.cookie ?? '')?.[1];
    if (!sid) return res.status(401).json({ error: 'sign in required' });
    const { rows } = await pool.query('SELECT account_id FROM sessions WHERE id = $1 AND expires_at > now()', [sid]);
    if (rows.length === 0) return res.status(401).json({ error: 'sign in required' });
    req.accountId = rows[0].account_id;
    next();
  };
}

module.exports = { authenticate };
