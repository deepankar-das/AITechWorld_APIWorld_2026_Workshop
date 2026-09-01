#!/bin/bash
# Enforcer — Generate TLS Certificates for Hub-Sentinel Communication
#
# Generates a self-signed CA, server cert (Hub Server), and client cert
# (Sentinel) for mutual TLS (mTLS) authentication.
#
# Usage:
#   sudo ./scripts/generate-certs.sh                    # Generate all certs
#   sudo ./scripts/generate-certs.sh --output /etc/enforcer/certs
#
# Output (both naming conventions are generated for compatibility):
#   ca.pem / ca.crt                 — Certificate Authority (both sides trust this)
#   ca-key.pem                      — CA private key (keep secret, only used for signing)
#   server.pem / server.crt         — Management Hub certificate
#   server-key.pem / server.key     — Management Hub private key
#   client.pem / client.crt         — Sentinel client certificate
#   client-key.pem / client.key     — Sentinel client private key

set -euo pipefail

OUTPUT_DIR="/etc/enforcer/certs"
DAYS=3650  # 10 years

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

banner() {
  local phase="$1"
  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "   _   _     ___ ___ ___ _____      ___   _    _"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "  /_\\ /_\\   | __|_ _| _ \\ __\\ \\    / /_\\ | |  | |"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " / _ \\ _ \\  | _| | ||   / _| \\ \\/\\/ / _ \\| |__| |__"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " /_/ \\_\\/\\_\\ |_| |___|_|_\\___|  \\_/\\_/_/ \\_\\____|____|"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "$phase"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
  echo ""
}

usage() {
  echo "Usage: $0 [--output DIR] [--days N]"
  echo "  --output, -o   Output directory for certs (default: /etc/enforcer/certs)"
  echo "  --days         Certificate validity in days (default: 3650)"
}

# Backward compatible positional arg support:
#   ./generate-certs.sh /path/to/certs
if [[ $# -eq 1 && "${1:-}" != --* ]]; then
  OUTPUT_DIR="$1"
else
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --output|-o)
        if [[ $# -lt 2 ]]; then
          echo -e "${RED}[ERROR]${NC} Missing value for $1"
          usage
          exit 1
        fi
        OUTPUT_DIR="$2"
        shift 2
        ;;
      --days)
        if [[ $# -lt 2 ]]; then
          echo -e "${RED}[ERROR]${NC} Missing value for $1"
          usage
          exit 1
        fi
        DAYS="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo -e "${RED}[ERROR]${NC} Unknown option: $1"
        usage
        exit 1
        ;;
    esac
  done
fi

mkdir -p -- "$OUTPUT_DIR"

banner "Certificate Generation Start"

# ── Certificate Authority ────────────────────────────────────────────────────

if [[ ! -f "$OUTPUT_DIR/ca.pem" ]]; then
  echo -e "${GREEN}[CA]${NC} Generating Certificate Authority..."
  openssl req -new -x509 -nodes \
    -days $DAYS \
    -keyout "$OUTPUT_DIR/ca-key.pem" \
    -out "$OUTPUT_DIR/ca.pem" \
    -subj "/C=US/ST=CA/O=Enforcer/CN=Enforcer CA" \
    2>/dev/null
  chmod 600 "$OUTPUT_DIR/ca-key.pem"
  chmod 644 "$OUTPUT_DIR/ca.pem"
  echo -e "${GREEN}[OK]${NC} CA certificate: $OUTPUT_DIR/ca.pem"
else
  echo -e "${GREEN}[OK]${NC} CA certificate already exists"
fi

# ── Central Server Certificate ───────────────────────────────────────────────

if [[ ! -f "$OUTPUT_DIR/server.pem" ]]; then
  echo -e "${GREEN}[SERVER]${NC} Generating Hub Server certificate..."

  # Create server config with SANs
  cat > "$OUTPUT_DIR/server.cnf" << EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
C = US
ST = CA
O = Enforcer
CN = Enforcer Hub Server

[v3_req]
subjectAltName = @alt_names
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = localhost
DNS.2 = *.enforcer.local
DNS.3 = enforcer-central
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

  # Generate key and CSR
  openssl req -new -nodes \
    -keyout "$OUTPUT_DIR/server-key.pem" \
    -out "$OUTPUT_DIR/server.csr" \
    -config "$OUTPUT_DIR/server.cnf" \
    2>/dev/null

  # Sign with CA
  openssl x509 -req \
    -in "$OUTPUT_DIR/server.csr" \
    -CA "$OUTPUT_DIR/ca.pem" \
    -CAkey "$OUTPUT_DIR/ca-key.pem" \
    -CAcreateserial \
    -out "$OUTPUT_DIR/server.pem" \
    -days $DAYS \
    -extensions v3_req \
    -extfile "$OUTPUT_DIR/server.cnf" \
    2>/dev/null

  chmod 600 "$OUTPUT_DIR/server-key.pem"
  chmod 644 "$OUTPUT_DIR/server.pem"
  rm -f "$OUTPUT_DIR/server.csr" "$OUTPUT_DIR/server.cnf"
  echo -e "${GREEN}[OK]${NC} Server certificate: $OUTPUT_DIR/server.pem"
else
  echo -e "${GREEN}[OK]${NC} Server certificate already exists"
fi

# ── Client Agent Certificate ─────────────────────────────────────────────────

if [[ ! -f "$OUTPUT_DIR/client.pem" ]]; then
  echo -e "${GREEN}[CLIENT]${NC} Generating Sentinel certificate..."

  cat > "$OUTPUT_DIR/client.cnf" << EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
C = US
ST = CA
O = Enforcer
CN = Enforcer Sentinel

[v3_req]
keyUsage = digitalSignature
extendedKeyUsage = clientAuth
EOF

  openssl req -new -nodes \
    -keyout "$OUTPUT_DIR/client-key.pem" \
    -out "$OUTPUT_DIR/client.csr" \
    -config "$OUTPUT_DIR/client.cnf" \
    2>/dev/null

  openssl x509 -req \
    -in "$OUTPUT_DIR/client.csr" \
    -CA "$OUTPUT_DIR/ca.pem" \
    -CAkey "$OUTPUT_DIR/ca-key.pem" \
    -CAcreateserial \
    -out "$OUTPUT_DIR/client.pem" \
    -days $DAYS \
    -extensions v3_req \
    -extfile "$OUTPUT_DIR/client.cnf" \
    2>/dev/null

  chmod 600 "$OUTPUT_DIR/client-key.pem"
  chmod 644 "$OUTPUT_DIR/client.pem"
  rm -f "$OUTPUT_DIR/client.csr" "$OUTPUT_DIR/client.cnf"
  echo -e "${GREEN}[OK]${NC} Client certificate: $OUTPUT_DIR/client.pem"
else
  echo -e "${GREEN}[OK]${NC} Client certificate already exists"
fi

# Cleanup
rm -f "$OUTPUT_DIR/ca.srl"

# Compatibility aliases for Go services and deploy scripts (.crt/.key names).
cp -f "$OUTPUT_DIR/ca.pem" "$OUTPUT_DIR/ca.crt"
cp -f "$OUTPUT_DIR/server.pem" "$OUTPUT_DIR/server.crt"
cp -f "$OUTPUT_DIR/server-key.pem" "$OUTPUT_DIR/server.key"
cp -f "$OUTPUT_DIR/client.pem" "$OUTPUT_DIR/client.crt"
cp -f "$OUTPUT_DIR/client-key.pem" "$OUTPUT_DIR/client.key"
chmod 644 "$OUTPUT_DIR/ca.crt" "$OUTPUT_DIR/server.crt" "$OUTPUT_DIR/client.crt"
chmod 600 "$OUTPUT_DIR/server.key" "$OUTPUT_DIR/client.key"

echo ""
echo -e "${CYAN}${BOLD}Certificates generated:${NC}"
echo "  CA:     $OUTPUT_DIR/ca.pem (alias: ca.crt)"
echo "  Server: $OUTPUT_DIR/server.pem + server-key.pem (aliases: server.crt/server.key)"
echo "  Client: $OUTPUT_DIR/client.pem + client-key.pem (aliases: client.crt/client.key)"
echo ""
echo "  Deploy ca.crt + server.crt/server.key to the Hub Server."
echo "  Deploy ca.crt + client.crt/client.key to each Sentinel machine."
echo ""
banner "Certificate Generation Complete"
