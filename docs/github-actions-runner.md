# Setup GitHub Actions Self-Hosted Runner (Local)

Panduan setup GitHub Actions runner di server lokal (VPS/bare metal) untuk CI/CD otomatis.

## Overview

```
[GitHub Push] --(trigger)--> [Self-Hosted Runner] --(build+deploy)--> [Local Services]
```

## 1. Register Runner di GitHub

### A. Buat Runner di Repository Settings

1. Buka repository GitHub → Settings → Actions → Runners
2. Klik **New self-hosted runner**
3. Pilih OS: **Linux** → Architecture: **x64** (atau sesuai server)
4. Copy commands untuk download dan configure runner

### B. Install Runner di Server

```bash
# SSH ke server
ssh root@YOUR_SERVER_IP

# Buat direktori
mkdir -p /actions-runner
cd /actions-runner

# Download runner (ganti versi jika ada yang lebih baru)
curl -o actions-runner-linux-x64-2.311.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-x64-2.311.0.tar.gz

# Extract
tar xzf ./actions-runner-linux-x64-2.311.0.tar.gz

# Configure (ganti URL dan token dengan yang dari GitHub)
./config.sh --url https://github.com/islakhun24/crypto-ingitor-service --token YOUR_RUNNER_TOKEN

# Install sebagai service
sudo ./svc.sh install
sudo ./svc.sh start
```

### C. Verifikasi Runner

```bash
# Cek status
sudo systemctl status actions.runner.*

# Atau lihat di GitHub:
# Settings → Actions → Runners → Status: Idle (green)
```

## 2. Install Dependencies di Runner

Runner harus punya Go, Docker, PostgreSQL client, dan tools lainnya.

```bash
# Install Go
curl -L https://go.dev/dl/go1.23.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
source /etc/profile

# Install Docker (jika belum)
curl -fsSL https://get.docker.com | sh
usermod -aG docker $USER

# Install PostgreSQL client
apt-get update && apt-get install -y postgresql-client

# Install Make dan tools
apt-get install -y make git curl

# Verifikasi
go version
docker --version
psql --version
```

## 3. Setup Environment Variables & Secrets

### A. Repository Secrets (via GitHub UI)

Buka: Repository → Settings → Secrets and variables → Actions → New repository secret

| Secret | Value | Required |
|--------|-------|----------|
| `POSTGRES_HOST` | `localhost` atau `postgres` | ✅ |
| `POSTGRES_PORT` | `5432` | ✅ |
| `POSTGRES_USER` | `postgres` | ✅ |
| `POSTGRES_PASSWORD` | password DB | ✅ |
| `POSTGRES_DB` | `crypto_ultimate` | ✅ |
| `REDIS_HOST` | `localhost` atau `redis` | ❌ |
| `SUPPORTED_EXCHANGES` | `binance,okx,bybit` | ✅ |

### B. .env.local di Server

```bash
cd /opt/crypto-ingitor-service
cp .env.example .env.local
nano .env.local
```

## 4. Workflow yang Tersedia

### CI Workflow (`ci.yml`)

Trigger: push ke `main`/`develop`, pull request

Jobs:
- **Build & Test** — Build 4 binary + run tests + coverage
- **Docker Build Check** — Build Docker image

Runs on: `ubuntu-latest` (GitHub-hosted)

### Deploy Local Workflow (`deploy-local.yml`)

Trigger: push ke `main`, atau manual dispatch

Jobs:
- Checkout code
- Verify environment (Go, Docker)
- Run tests
- Build binaries
- Build Docker image
- Apply DB migrations
- Deploy API service
- Health check
- Seed endpoints

Runs on: `self-hosted`

### Scheduler & Collector Jobs (`jobs-local.yml`)

Trigger: cron schedule (setiap 2 menit), manual dispatch

Jobs:
- **Scheduler** — Generate collection jobs
- **Collector** — Execute collection jobs (depends on scheduler)
- **Retention** — Data cleanup (daily at 2 AM)

Runs on: `self-hosted`

## 5. Jalankan Workflow Pertama Kali

### Manual Trigger

1. Buka GitHub repository → Actions tab
2. Pilih **Deploy to Local Runner**
3. Klik **Run workflow** → pilih branch `main`

### Push Trigger

```bash
# Push code ke main
git push origin main

# Lihat di: GitHub → Actions tab
```

## 6. Monitoring Runner

### Cek Logs Runner

```bash
# View service logs
journalctl -u actions.runner.* -f

# View runner logs
cd /actions-runner
tail -f _diag/*.log
```

### Cek Workflow Logs

Lihat langsung di: GitHub → Actions → Pilih workflow run

### Cek Service Health

```bash
# API health
curl http://localhost:8080/healthz

# Docker containers
docker ps

# Logs
docker compose logs -f api-service
```

## 7. Update Runner

```bash
cd /actions-runner

# Stop
sudo ./svc.sh stop

# Update
./runsvc.sh &
# Atau download versi baru dan ulangi install

# Start
sudo ./svc.sh start
```

## 8. Remove Runner

```bash
cd /actions-runner

# Stop service
sudo ./svc.sh stop

# Uninstall
sudo ./svc.sh uninstall

# Remove from GitHub
./config.sh remove --token YOUR_RUNNER_TOKEN

# Hapus files
cd /
rm -rf /actions-runner
```

## Troubleshooting

### Runner tidak muncul di GitHub

```bash
# Cek status service
sudo systemctl status actions.runner.*

# Restart
sudo ./svc.sh stop
sudo ./svc.sh start

# Re-configure jika perlu
./config.sh remove --token TOKEN
./config.sh --url URL --token TOKEN
```

### Workflow failed: "go: command not found"

```bash
# Cek PATH
echo $PATH
which go

# Tambah ke PATH jika perlu
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

### Workflow failed: "Cannot connect to Docker"

```bash
# Cek Docker
sudo systemctl status docker

# Fix permissions
sudo usermod -aG docker $USER
# Logout dan login ulang
```

### Workflow failed: "connection refused" ke PostgreSQL

```bash
# Cek PostgreSQL berjalan
pg_isready -h localhost

# Cek env secrets sudah di-set di GitHub
# Settings → Secrets → Actions
```

## 9. Tips

1. **Gunakan labels** untuk multiple runners:
   ```bash
   ./config.sh --url URL --token TOKEN --labels production,self-hosted
   ```
   Lalu di workflow:
   ```yaml
   runs-on: [self-hosted, production]
   ```

2. **Auto-start on boot**:
   ```bash
   sudo systemctl enable actions.runner.*
   ```

3. **Resource monitoring** — gunakan `htop` atau `docker stats` untuk monitor resource usage runner

4. **Log rotation** — setup logrotate untuk runner logs:
   ```bash
   cat << 'EOF' > /etc/logrotate.d/github-runner
   /actions-runner/_diag/*.log {
       daily
       rotate 7
       compress
       missingok
       notifempty
   }
   EOF
   ```
