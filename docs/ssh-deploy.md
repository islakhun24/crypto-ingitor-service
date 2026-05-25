# Deploy via SSH ke VPS

Panduan deploy Aggregator Services ke VPS (Virtual Private Server) menggunakan SSH key authentication.

## Overview

```
[GitHub] --(git pull)--> [VPS] --(docker compose)--> [Running Services]
```

## 1. Persiapan VPS

### Requirements Server

- Ubuntu 22.04+ (disarankan)
- Docker 24+ & Docker Compose v2+
- Git
- Minimal 2 CPU, 4GB RAM, 50GB SSD

### Install Docker (jika belum)

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
sudo apt install -y docker.io docker-compose-v2

# Start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Add user ke docker group (logout & login ulang setelah ini)
sudo usermod -aG docker $USER
```

## 2. Setup SSH Key Authentication

### A. Generate SSH Key (di local machine)

```bash
# Cek existing key
ls ~/.ssh/id_*.pub

# Generate baru (jika belum ada)
ssh-keygen -t ed25519 -C "deploy@crypto-aggregator"
# Enter file: ~/.ssh/id_ed25519_deploy
# Passphrase: (kosongkan untuk deploy otomatis)
```

### B. Copy Public Key ke VPS

```bash
# Metode 1: ssh-copy-id
ssh-copy-id -i ~/.ssh/id_ed25519_deploy.pub root@YOUR_VPS_IP

# Metode 2: manual
cat ~/.ssh/id_ed25519_deploy.pub
# Copy output, lalu paste ke VPS:
ssh root@YOUR_VPS_IP "mkdir -p ~/.ssh && echo 'PASTE_PUB_KEY_HERE' >> ~/.ssh/authorized_keys"
```

### C. Test SSH Key Login

```bash
ssh -i ~/.ssh/id_ed25519_deploy root@YOUR_VPS_IP
# Jika berhasil login tanpa password, SSH key sudah OK
exit
```

### D. (Opsional) Disable Password Login

```bash
# Edit ssh config
sudo nano /etc/ssh/sshd_config

# Ubah:
PasswordAuthentication no
PermitRootLogin prohibit-password
PubkeyAuthentication yes

# Restart SSH
sudo systemctl restart sshd
```

## 3. Setup Repository di VPS

```bash
# SSH ke VPS
ssh -i ~/.ssh/id_ed25519_deploy root@YOUR_VPS_IP

# Buat direktori
mkdir -p /opt/crypto-ingitor-service
cd /opt/crypto-ingitor-service

# Clone repository
git clone https://github.com/islakhun24/crypto-ingitor-service.git .

# Setup environment
cp .env.example .env.local
nano .env.local
```

Edit `.env.local` sesuai VPS:

```env
# === DATABASE ===
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=STRONG_PASSWORD_HERE
POSTGRES_DB=crypto_ultimate
POSTGRES_SSLMODE=disable

# === REDIS ===
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# === APP ===
APP_PORT=8080
HTTP_ADDR=:8080
APP_PORT_PUBLISHED=80

# === RATE LIMIT ===
RATE_LIMIT_REQUESTS_PER_SECOND=5
RATE_LIMIT_BURST=10
CIRCUIT_BREAKER_FAILURE_LIMIT=5
CIRCUIT_BREAKER_COOLDOWN_SECONDS=30

# === EXCHANGES ===
SUPPORTED_EXCHANGES=binance,okx,bybit,bitget,gate,mexc

# === RETENTION ===
RETENTION_MAX_ROWS_PER_RUN=50000

# === NETWORK ===
AGGREGATOR_NETWORK_NAME=aggregator-services
AGGREGATOR_NETWORK_EXTERNAL=false
```

## 4. First Deploy (Manual)

```bash
cd /opt/crypto-ingitor-service

# Build dan start
docker compose -f docker-compose.yml --env-file .env.local up -d --build postgres redis api-service prometheus

# Tunggu database ready (10-15 detik)
sleep 15

# Apply migrations
docker compose exec -T postgres sh -c 'for f in /migrations/*.sql; do psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$f"; done'

# Seed endpoints
docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /migrations/000007_seed_exchange_api_endpoints.sql'
docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /migrations/000008_seed_derivative_collection_policies.sql'

# Verifikasi
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## 5. Setup Cron Jobs (Scheduler & Collector)

```bash
# Edit crontab
sudo crontab -e
```

Tambahkan:

```cron
# Crypto Aggregator - Scheduler (every minute)
* * * * * cd /opt/crypto-ingitor-service && docker compose --env-file .env.local run --rm scheduler-service >> /var/log/aggregator-scheduler.log 2>&1

# Crypto Aggregator - Collector (every minute)
* * * * * cd /opt/crypto-ingitor-service && docker compose --env-file .env.local run --rm collector-service >> /var/log/aggregator-collector.log 2>&1

# Crypto Aggregator - Retention (daily at 2 AM)
0 2 * * * cd /opt/crypto-ingitor-service && docker compose --env-file .env.local run --rm retention-service >> /var/log/aggregator-retention.log 2>&1
```

Buat log directory:

```bash
sudo mkdir -p /var/log
touch /var/log/aggregator-scheduler.log /var/log/aggregator-collector.log /var/log/aggregator-retention.log
```

## 6. Automasi Deploy dengan Script

### Deploy Script (di VPS)

Simpan sebagai `/opt/crypto-ingitor-service/deploy.sh`:

```bash
#!/bin/bash
set -e

PROJECT_DIR="/opt/crypto-ingitor-service"
LOG_FILE="/var/log/aggregator-deploy.log"

echo "[$(date)] Starting deploy..." >> "$LOG_FILE"

cd "$PROJECT_DIR"

# Pull latest code
git pull origin main >> "$LOG_FILE" 2>&1

# Rebuild dan restart API
docker compose --env-file .env.local up -d --build api-service >> "$LOG_FILE" 2>&1

# Cleanup old images
docker image prune -f >> "$LOG_FILE" 2>&1

echo "[$(date)] Deploy finished" >> "$LOG_FILE"
```

```bash
chmod +x /opt/crypto-ingitor-service/deploy.sh
```

### Local Deploy Script

Simpan di local machine sebagai `~/deploy.sh`:

```bash
#!/bin/bash
set -e

VPS_IP="YOUR_VPS_IP"
SSH_KEY="~/.ssh/id_ed25519_deploy"
VPS_USER="root"

echo "Deploying to $VPS_IP..."

# SSH ke VPS dan jalankan deploy
ssh -i "$SSH_KEY" "$VPS_USER@$VPS_IP" "bash /opt/crypto-ingitor-service/deploy.sh"

echo "Deploy complete!"
echo ""
echo "Check health:"
curl -s http://$VPS_IP:8080/healthz || echo "API not responding"
```

```bash
chmod +x ~/deploy.sh
```

### Deploy dengan satu command:

```bash
~/deploy.sh
```

## 7. Setup GitHub Actions (CI/CD Auto Deploy)

### Tambahkan SSH Key ke GitHub Secrets

1. Buka repository GitHub → Settings → Secrets and variables → Actions
2. Tambahkan secrets:
   - `SSH_PRIVATE_KEY` : isi dengan `~/.ssh/id_ed25519_deploy` (private key)
   - `VPS_IP` : IP address VPS
   - `VPS_USER` : username SSH (biasanya `root`)

### Buat GitHub Actions Workflow

Buat file `.github/workflows/deploy.yml`:

```yaml
name: Deploy to VPS

on:
  push:
    branches: [ main ]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - name: Setup SSH
      uses: webfactory/ssh-agent@v0.9.0
      with:
        ssh-private-key: ${{ secrets.SSH_PRIVATE_KEY }}

    - name: Add known hosts
      run: |
        mkdir -p ~/.ssh
        ssh-keyscan -H ${{ secrets.VPS_IP }} >> ~/.ssh/known_hosts

    - name: Deploy to VPS
      run: |
        ssh ${{ secrets.VPS_USER }}@${{ secrets.VPS_IP }} "bash /opt/crypto-ingitor-service/deploy.sh"

    - name: Verify deployment
      run: |
        sleep 5
        curl -s -o /dev/null -w "%{http_code}" http://${{ secrets.VPS_IP }}:8080/healthz
```

## 8. Monitoring

### Cek Service Status

```bash
# SSH ke VPS
ssh -i ~/.ssh/id_ed25519_deploy root@YOUR_VPS_IP

# Cek container
docker compose ps

# Cek logs
docker compose logs -f --tail=100 api-service
docker compose logs -f --tail=100 scheduler-service
docker compose logs -f --tail=100 collector-service

# Cek resource usage
docker stats
```

### Prometheus

Buka `http://YOUR_VPS_IP:9090` untuk akses Prometheus.

### Log Files

```bash
# Scheduler logs
tail -f /var/log/aggregator-scheduler.log

# Collector logs
tail -f /var/log/aggregator-collector.log

# Retention logs
tail -f /var/log/aggregator-retention.log

# Deploy logs
tail -f /var/log/aggregator-deploy.log
```

## 9. Rollback

Jika deploy gagal:

```bash
# SSH ke VPS
ssh -i ~/.ssh/id_ed25519_deploy root@YOUR_VPS_IP

cd /opt/crypto-ingitor-service

# Rollback ke commit sebelumnya
git log --oneline -5
git reset --hard HEAD~1

# Rebuild
docker compose --env-file .env.local up -d --build api-service
```

## 10. Security Checklist

- [ ] SSH key authentication enabled
- [ ] Password login disabled (`PasswordAuthentication no`)
- [ ] Firewall aktif (`ufw enable`)
- [ ] Hanya port yang diperlukan yang terbuka (80, 443, 22)
- [ ] PostgreSQL password kuat
- [ ] `.env.local` tidak di-commit ke Git (`chmod 600 .env.local`)
- [ ] Docker logs capped (sudah di docker-compose.yml)
- [ ] Auto-update security patches (`unattended-upgrades`)

### Setup Firewall (UFW)

```bash
sudo apt install -y ufw
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS (jika pakai SSL)
sudo ufw enable
```

## Ringkasan Perintah

| Aksi | Command |
|------|---------|
| SSH ke VPS | `ssh -i ~/.ssh/id_ed25519_deploy root@IP` |
| Manual deploy | `cd /opt/crypto-ingitor-service && git pull && docker compose up -d --build` |
| Cek health | `curl http://IP:8080/healthz` |
| Cek logs | `docker compose logs -f api-service` |
| Restart API | `docker compose restart api-service` |
| View stats | `docker stats` |
| Backup DB | `docker exec aggregator-postgres pg_dump -U postgres crypto_ultimate > backup.sql` |
