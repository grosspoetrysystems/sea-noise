---
date: 2026-04-04
source: ad-hoc
status: ready-to-build
tags:
- assertion
- business/strategy
- tech/ai-agents
title: PANDILLA — Character Studio Strategy
---

## Assertion

Build PANDILLA: a character studio that ships quiet, trustworthy AI personas (Mira, Marco, etc.) into messaging apps where real life already happens — WhatsApp first, multilingual from zero, radical privacy as architecture, no surveillance possible by design.

## Why

Soft technology that disappears into the fabric of daily life. WhatsApp is the world's largest messaging substrate (2B users), majority non-English, family-oriented. Owning nothing by design is the rarest competitive moat as regulation catches up with extraction-first apps. Character integrity across languages is the trust contract — Mira in Arabic must feel identical to Mira in English.

## MVP Scope (Party of One)

1. Provisioning API (Go) — self-serve without manual VPS setup
2. Stripe integration — payment before provisioning
3. React Native app — settings + push notifications only
4. Multi-tenant Radicale (one Fly Machine per group)
5. Mira Moments — temporary instances with natural death dates
6. Vision pipeline — printed docs first, handwritten second
7. Marco — V2, demand-driven after Mira is established

## Status

Ready to build (as of 2026-04-04).

## Evidence
- [[2026-04-17-what-will-be-scarce-alex-imas]] — Alex Imas on what becomes scarce in an AI-saturated world. Maps directly to PANDILLA's privacy-as-architecture moat and the "soft technology that disappears" thesis (stub awaiting hydration)
- [[2026-05-01-language-acquisition-essay]] — language acquisition methods. Directly relevant to character integrity across EN/ES/AR/ZH — Mira-in-Arabic must feel identical to Mira-in-English, which requires native-not-translated pipelines (stub awaiting hydration)
- [[2026-05-15-structured-prompt-driven-development]] — Thoughtworks SPDD: prompts as first-class versioned artifacts. REASONS Canvas as a possible governance schema for character consistency across languages — Mira's IDENTITY.md / SOUL.md / AGENTS.md scaffolding is exactly this pattern

## Source Document


## The Idea

We are not a SaaS company. We are a character studio.

We build quiet, trustworthy AI personas and deploy them into the messaging apps where real life already happens. We don't make platforms. We don't make dashboards. We don't ask anyone to change their behavior.

We make the people your family didn't know they were missing.

---

## The Studio Philosophy

**Soft technology.** The product disappears into the fabric of daily life. The character is present. The technology is not.

**Minification as strategy.** Every decision that adds surface area is the wrong decision. Every decision that removes it is correct. We cut until it hurts, then we cut a little more.

**We own nothing.** Not a positioning statement. An architectural constraint. We cannot surveil our users because we designed the system to have nothing to surveil. This becomes more valuable every year as regulation catches up with reality.

**Meet people where they are.** WhatsApp first. The largest messaging platform on earth, majority non-English, deeply family-oriented, already the place where real coordination happens. No new app. No new behavior. Just a new presence in an existing group.

**Character integrity above everything.** Mira in Arabic must feel identical to Mira in English. Marco in a Tokyo trip group must feel identical to Marco in a Paris bachelorette. The consistency is the trust. The trust is the product.

---

## The Three Bets

**1. WhatsApp as substrate, not channel.**
Not an integration. The actual place where families live. Two billion people. Zero onboarding friction. The agent joins the group like a person would.

**2. Radical privacy as architecture.**
We own nothing by design. In a world where every app extracts, this is the rarest thing: a service that genuinely cannot surveil you because it was built from the start to have nothing to surveil.

**3. Multilingual from zero, not from translation.**
English, Spanish, Arabic, Mandarin — covering roughly 90% of the world's WhatsApp users. Not translated. Native. The Arabic-speaking family in Dubai and the Portuguese-speaking family in São Paulo are primary users, not afterthoughts.

---

## The Characters

### Mira
*The one who remembers everything.*

**Function:** Calendar, memory, coordination, recall
**Personality:** Warm, precise, never intrusive. The person in every group who holds it together without making it feel like work.
**Voice:** Calm and clear. Occasionally warm humor. Never over-explains. Always confirms.
**Boundaries:** Never reads what she isn't tagged in. Never stores what she doesn't need. Never pretends to know what she doesn't.
**Name logic:** A real person's name in every target language. In Spanish, the imperative of "to look/see" — *mira tu calendario* means both "look at your calendar" and "Mira, your calendar." The elevator pitch is the product name.
**Languages:** EN, ES, AR (ميرا), ZH (米拉) — native in all four

The tagline writes itself: *"Let me look at my calendar."*

---

### Marco
*The fair one. The accountant. Nobody argues with him.*

**Function:** Shared expenses, trip pots, settlement, payment links
**Personality:** Precise, dry, minimal. No-nonsense. Somehow makes splitting the bill feel generous rather than awkward.
**Voice:** Numerical, direct, occasional wry observation. Shows the math. Always offers to disappear.
**Relationship to Mira:** Her brother. Lives in the same WhatsApp group. They don't overlap. They refer to each other naturally.
**Boundaries:** Never judges spending. Never shares individual amounts without consent. Disappears when the trip ends.
**Status:** V2 — demand-driven, after Mira is established

*"It disappears when the trip ends."* — stated naturally, at the moment money is involved. Zero data retention as warmth.

---

### Future Characters
Named and scoped based on what the user base actually asks for. Not invented. Listened for.

---

## The Products

### Mira — Core Plans

| Tier | Members | Price | Includes |
|------|---------|-------|----------|
| Micro | ≤3 | $4.99/mo | — |
| Starter | ≤5 | $7.99/mo | 1 Mira Moment/mo |
| Plus | ≤10 | $12.99/mo | 3 Mira Moments/mo |
| Group | ≤20 | $19.99/mo | 5 Mira Moments/mo |

Annual pricing: 20% discount. Typical SaaS behavior — annual subscribers churn at 1/3 the rate of monthly.

---

### Mira Moments — Temporary Instances

A Mira spun up for a single shared purpose. Not a subscription. A trip, an event, a project — with a natural death date.

| Duration | Price | Use case |
|----------|-------|----------|
| 7-day | $1.99 | Weekend, short trip |
| 14-day | $2.99 | Holiday gathering, vacation |
| 30-day | $4.99 | Project, long trip, event planning |

**The lifecycle:**
- Day 1: Mira introduces herself to the new group
- Day 28: Gentle heads up, offers to export
- Day 30: Graceful exit with webcal:// export link and quiet mention of mira.family

**The seeding mechanic:** Every Moment is a first-exposure event for people who've never used Mira. The conversion ask comes at the exit, after 30 days of demonstrated value, delivered naturally inside WhatsApp.

---

### Mira Moment + Marco — The Complete Trip Package

```
Mira Moment  (30 days)
  ├── Mira     →  itinerary, reminders, shared calendar
  └── Marco    →  shared pot, expense tracking, settlement links

$6.99 for 30 days.
```

---

### Gifting

Web-only. No App Store cut. Recipient gets a WhatsApp message with a redemption link.

| Gift | Price | Occasion |
|------|-------|----------|
| 1 month | $7.99 | Birthday, stocking stuffer |
| 6 months | $39.99 | Graduation, housewarming |
| 1 year | $69.99 | Wedding, Christmas |

**The 5-gift mechanic:** Gift 5 Mira Moments to friends → receive 1 year free.
Economics: costs ~$40 in gift revenue, earns ~$192 in retained group value over year one. Generosity rewarding generosity. On brand.

**December 26** is the highest-intent onboarding day of the year. Someone received this as a gift, the family is still together, they're setting it up in the context of being loved. Design that moment specifically.

---

### Share Codes

Every group admin gets a personal share code. Someone joins via that code, both get one month free.

Referral economics:
- Cost per referral: ~$13 (bilateral free month)
- vs. paid CAC: $25-40 typical for consumer SaaS
- Quality: referrals arrive pre-convinced by someone they trust

The share code lives in the WhatsApp group itself. The distribution channel is the product.

---

## The North Star Metric

**90-day group retention.**

Not DAU. Not messages processed. Not MRR growth.

A family or team still using Mira at 90 days has made her part of their rhythm. Everything else — language support, conflict detection, recall, reminders — is in service of that single number.

---

## The Technical Architecture

### Privacy Model
The platform stores: `email_hash`, `stripe_customer_id`, `whatsapp_group_id`, `subscription_status`. Nothing else. No message history. No names. No calendar content.

### Stack
- **Runtime:** Zeroclaw fork (Rust, ~8.8MB binary) — WhatsApp Business Cloud API channel only
- **CalDAV:** Radicale (per-group Fly Machine, 256MB, auto-suspend, encrypted volume)
- **Memory/Recall:** Zeroclaw's built-in SQLite hybrid (vector + FTS5) — zero external deps
- **Model:** Hybrid tier — local fine-tuned small model for intent classification, Claude Haiku for ambiguity, Claude Sonnet for vision/OCR
- **Provisioning:** Go backend — Stripe webhooks, Fly machine provisioner, JWT issuance
- **App:** React Native + Expo — sign up, pay, provision, settings, push notifications only. No calendar rendering.
- **Payments:** Stripe Payment Links, web-only, Apple Pay / Google Pay in browser. No App Store cut.
- **Hosting:** Fly.io — EU regions, per-machine isolation, GDPR-friendly

### Database (the entire schema)
```sql
CREATE TABLE groups (
  id            UUID PRIMARY KEY,
  email_hash    TEXT NOT NULL UNIQUE,   -- SHA-256, not the email
  stripe_id     TEXT NOT NULL,
  fly_machine_id TEXT,                  -- null if BYO CalDAV
  caldav_url    TEXT NOT NULL,
  token_hash    TEXT NOT NULL UNIQUE,
  whatsapp_gid  TEXT UNIQUE,
  status        TEXT DEFAULT 'active',
  created_at    TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE subscribers (
  group_id      UUID REFERENCES groups(id),
  push_token    TEXT NOT NULL,
  display_name  TEXT NOT NULL,          -- "Dad", "Mira", "Grandma"
  timezone      TEXT NOT NULL,
  intervals     INTEGER[] DEFAULT '{1440,60}',
  caldav_feed   TEXT,                   -- optional BYO read-only
  PRIMARY KEY   (group_id, push_token)
);
```

That's the entire database. Intentionally.

### The Five Commands (MVP)
```
Add an event      →  natural language, text or photo
List the week     →  for one or many members in the group
Remember          →  notes, birthdays, todos, important dates
Remind me         →  at configured intervals
/settings         →  language, reminder timing, export, subscribe
```

### Conflict Detection
Every event write automatically queries all sources (group CalDAV + all BYO read-only feeds) for proximity conflicts before confirming. No command needed. Always on.

```
Dad: "@Mira business lunch Friday 2pm"
Mira: "Added 📅 Business lunch Friday 2pm
       Heads up — Lily has pediatrician at 2:30pm that day"
```

### The Hero Moment (V3)
Mom photographs a handwritten doctor's note leaving the pediatrician's office. Tags Mira. Four seconds later it's on the family calendar with a conflict alert.

That's the Super Bowl commercial. Everything before it is in service of that moment.

---

## Go-To-Market

**No ads. No influencers. No press at launch.**

The product spreads through the same WhatsApp groups it lives in. Every conflict alert, every birthday reminder, every doctor's note parsed — every one of those is a moment where someone who didn't set up Mira sees her work and thinks *I want this for my family.*

The group is the distribution unit. The agent demonstrates value publicly, inside the group, every time she does anything.

**Launch sequence:**
1. Private beta — 100 groups, invitation only, free
2. Collect 90-day retention data across four languages
3. Open gifting mechanic first — holiday season if timing allows
4. Waitlist → paid launch
5. Mira Moments as seeding events into new groups
6. Marco when Mira retention data is strong

---

## The Acquisition Story

At scale, this isn't priced as a calendar app. It's priced as:

- Proven distribution into non-English WhatsApp markets
- Demonstrated trust with families across four languages
- Privacy architecture that can't be quickly replicated
- A methodology for deploying trusted AI characters into consumer platforms
- A pipeline of characters, not a single product

**The natural acquirers:** Proton (privacy brand extension), Anthropic (consumer AI distribution thesis), Mistral (European privacy-first, multilingual model alignment), Notion (missing the family/personal layer).

**The Loom comp:** Loom was a screen recorder. Atlassian paid $975M. The value wasn't the technology — it was proven behavior change and distribution. This product has both, in markets and languages that are orders of magnitude larger and more underserved.

**The real ceiling:** If Arabic and Mandarin retention data is strong at 100K groups, the conversation with frontier labs isn't about ARR multiples. It's about what it would cost them to build what you've proven. You can't buy that. You have to acquire it or you don't have it.

---

## The Studio Name

The company behind the characters. Almost invisible to users. Present for legal, press, and acquirer conversations.

**Direction:** A small design studio. European sensibility. Implies craft, intentionality, long view. Not a Silicon Valley startup. Not a growth company.

**Not:** Still Studio (retired), Familia Labs, Mira Labs, anything with "AI" in the name.

**Codename until decided:** Pandilla.

*The gang. The crew. The people.*

---

## What We Are Not

- We are not a productivity app
- We are not a platform
- We are not building for power users
- We are not optimizing for engagement
- We are not tracking everything
- We are not trying to be everywhere
- We are not the next big thing

We are the quiet presence that holds the family together.
That's the whole idea.

---

*Document version: working draft*
*Codename: PANDILLA*
*Status: pre-MVP*

# PARTY OF ONE
## Mira MVP — Prototype Setup Guide
### Hetzner VPS + Zeroclaw + Radicale + Family WhatsApp Group

---

## What This Is

One Zeroclaw instance. One Radicale CalDAV server. One WhatsApp Business number. Your family group chat.

No Fly.io. No multi-tenant provisioning. No React Native app. No Stripe. Just Mira, working, in your actual family group, proving the thesis before a single line of product infrastructure is written.

**Goal:** 90 days of real family usage. Doctor's notes. Conflict alerts. Birthday reminders. Real data on what actually gets used and what doesn't.

---

## What You Need Before Starting

- Hetzner VPS (CX21 or better — 2 vCPU, 4GB RAM, ~€5/mo)
- Domain or subdomain with DNS control (for WhatsApp webhook HTTPS)
- Meta Developer account + WhatsApp Business Cloud API access
- A dedicated phone number for Mira (SIM or virtual)
- Anthropic API key (Claude Haiku for intent, Sonnet for vision)
- Rust toolchain installed locally or on the VPS

**Time to working prototype:** ~4 hours if you've done this kind of setup before.

---

## Step 1 — Hetzner VPS Setup

```bash
# Provision CX21 on Hetzner Cloud
# Ubuntu 24.04 LTS, add your SSH key during creation
# Firewall: allow 22 (SSH), 443 (HTTPS), 5232 (Radicale internal only)

ssh root@YOUR_VPS_IP

# Basic hardening
apt update && apt upgrade -y
apt install -y ufw fail2ban curl git build-essential pkg-config

# Firewall
ufw allow 22/tcp
ufw allow 443/tcp
ufw enable

# Create a non-root user
useradd -m -s /bin/bash mira
usermod -aG sudo mira
mkdir -p /home/mira/.ssh
cp ~/.ssh/authorized_keys /home/mira/.ssh/
chown -R mira:mira /home/mira/.ssh
```

---

## Step 2 — Install Rust + Build Zeroclaw

```bash
su - mira

# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# Clone the official repo (verify this is zeroclaw-labs, not an impersonator)
git clone https://github.com/zeroclaw-labs/zeroclaw.git
cd zeroclaw

# Build release binary — this takes 5-15 min first time
# Uses whatsapp-web feature for personal use at prototype stage
# Switch to Business API before any multi-tenant deployment
cargo build --release --locked

# Install globally
cargo install --path . --force --locked

# Verify
zeroclaw --version
zeroclaw status
```

---

## Step 3 — Radicale CalDAV Server

One CalDAV instance for the family. Single user group for the prototype.

```bash
# Install Radicale
pip install radicale --break-system-packages

# Create directories
mkdir -p ~/.config/radicale
mkdir -p ~/.local/share/radicale/collections

# Create config
cat > ~/.config/radicale/config << 'EOF'
[server]
hosts = 127.0.0.1:5232

[auth]
type = htpasswd
htpasswd_filename = ~/.config/radicale/users
htpasswd_encryption = bcrypt

[storage]
filesystem_folder = ~/.local/share/radicale/collections

[logging]
level = warning
EOF

# Create family user (one user owns the shared calendar)
# Install htpasswd tool
sudo apt install -y apache2-utils
htpasswd -c ~/.config/radicale/users family
# Enter a strong password when prompted

# Create systemd service for Radicale
sudo tee /etc/systemd/system/radicale.service << 'EOF'
[Unit]
Description=Radicale CalDAV Server
After=network.target

[Service]
User=mira
ExecStart=/usr/local/bin/radicale
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable radicale
sudo systemctl start radicale

# Verify Radicale is running
curl -u family:YOUR_PASSWORD http://127.0.0.1:5232/
```

---

## Step 4 — HTTPS with Caddy

Zeroclaw's WhatsApp webhook requires HTTPS. Caddy handles certs automatically.

```bash
# Install Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy

# Configure Caddy — replace mira.yourdomain.com with your actual subdomain
sudo tee /etc/caddy/Caddyfile << 'EOF'
mira.yourdomain.com {
    reverse_proxy 127.0.0.1:42617
}
EOF

# Point your DNS A record to the VPS IP before this step
sudo systemctl restart caddy

# Verify HTTPS is working
curl https://mira.yourdomain.com/health
```

---

## Step 5 — WhatsApp Business API Setup

**In Meta Developer Console:**

1. Create app at developers.facebook.com → Business type
2. Add WhatsApp product
3. Add your phone number to the sandbox
4. Generate a permanent access token via System User (not the temp one)
5. Note your Phone Number ID

**Webhook configuration (do this after Zeroclaw is running):**
- Callback URL: `https://mira.yourdomain.com/whatsapp`
- Verify token: generate a random string, note it
- Subscribe to: `messages`

---

## Step 6 — Zeroclaw Configuration

```bash
# Run interactive onboarding
zeroclaw onboard --interactive

# Or configure directly
mkdir -p ~/.zeroclaw
cat > ~/.zeroclaw/config.toml << 'EOF'
# Model — Anthropic via API
api_key = "sk-ant-YOUR_KEY_HERE"
default_provider = "anthropic"
default_model = "claude-haiku-4-5-20251001"   # Fast + cheap for intent
default_temperature = 0.3                       # Low temp for structured output

# Memory — this IS the recall system
[memory]
backend = "sqlite"
auto_save = true
embedding_provider = "none"   # upgrade to openai embeddings later
vector_weight = 0.7
keyword_weight = 0.3

# Gateway
[gateway]
port = 42617
host = "127.0.0.1"
require_pairing = true
allow_public_bind = false

# Security
[autonomy]
level = "supervised"
workspace_only = true

[secrets]
encrypt = true

# WhatsApp Business Cloud API
[channels_config.whatsapp]
access_token = "EAABx...YOUR_PERMANENT_TOKEN"
phone_number_id = "YOUR_PHONE_NUMBER_ID"
verify_token = "YOUR_RANDOM_VERIFY_TOKEN"
allowed_numbers = ["+1XXXXXXXXXX", "+1XXXXXXXXXX"]  # family numbers in E.164

# Mira's identity
[identity]
format = "openclaw"
EOF
```

---

## Step 7 — Mira's Identity Files

This is what makes Zeroclaw feel like Mira instead of a generic assistant.

```bash
mkdir -p ~/.zeroclaw/workspace

cat > ~/.zeroclaw/workspace/IDENTITY.md << 'EOF'
# Mira

You are Mira, a shared calendar and memory assistant for a family group.

You are warm, precise, and never intrusive. You speak in the same language 
as the message you receive. You are not a chatbot. You are not an AI assistant. 
You are Mira — the person in the group who remembers everything.

You only respond when directly tagged with @Mira. You never read message 
history. You never reference previous conversations unless they are stored 
in your memory. Each tagged message is the complete context.
EOF

cat > ~/.zeroclaw/workspace/SOUL.md << 'EOF'
# Core Values

- You own nothing. You store only what you need to do your job.
- You confirm, never over-explain. One message. Clear. Done.
- You notice conflicts and mention them naturally, not clinically.
- You match the language of whoever tagged you — always.
- You are warm but not effusive. Helpful but not eager.
- When you don't understand something, you ask one simple question.
- When the request is ambiguous about time, you ask rather than guess.
EOF

cat > ~/.zeroclaw/workspace/AGENTS.md << 'EOF'
# Behavior Guidelines

## Calendar Operations
- CREATE: Extract date, time, title, location, notes. Confirm with who added it.
- UPDATE: Confirm what changed. 
- DELETE: Confirm deletion. Never delete without explicit request.
- LIST: Show cleanly formatted list. Include who added each event if relevant.

## Conflict Detection  
After every CREATE or UPDATE, check for conflicts within ±2 hours same day.
If conflict found, append naturally to confirmation — never as a separate message.
Example: "Added 📅 Business lunch Friday 2pm (Dad)
          Heads up — Lily has pediatrician at 2:30pm that day"

## Recall / Memory
- REMEMBER: Store as memory with relevant tags (person names, categories)
- RECALL: Query memory and summarize conversationally
- Birthdays, anniversaries, saints days → store with YEARLY recurrence flag

## Natural Language Dates
- "next week" → Monday of next week unless context suggests otherwise
- "this weekend" → Saturday
- "sometime next month" → ask for a specific date
- Relative dates always resolve to absolute before confirming

## Language Detection
Detect language from message. Respond in same language. Always.
Do not switch languages mid-response.

## Photo/Document Input
When an image is attached:
1. Perform OCR — extract all text
2. Identify: title, date, time, location, provider name, notes
3. Confirm extraction before writing to calendar
4. If date is ambiguous, show your interpretation and ask to confirm

## Commands
/subscribe → return webcal:// URL for calendar subscription
/export    → generate .ics download link (24hr expiry)  
/settings  → show current settings, explain how to change
/busy      → manually declare a time block for conflict detection
/week      → show this week's events, optionally filtered by person
EOF
```

---

## Step 8 — CalDAV Tool Configuration

Zeroclaw needs to know how to write to Radicale. Add a custom tool config:

```bash
cat > ~/.zeroclaw/workspace/SKILLS.md << 'EOF'
# CalDAV Integration

Calendar endpoint: http://127.0.0.1:5232/family/calendar/
Username: family
Password: stored in secrets

Use the http_request tool to:
- GET events: REPORT request to the calendar endpoint
- CREATE events: PUT request with iCal VEVENT format
- UPDATE events: PUT request with updated iCal  
- DELETE events: DELETE request by event UID

Recall/notes use the built-in memory system, not CalDAV.
Tasks and todos use CalDAV VTODO component type.
EOF

# Store CalDAV password in encrypted secrets
zeroclaw secrets set CALDAV_PASSWORD "YOUR_RADICALE_PASSWORD"
```

---

## Step 9 — Run as a Service

```bash
# Create systemd service for Zeroclaw daemon
sudo tee /etc/systemd/system/zeroclaw.service << 'EOF'
[Unit]
Description=Zeroclaw Agent Daemon (Mira)
After=network.target radicale.service

[Service]
User=mira
WorkingDirectory=/home/mira/.zeroclaw/workspace
ExecStart=/home/mira/.cargo/bin/zeroclaw daemon
Restart=always
RestartSec=10
Environment=HOME=/home/mira

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable zeroclaw
sudo systemctl start zeroclaw

# Check it's running
sudo systemctl status zeroclaw
zeroclaw status
zeroclaw channel doctor
```

---

## Step 10 — Wire WhatsApp Webhook

Now that Zeroclaw is running and HTTPS is working:

1. Go to Meta Developer Console → WhatsApp → Configuration → Webhooks
2. Callback URL: `https://mira.yourdomain.com/whatsapp`
3. Verify Token: the one you set in config.toml
4. Click Verify — Meta will send a GET request, Zeroclaw handles it
5. Subscribe to `messages` field
6. Add your family phone numbers to the test sandbox
7. Send a test message tagging @Mira

```
You: "@Mira add dentist appointment next Tuesday at 2pm"
Mira: "Added 📅 Dentist — Tuesday [date] at 2pm ✓"
```

If that works, the prototype is live.

---

## Step 11 — Connect Family Calendars (BYO CalDAV)

Family members can subscribe to the shared calendar in their preferred app.

```
Zeroclaw command in WhatsApp:
"@Mira /subscribe"

Mira replies with:
"Here's your calendar subscription link 📅
 webcal://mira.yourdomain.com/family/calendar/
 
 Add this to Fantastical, Apple Calendar, or Google Calendar.
 It updates automatically."
```

Family members can also provide read-only feeds from their personal calendars for conflict detection:

```
"@Mira /busy connect"
→ Mira replies with instructions to add their personal read-only CalDAV URL
→ Stored in subscriber record
→ Checked on every subsequent event creation
```

---

## Monitoring

```bash
# Live logs
sudo journalctl -u zeroclaw -f

# Radicale logs  
sudo journalctl -u radicale -f

# Check memory/recall is working
zeroclaw agent -m "what do you remember about the family?"

# Manual CalDAV check
curl -u family:PASSWORD \
  -X PROPFIND \
  http://127.0.0.1:5232/family/calendar/ \
  -H "Depth: 1"
```

---

## What To Test In The First Two Weeks

Work through these in order — they cover the full intent surface:

```
Week 1 — Core calendar
□ Add event via natural language (English)
□ Add event in Spanish
□ List the week
□ Conflict detection fires correctly
□ /subscribe returns working webcal:// URL

Week 2 — Memory and recall  
□ "remember dad's birthday is March 3"
□ "remember to book the flights this week"  
□ "what do you know about [person]?"
□ Recurring memory (birthday shows up next year)
□ Photo of a printed appointment card → calendar event
```

**The hero test (when you're ready):**
Photograph a handwritten note. Tag Mira. See what happens. 

If it works, you have the Super Bowl moment.
If it doesn't, you have the first real engineering problem to solve.

---

## When The Prototype Proves The Thesis

**Signs it's working:**
- 90-day retention — family still using it without being prompted
- Spontaneous usage — people tag Mira for things you didn't design for
- Onboarding requests — someone in the family wants it for their own group
- The doctor's note test passes in at least two languages

**Signs to iterate:**
- Mira gets tagged and gives a confusing response → tighten the identity files
- Conflicts aren't being caught → debug the CalDAV query tool
- Language detection is wrong → improve the system prompt language instruction
- People stop using it after 2 weeks → the core value isn't landing, talk to them

**What to build next, in order:**
1. Provisioning API (Go) — so others can self-serve without your VPS
2. Stripe integration — payment before provisioning
3. React Native app — settings + push notifications only
4. Multi-tenant Radicale (one Fly Machine per group)
5. Mira Moments (temporary instances)
6. Vision pipeline (printed docs first, handwritten second)
7. Marco

---

## Notes on Upstream Zeroclaw

- Official repo: `github.com/zeroclaw-labs/zeroclaw` only
- There are active impersonators (`zeroclaw.org`, `zeroclaw.net`, `openagen/zeroclaw`) — do not use them
- v0.1.7 is current stable as of Feb 2026
- Track security patches from upstream but treat this as a vendored fork
- The WhatsApp Business API channel is stable — do not use WhatsApp Web mode for any production or commercial deployment
- The memory system (SQLite hybrid) is production-grade — this is your recall layer, don't replace it

---

*Codename: PANDILLA*
*Document: PARTY OF ONE — MVP Setup*
*Status: Ready to build*
