<div align="center">

# 💬 WA Bot Server (Golang Edition)

**A high-performance WhatsApp Bot API powered by `whatsmeow` and `Fiber`, with a premium web dashboard and webhook system.**

[![Runtime](https://img.shields.io/badge/runtime-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![WhatsApp](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://web.whatsapp.com)
[![Fiber](https://img.shields.io/badge/Fiber-000000?style=for-the-badge&logo=go&logoColor=white)](https://gofiber.io)
[![License](https://img.shields.io/badge/license-ISC-blue?style=for-the-badge)](LICENSE)

---

_Send messages • Manage groups • Full Auth • Fast & Concurrent • Zero Chromium dependencies_

</div>

---

## ✨ Features

| Feature                   | Description                                                       |
| ------------------------- | ----------------------------------------------------------------- |
| ⚡ **Go-powered Backend** | Rewritten completely in Go for ultra-low memory and extreme speed |
| 📨 **Send Messages**      | Send text messages to any phone number or group                   |
| 👥 **Group Management**   | Join, leave, and add members to groups                            |
| 📱 **Phone Pairing**      | Link WhatsApp accounts directly via Phone Number (no QR needed)   |
| 🔔 **Webhooks**           | Register webhook URLs to receive incoming messages in real-time   |
| 📊 **Live Dashboard**     | Premium dark glassmorphism UI with live stats and message log     |
| 🔐 **Multi-user Auth**    | Full user registration, encrypted JWT tokens, and login system    |
| 💾 **JSON & SQLite**      | Uses high-performance pure Go SQLite (`glebarez/sqlite`)          |

---

## 🚀 Quick Start

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)

### Installation

```bash
# Clone the repository
git clone https://github.com/gideonsigilai/whatsapp-bot.git
cd whatsapp-bot

# Download dependencies
go mod download
```

### Start the Server

```bash
go build -o server.exe main.go
./server.exe
```

Open **http://localhost:3000** in your browser. Register an account and use the pairing code interface to quickly link your device.

---

## ☁️ Deployment (Railway / Cloudflare)

This repository includes a `nixpacks.toml` file to automatically deploy the Go application on platforms like [Railway](https://railway.app).

### ⚠️ IMPORTANT: Persistent Data

Because WhatsApp session tokens and user accounts are stored in the filesystem (`data/` directory), **your deploying platform MUST be configured with a Persistent Volume mounted to `/app/data`**.

**On Railway:**

1. Open your Service Settings.
2. Scroll down to **Volumes**.
3. Create a new Volume and set the **Mount Path** to `/app/data`.
4. Without this volume, every redeploy will wipe the `data/` folder and force users to re-scan WhatsApp!

---

## 📡 API Reference

> Full interactive documentation available at **http://localhost:3000/docs.html**

### Authentication

| Method | Endpoint         | Description                  |
| ------ | ---------------- | ---------------------------- |
| `POST` | `/auth/register` | Create new dashboard account |
| `POST` | `/auth/login`    | Log in to the dashboard      |

### WhatsApp Interaction

_Note: All `/api/*` endpoints require a Bearer token or `wa_token` cookie._

| Method   | Endpoint                  | Description                     |
| -------- | ------------------------- | ------------------------------- |
| `POST`   | `/api/send-message`       | Send message to a phone number  |
| `POST`   | `/api/send-group-message` | Send message to a group         |
| `GET`    | `/api/messages`           | Get recent message log          |
| `GET`    | `/api/groups`             | List all joined groups          |
| `POST`   | `/api/join-group`         | Join group via invite link      |
| `POST`   | `/api/leave-group`        | Leave a group                   |
| `POST`   | `/api/add-to-group`       | Add participants to a group     |
| `GET`    | `/api/hooks`              | List registered webhooks        |
| `POST`   | `/api/hooks/register`     | Register a webhook URL          |
| `DELETE` | `/api/hooks/unregister`   | Remove a webhook                |
| `GET`    | `/api/status`             | Bot connection status + QR code |
| `POST`   | `/api/reconnect`          | Disconnect/Restart connection   |

### Example: Send a Message

```bash
curl -X POST http://localhost:3000/api/send-message \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json" \
  -d '{
    "number": "254712345678",
    "message": "Hello from the new Go bot! 🤖"
  }'
```

---

## 📁 Project Structure

```
wa-server/
├── main.go                  # Fiber web server entrypoint
├── storage/
│   ├── auth.go              # User registration, bcrypt, and OTP handling
│   └── store.go             # JSON persistence handling (`data/users`)
├── whatsapp/
│   └── client.go            # whatsmeow client encapsulation, SQLite, & events
├── nicks.toml               # Railway Go deployment configuration
├── .github/workflows/       # Automated CI build runner
└── public/
    ├── index.html           # Dashboard UI
    ├── docs.html            # API documentation page
    ├── style.css            # Dark glassmorphism theme
    └── app.js               # Client-side JavaScript
```

---

## 📄 License

This project is licensed under the ISC License.

---

<div align="center">

**Built with ❤️ using Go + Fiber + whatsmeow**

</div>
