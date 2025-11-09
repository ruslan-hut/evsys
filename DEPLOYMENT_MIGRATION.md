# OCPP Migration Branch Deployment Guide

This guide explains how to deploy the `ocpp-migration` branch alongside the production `master` branch on the same server.

## Overview

The migration deployment (`evsys-m`) runs in parallel with the production deployment (`evsys`) to allow testing OCPP 2.0.1 functionality without affecting the production system.

**Deployment Details:**
- **Branch:** `ocpp-migration`
- **Binary:** `evsys-m`
- **Config:** `/etc/conf/evsys-m.yml`
- **Service:** `evsys-m.service`
- **Server:** Same as production (wattbrews.me)

## GitHub Actions Setup

### Required Variables

Add these variables in GitHub repository settings under **Settings → Secrets and variables → Actions → Variables**:

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT_M` | WebSocket port for OCPP connections | `5100` |
| `API_PORT_M` | REST API port | `5101` |
| `METRICS_PORT_M` | Prometheus metrics port | `9191` |
| `MONGO_DB_M` | MongoDB database name | `evsys_migration` |

**Note:** Use different ports than production to avoid conflicts. Production uses ports 5000, 5001, 9090.

### Existing Secrets (Reused)

These secrets are shared with production deployment:
- `SERVER_IP` - Server IP address
- `SERVER_USER` - SSH username
- `SSH_PRIVATE_KEY` - SSH private key for deployment
- `MONGO_USER` - MongoDB username
- `MONGO_PASS` - MongoDB password
- `PAYMENT_API_URL` - Payment API endpoint
- `PAYMENT_API_KEY` - Payment API key
- `TELEGRAM_API_KEY` - Telegram bot API key
- `OCPI_TOKEN` - OCPI authentication token

### Existing Variables (Reused)

- `TIME_ZONE` - Server timezone (e.g., `Europe/Kiev`)
- `TLS_ENABLED` - Enable TLS for WebSocket (`true`/`false`)
- `API_TLS_ENABLED` - Enable TLS for API (`true`/`false`)
- `CERT_FILE` - TLS certificate file path
- `KEY_FILE` - TLS key file path
- `MONGO_HOST` - MongoDB host
- `MONGO_PORT` - MongoDB port
- `OCPI_URL` - OCPI service URL

## Server Setup

### 1. Install the systemd Service

Copy the service file to the server:

```bash
scp evsys-m.service user@server:/etc/systemd/system/
```

Or create it directly on the server at `/etc/systemd/system/evsys-m.service` with the contents from `evsys-m.service`.

### 2. Enable and Start the Service

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable the service to start on boot
sudo systemctl enable evsys-m.service

# Start the service
sudo systemctl start evsys-m.service

# Check status
sudo systemctl status evsys-m.service
```

### 3. Service Management Commands

```bash
# Start the service
sudo systemctl start evsys-m.service

# Stop the service
sudo systemctl stop evsys-m.service

# Restart the service
sudo systemctl restart evsys-m.service

# View logs
sudo journalctl -u evsys-m.service -f

# View recent logs
sudo journalctl -u evsys-m.service -n 100
```

## MongoDB Setup

Create a separate database for the migration:

```javascript
// Connect to MongoDB
mongosh

// Switch to migration database
use evsys_migration

// Create collections (will be created automatically by the app)
// But you can verify they exist after first run:
show collections
```

## Testing the Deployment

### 1. Check if Service is Running

```bash
sudo systemctl status evsys-m.service
```

### 2. Verify Ports are Listening

```bash
# Check WebSocket port
sudo netstat -tlnp | grep 5100

# Check API port
sudo netstat -tlnp | grep 5101

# Check metrics port
sudo netstat -tlnp | grep 9191
```

### 3. Test API Endpoint

```bash
curl http://localhost:5101/api
```

### 4. Test WebSocket Connection

Use an OCPP charge point simulator configured to connect to:
```
ws://wattbrews.me:5100/ws/TEST_CP_ID
```

Or with OCPP 2.0.1 subprotocol:
```
ws://wattbrews.me:5100/ws/TEST_CP_ID
Sec-WebSocket-Protocol: ocpp2.0.1
```

## Automatic Deployment

The GitHub Actions workflow automatically deploys when changes are pushed to the `ocpp-migration` branch:

1. **Build:** Compiles the Go application to `evsys-m` binary
2. **Config:** Creates `/etc/conf/evsys-m.yml` with environment-specific values
3. **Deploy:** Copies binary to `/usr/local/bin/evsys-m`
4. **Restart:** Restarts `evsys-m.service`

## Monitoring

### Application Logs

```bash
# Real-time logs
sudo journalctl -u evsys-m.service -f

# Logs from the last hour
sudo journalctl -u evsys-m.service --since "1 hour ago"

# Logs with specific priority
sudo journalctl -u evsys-m.service -p err
```

### Prometheus Metrics

Access metrics at:
```
http://localhost:9191/metrics
```

## Configuration Differences from Production

| Setting | Production | Migration |
|---------|-----------|-----------|
| WebSocket Port | 5000 | 5100 |
| API Port | 5001 | 5101 |
| Metrics Port | 9090 | 9191 |
| MongoDB Database | `evsys` | `evsys_migration` |
| Binary | `evsys` | `evsys-m` |
| Config | `evsys.yml` | `evsys-m.yml` |
| Service | `evsys.service` | `evsys-m.service` |

## Protocol Version Support

The migration deployment (`evsys-m`) supports both:
- **OCPP 1.6J** - Backward compatible with existing charge points
- **OCPP 2.0.1** - New protocol with enhanced features

Charge points can connect using either protocol version. The system automatically detects the protocol version during WebSocket handshake via the `Sec-WebSocket-Protocol` header.

## Troubleshooting

### Service Won't Start

```bash
# Check for errors
sudo journalctl -u evsys-m.service -n 50

# Verify binary exists and is executable
ls -la /usr/local/bin/evsys-m

# Verify config file exists
ls -la /etc/conf/evsys-m.yml

# Check config syntax
cat /etc/conf/evsys-m.yml
```

### Port Already in Use

```bash
# Find process using the port
sudo lsof -i :5100

# Kill the process if needed
sudo kill -9 <PID>
```

### Database Connection Issues

```bash
# Check MongoDB is running
sudo systemctl status mongod

# Test connection
mongosh --host localhost --port 27017 -u <username> -p
```

## Rollback

To rollback to a previous version:

1. **Manual:** SSH to server and replace binary
   ```bash
   sudo systemctl stop evsys-m.service
   # Copy old binary back
   sudo systemctl start evsys-m.service
   ```

2. **Re-deploy:** Push a commit to `ocpp-migration` branch that reverts changes

## Migration to Production

When ready to migrate to production:

1. Test thoroughly on migration deployment
2. Update production environment variables if needed
3. Merge `ocpp-migration` → `master`
4. Production deployment will automatically trigger

## Support

For issues or questions:
- Check logs: `sudo journalctl -u evsys-m.service -f`
- Review GitHub Actions workflow runs
- Verify all environment variables are set correctly
