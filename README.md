<div align="center">

# 💬 WA Bot Server

**A powerful WhatsApp Bot API with a premium web dashboard, webhook system, and Cloudflare Tunnel integration.**

[![Runtime](https://img.shields.io/badge/runtime-Bun-f472b6?style=for-the-badge&logo=bun&logoColor=white)](https://bun.sh)
[![WhatsApp](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://web.whatsapp.com)
[![Express](https://img.shields.io/badge/Express-000000?style=for-the-badge&logo=express&logoColor=white)](https://expressjs.com)
[![Cloudflare](https://img.shields.io/badge/Cloudflare_Tunnel-F38020?style=for-the-badge&logo=cloudflare&logoColor=white)](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
[![License](https://img.shields.io/badge/license-ISC-blue?style=for-the-badge)](LICENSE)

---

_Send messages • Manage groups • Receive webhooks • Beautiful dashboard_

</div>

---

## ✨ Features

| Feature                    | Description                                                     |
| -------------------------- | --------------------------------------------------------------- |
| 📨 **Send Messages**       | Send text messages to any phone number or group                 |
| 👥 **Group Management**    | Join, leave, and add members to groups                          |
| 🔔 **Webhooks**            | Register webhook URLs to receive incoming messages in real-time |
| 📊 **Live Dashboard**      | Premium dark glassmorphism UI with live stats and message log   |
| 📖 **API Docs**            | Built-in interactive API documentation page                     |
| 💾 **JSON Persistence**    | Messages, stats, and webhooks saved to `data.json`              |
| ☁️ **Cloudflare Tunnel**   | Auto-generates a public URL or use your own custom domain       |
| 🔐 **Session Persistence** | WhatsApp session saved locally — scan QR once                   |

---

## 🚀 Quick Start

### Prerequisites

- [Bun](https://bun.sh) (runtime)
- [Google Chrome](https://www.google.com/chrome/) (for WhatsApp Web automation)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-username/wa-server.git
cd wa-server

# Install dependencies
bun install
```

### Start the Server

```bash
bun run start
```

Open **http://localhost:3000** in your browser. Scan the QR code with WhatsApp to link the bot.

---

## 📡 API Reference

> Full interactive documentation available at **http://localhost:3000/docs.html**

### Messaging

| Method | Endpoint                  | Description                    |
| ------ | ------------------------- | ------------------------------ |
| `POST` | `/api/send-message`       | Send message to a phone number |
| `POST` | `/api/send-group-message` | Send message to a group        |
| `GET`  | `/api/messages`           | Get recent message log         |

### Groups

| Method | Endpoint            | Description                 |
| ------ | ------------------- | --------------------------- |
| `GET`  | `/api/groups`       | List all joined groups      |
| `POST` | `/api/join-group`   | Join group via invite link  |
| `POST` | `/api/leave-group`  | Leave a group               |
| `POST` | `/api/add-to-group` | Add participants to a group |

### Webhooks

| Method   | Endpoint                | Description              |
| -------- | ----------------------- | ------------------------ |
| `GET`    | `/api/hooks`            | List registered webhooks |
| `POST`   | `/api/hooks/register`   | Register a webhook URL   |
| `DELETE` | `/api/hooks/unregister` | Remove a webhook         |

### System

| Method | Endpoint      | Description                     |
| ------ | ------------- | ------------------------------- |
| `GET`  | `/api/status` | Bot connection status + QR code |
| `GET`  | `/api/stats`  | Dashboard statistics            |
| `GET`  | `/api/tunnel` | Cloudflare tunnel URL           |

### Example: Send a Message

```bash
curl -X POST http://localhost:3000/api/send-message \
  -H "Content-Type: application/json" \
  -d '{
    "number": "254712345678",
    "message": "Hello from the bot! 🤖"
  }'
```

### Example: Register a Webhook

```bash
curl -X POST http://localhost:3000/api/hooks/register \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://your-app.com/webhook",
    "name": "My App"
  }'
```

**Webhook payload** (POSTed to your URL on each incoming message):

```json
{
  "id": "false_254712345678@c.us_3EB0...",
  "from": "254712345678@c.us",
  "body": "Hello!",
  "timestamp": "2026-02-21T12:00:00.000Z",
  "type": "received",
  "contactName": "John",
  "isGroup": false
}
```

---

## ☁️ Custom Domain with Cloudflare

Set up a permanent custom domain instead of random tunnel URLs:

```bash
# Interactive setup wizard
bun run setup:cloudflare

# Start server with your custom domain
bun run start:tunnel
```

**Requirements:**

- A Cloudflare account with a domain added
- An API Token with **Cloudflare Tunnel (Edit)** and **DNS (Edit)** permissions
- Your Account ID (found on the Cloudflare dashboard)

The script creates a named tunnel, configures ingress rules, and sets up the DNS CNAME record automatically.

---

## 📁 Project Structure

```
wa-server/
├── index.js                 # Express server + tunnel setup
├── wa-client.js             # WhatsApp client wrapper
├── db.js                    # JSON persistence layer
├── setup-cloudflare.js      # Cloudflare domain setup script
├── data.json                # Auto-generated data store
├── cloudflare-config.json   # Auto-generated tunnel config
├── routes/
│   ├── api.js               # REST API endpoints
│   └── hooks.js             # Webhook management
└── public/
    ├── index.html           # Dashboard UI
    ├── docs.html            # API documentation page
    ├── style.css            # Dark glassmorphism theme
    └── app.js               # Client-side JavaScript
```

---

## 🛠️ Scripts

| Command                    | Description                             |
| -------------------------- | --------------------------------------- |
| `bun run start`            | Start the server                        |
| `bun run dev`              | Start with hot reload                   |
| `bun run setup:cloudflare` | Configure custom Cloudflare domain      |
| `bun run start:tunnel`     | Start with named tunnel (custom domain) |

---

## 🔧 Configuration

The server stores its configuration in `data.json`, which includes:

- **config** — Server port and settings
- **messages** — Message log (capped at 500)
- **webhooks** — Registered webhook URLs
- **stats** — Message counts, group joins/leaves

---

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the ISC License.

---

<div align="center">

**Built with ❤️ using Bun + Express + whatsapp-web.js**

</div>
