import express from 'express';
import { createProxyMiddleware } from 'http-proxy-middleware';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const app = express();
const PORT = process.env.PORT || 8080;
const ACS_HOST = process.env.ACS_HOST || 'localhost';
const ACS_PORT = process.env.ACS_PORT || '8443';
const DS_HOST = process.env.DS_HOST || 'localhost';
const DS_PORT = process.env.DS_PORT || '9443';
const IDENTITY_HOST = process.env.IDENTITY_HOST || 'localhost';
const IDENTITY_PORT = process.env.IDENTITY_PORT || '8444';
const SECRETS_HOST = process.env.SECRETS_HOST || 'localhost';
const SECRETS_PORT = process.env.SECRETS_PORT || '8445';
const acsTarget = `http://${ACS_HOST}:${ACS_PORT}`;
const dsTarget = `http://${DS_HOST}:${DS_PORT}`;
const identityTarget = `http://${IDENTITY_HOST}:${IDENTITY_PORT}`;
const secretsTarget = `http://${SECRETS_HOST}:${SECRETS_PORT}`;

// Proxy: Identity Server endpoints
app.use('/auth', createProxyMiddleware({ target: identityTarget, changeOrigin: true, pathRewrite: (path) => '/auth' + path }));

// Proxy: ACS endpoints
const proxyOpts = { target: acsTarget, changeOrigin: true };
app.use('/api', createProxyMiddleware({ ...proxyOpts, pathRewrite: (path) => '/api' + path }));
app.use('/v1', createProxyMiddleware({ ...proxyOpts, pathRewrite: (path) => '/v1' + path }));
app.use('/health', createProxyMiddleware(proxyOpts));

// Proxy: Secrets Manager
app.use('/secrets', createProxyMiddleware({ target: secretsTarget, changeOrigin: true, pathRewrite: (path) => '/secrets' + path }));

// Proxy: DS emulator API
app.use('/ds-api', createProxyMiddleware({ target: dsTarget, changeOrigin: true, pathRewrite: (path) => '/api' + path }));

// Serve static files from Vite build
app.use(express.static(join(__dirname, 'dist')));

// SPA fallback
app.get('*', (req, res) => {
  res.sendFile(join(__dirname, 'dist', 'index.html'));
});

app.listen(PORT, () => {
  console.log(`UI server running on :${PORT}, proxying ACS=${acsTarget}, DS=${dsTarget}, Identity=${identityTarget}`);
});
