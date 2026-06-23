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

| Feature                   | Description                                                              |
| ------------------------- | ------------------------------------------------------------------------ |
| ⚡ **Go-powered Backend** | Rewritten completely in Go for ultra-low memory and extreme speed        |
| 📨 **Rich Messaging**     | Text, image, video, audio/voice, documents, stickers, location, contacts, polls |
| ✏️ **Message Actions**    | Reply, react, edit, delete-for-everyone, mark-as-read, typing indicators |
| 👥 **Full Group Mgmt**    | Create, rename, topic, photo, announce/lock, invites, join-requests, communities |
| 📇 **Contacts & Profile** | Check numbers, user/business info, profile pictures, block list, privacy |
| 📰 **Channels**           | Create, follow, mute and inspect WhatsApp channels (newsletters)         |
| 🔌 **Realtime WebSocket** | Live `/ws` event stream (messages, status, receipts, presence, calls)    |
| 📱 **Phone Pairing**      | Link WhatsApp accounts directly via Phone Number (no QR needed)          |
| 🔔 **Webhooks**           | Register webhook URLs to receive incoming messages in real-time          |
| 📊 **Live Dashboard**     | Dark UI with live activity feed, media viewer, contacts, user lookup &amp; misc tools |
| 🔐 **Multi-user Auth**    | Full user registration, encrypted tokens, and login system               |
| 💾 **JSON & SQLite**      | Uses high-performance pure Go SQLite (`glebarez/sqlite`)                 |

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

#### Inbox & conversations

A WhatsApp/email-style view built from the message log: the inbox groups messages by
conversation and surfaces each one's most recent message, newest first.

| Method | Endpoint                     | Body / Query                                    | Description                                              |
| ------ | ---------------------------- | ----------------------------------------------- | -------------------------------------------------------- |
| `GET`  | `/api/inbox`                 | `?enrich=true&limit=`                            | Conversation list + last message. `enrich` adds profile picture, resolved name &amp; about/topic |
| `GET`  | `/api/conversation`          | `?chat=&limit=`                                  | Messages of one conversation (oldest first)              |
| `GET`  | `/api/conversation/actions`  | `?chat=`                                         | Actions available for a contact/group (descriptor)       |
| `POST` | `/api/conversation/action`   | `chat`, `action`, …action params                | Run an action against the contact/group                  |

**Actions** — contacts: `block`, `unblock`, `subscribe_presence`. Groups: `exit`,
`set_name` (`name`), `set_description` (`description`), `set_photo` (`image`),
`announce` (`announce`), `lock` (`locked`), `invite_link` (`reset`),
`add`/`remove`/`promote`/`demote` (`participants[]`). Example:

```bash
curl -X POST http://localhost:3000/api/conversation/action \
  -H "Authorization: Bearer YOUR_TOKEN" -H "Content-Type: application/json" \
  -d '{"chat":"12345-67890@g.us","action":"set_name","name":"New group name"}'
```

> Realtime new messages still arrive over the WebSocket (`message` events); the inbox /
> conversation endpoints are for the cached history and the conversation list itself.

#### Messaging & media

| Method | Endpoint                  | Body / Query                                                  | Description                          |
| ------ | ------------------------- | ------------------------------------------------------------ | ------------------------------------ |
| `POST` | `/api/send-message`       | `number`, `message`                                          | Send a text message to a number      |
| `POST` | `/api/send-group-message` | `groupId`, `message`                                         | Send a text message to a group       |
| `POST` | `/api/send-image`         | `to`, `image` (url/data-uri/base64), `caption`              | Send an image                        |
| `POST` | `/api/send-video`         | `to`, `video`, `caption`, `gifPlayback`                     | Send a video / GIF                   |
| `POST` | `/api/send-audio`         | `to`, `audio`, `ptt`                                        | Send audio (`ptt:true` = voice note) |
| `POST` | `/api/send-document`      | `to`, `document`, `filename`, `caption`, `mimetype`        | Send a file as a document            |
| `POST` | `/api/send-sticker`       | `to`, `sticker`                                            | Send a sticker (WebP)                |
| `POST` | `/api/send-location`      | `to`, `latitude`, `longitude`, `name`, `address`          | Send a location pin                  |
| `POST` | `/api/send-contact`       | `to`, `displayName`, `phone` _or_ `vcard`                 | Send a contact card                  |
| `POST` | `/api/send-poll`          | `to`, `name`, `options[]`, `selectableCount`              | Send a poll                          |
| `POST` | `/api/reply-message`      | `to`, `messageId`, `participant`, `text`, `quotedText`    | Reply quoting a message              |
| `POST` | `/api/send-reaction`      | `to`, `messageId`, `participant`, `emoji`, `fromMe`       | React (empty emoji removes it)       |
| `POST` | `/api/edit-message`       | `to`, `messageId`, `newText`                              | Edit one of your messages            |
| `POST` | `/api/revoke-message`     | `to`, `messageId`, `participant`, `fromMe`                | Delete a message for everyone        |
| `POST` | `/api/mark-read`          | `chat`, `sender`, `messageIds[]`                          | Mark messages as read (blue ticks)   |
| `POST` | `/api/presence`           | `presence` (`available`/`unavailable`)                    | Set online / offline                 |
| `POST` | `/api/chat-presence`      | `to`, `state` (`composing`/`paused`), `media`             | Typing / recording indicator         |
| `POST` | `/api/subscribe-presence` | `jid`                                                     | Subscribe to a user's presence       |
| `GET`  | `/api/download-media`     | `?messageId=`                                              | Download media from a received message |

#### Groups & communities

| Method | Endpoint                     | Body / Query                              | Description                          |
| ------ | ---------------------------- | ----------------------------------------- | ------------------------------------ |
| `GET`  | `/api/groups`                | —                                         | List joined groups                   |
| `GET`  | `/api/group-info`            | `?groupId=`                               | Full metadata for a group            |
| `POST` | `/api/create-group`          | `name`, `participants[]`                  | Create a group                       |
| `POST` | `/api/join-group`            | `inviteLink`                              | Join via invite link                 |
| `POST` | `/api/leave-group`           | `groupId`                                 | Leave a group                        |
| `POST` | `/api/add-to-group`          | `groupId`, `participants[]`               | Add participants (legacy)            |
| `POST` | `/api/group/participants`    | `groupId`, `action`, `participants[]`     | add / remove / promote / demote      |
| `POST` | `/api/group/name`            | `groupId`, `name`                         | Rename a group                       |
| `POST` | `/api/group/topic`           | `groupId`, `topic`                        | Set group description                |
| `POST` | `/api/group/photo`           | `groupId`, `image`                        | Set / remove group picture           |
| `POST` | `/api/group/announce`        | `groupId`, `announce`                     | Admins-only messaging                |
| `POST` | `/api/group/locked`          | `groupId`, `locked`                       | Admins-only info editing             |
| `GET`  | `/api/group/invite-link`     | `?groupId=&reset=`                        | Get / revoke invite link             |
| `GET`  | `/api/group/info-from-link`  | `?link=`                                  | Resolve a link without joining       |
| `GET`  | `/api/group/join-requests`   | `?groupId=`                               | List pending join requests           |
| `POST` | `/api/group/join-requests`   | `groupId`, `action`, `participants[]`     | approve / reject join requests       |
| `POST` | `/api/group/member-add-mode` | `groupId`, `mode`                         | Who may add members                  |
| `POST` | `/api/group/disappearing`    | `groupId`, `seconds`                      | Disappearing-message timer           |
| `GET`  | `/api/group/subgroups`       | `?communityId=`                           | List community subgroups             |
| `POST` | `/api/group/link`            | `parent`, `child`                         | Link a group into a community        |
| `POST` | `/api/group/unlink`          | `parent`, `child`                         | Unlink a group from a community      |

#### Contacts, profile & privacy

| Method | Endpoint                  | Body / Query              | Description                          |
| ------ | ------------------------- | ------------------------- | ------------------------------------ |
| `POST` | `/api/check-number`       | `numbers[]`               | Check if numbers are on WhatsApp     |
| `GET`  | `/api/user-info`          | `?jids=` (csv)            | Status / picture id / devices        |
| `GET`  | `/api/profile-picture`    | `?jid=&preview=`          | Profile picture URL                  |
| `GET`  | `/api/business-profile`   | `?jid=`                   | Business profile details             |
| `GET`  | `/api/user-devices`       | `?jids=` (csv)            | Device JIDs for users                |
| `GET`  | `/api/contacts`           | —                         | Local contact cache                  |
| `GET`  | `/api/blocklist`          | —                         | List blocked users                   |
| `POST` | `/api/block`              | `jid`                     | Block a user                         |
| `POST` | `/api/unblock`            | `jid`                     | Unblock a user                       |
| `GET`  | `/api/privacy-settings`   | —                         | Read privacy settings                |
| `POST` | `/api/set-status`         | `status`                  | Set your "about" text                |

#### Channels (newsletters)

| Method | Endpoint                            | Body / Query           | Description                |
| ------ | ----------------------------------- | ---------------------- | -------------------------- |
| `GET`  | `/api/newsletters`                  | —                      | List followed channels     |
| `GET`  | `/api/newsletter/info`              | `?jid=`                | Channel metadata           |
| `GET`  | `/api/newsletter/info-from-invite`  | `?key=`                | Resolve a channel invite   |
| `POST` | `/api/newsletter/create`            | `name`, `description`  | Create a channel           |
| `POST` | `/api/newsletter/follow`            | `jid`                  | Follow a channel           |
| `POST` | `/api/newsletter/unfollow`          | `jid`                  | Unfollow a channel         |
| `POST` | `/api/newsletter/mute`              | `jid`, `mute`          | Mute / unmute a channel    |

#### Session & webhooks

| Method   | Endpoint                | Description                       |
| -------- | ----------------------- | -------------------------------- |
| `GET`    | `/api/status`           | Connection status + QR code      |
| `GET`    | `/api/stats`            | Message / group counters         |
| `GET`    | `/api/messages`         | Recent message log               |
| `POST`   | `/api/reconnect`        | (Re)connect via `qr`/`pairing_code` |
| `POST`   | `/api/disconnect`       | Log out and disconnect           |
| `GET`    | `/api/hooks`            | List registered webhooks         |
| `POST`   | `/api/hooks/register`   | Register a webhook URL           |
| `DELETE` | `/api/hooks/unregister` | Remove a webhook                 |

> `to` / `participant` / `jid` accept a bare phone number (`254712345678`) or a full
> JID (`254712345678@s.whatsapp.net`, `12345-67890@g.us`, `123@newsletter`). Media
> fields accept an `https://` URL, a `data:` URI, or a raw base64 string.

### 🔌 Realtime WebSocket

Connect to **`/ws`** for a live push stream instead of polling. Authenticate with a
Bearer header, a `?token=` query param, or the `wa_token` cookie:

```js
const ws = new WebSocket(`ws://localhost:3000/ws?token=YOUR_TOKEN`);
ws.onmessage = (e) => {
  const { event, data, timestamp } = JSON.parse(e.data);
  console.log(event, data); // status | message | receipt | presence | ...
};
```

Every frame is a JSON envelope `{ "event": string, "data": any, "timestamp": string }`.
On connect the server immediately sends a `status` snapshot. Emitted events include:
`status`, `pair_success`, `message`, `receipt`, `presence`, `chat_presence`,
`group_info`, `joined_group`, `picture`, and `call`. The bundled dashboard uses this
stream for instant updates and falls back to polling if the socket drops.

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
├── main.go                  # Fiber web server entrypoint + /ws upgrade
├── api_helpers.go           # Shared route helpers (jsonResult, auth uid)
├── api_messaging.go         # Messaging / media / presence routes
├── api_groups.go            # Group & community routes
├── api_contacts.go          # Contacts / profile / privacy + newsletter routes
├── storage/
│   ├── auth.go              # User registration, bcrypt, and OTP handling
│   └── store.go             # JSON persistence handling (`data/users`)
├── whatsapp/
│   ├── client.go            # whatsmeow client, SQLite, & event broadcasting
│   ├── hub.go               # Realtime WebSocket hub (per-user fan-out)
│   ├── helpers.go           # JID parsing, media resolution, client guards
│   ├── messaging.go         # Send media/reactions/edits/presence
│   ├── groups.go            # Group & community operations
│   ├── contacts.go          # Contacts / profile / privacy / blocklist
│   └── newsletter.go        # Channel (newsletter) operations
├── nixpacks.toml            # Railway Go deployment configuration
├── .github/workflows/       # Automated CI build runner
└── public/
    ├── index.html           # Dashboard UI
    ├── docs.html            # API documentation page
    ├── style.css            # Dark glassmorphism theme
    └── app.js               # Client-side JS (+ realtime WebSocket client)
```

---

## 📄 License

This project is licensed under the ISC License.

---

<div align="center">

**Built with ❤️ using Go + Fiber + whatsmeow**

</div>
