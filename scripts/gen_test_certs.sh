#!/usr/bin/env bash
# gen_test_certs.sh — Generate self-signed CA + server + client certs for TLS testing.
#
# Output:
#   certs/ca.pem           CA certificate (add to server trust store)
#   certs/server.pem       Server certificate (install on SBC/PBX)
#   certs/server.key       Server private key
#   certs/client.pem       Client certificate (pass as TLS_CERT to k6)
#   certs/client.key       Client private key (pass as TLS_KEY to k6)
#
# Usage:
#   SIP_DOMAIN=pbx.example.com bash scripts/gen_test_certs.sh
#   SIP_DOMAIN=192.168.1.100   bash scripts/gen_test_certs.sh

set -euo pipefail

DOMAIN="${SIP_DOMAIN:-pbx.example.com}"
DAYS=3650          # 10-year certs — fine for load testing
OUT="certs"

mkdir -p "$OUT"

echo "==> Generating CA key + cert (${DOMAIN})"
openssl genrsa -out "${OUT}/ca.key" 4096

openssl req -new -x509 -days "${DAYS}" \
  -key "${OUT}/ca.key" \
  -out "${OUT}/ca.pem" \
  -subj "/CN=xk6-sip-media Test CA/O=LoadTest/C=US"

echo "==> Generating server key + CSR"
openssl genrsa -out "${OUT}/server.key" 2048

openssl req -new \
  -key "${OUT}/server.key" \
  -out "${OUT}/server.csr" \
  -subj "/CN=${DOMAIN}/O=LoadTest/C=US"

cat > "${OUT}/server_ext.cnf" <<EOF
[SAN]
subjectAltName=DNS:${DOMAIN},IP:127.0.0.1
EOF

echo "==> Signing server cert with CA"
openssl x509 -req -days "${DAYS}" \
  -in  "${OUT}/server.csr" \
  -CA  "${OUT}/ca.pem" \
  -CAkey "${OUT}/ca.key" \
  -CAcreateserial \
  -out "${OUT}/server.pem" \
  -extfile "${OUT}/server_ext.cnf" \
  -extensions SAN

echo "==> Generating client key + CSR"
openssl genrsa -out "${OUT}/client.key" 2048

openssl req -new \
  -key "${OUT}/client.key" \
  -out "${OUT}/client.csr" \
  -subj "/CN=xk6-load-client/O=LoadTest/C=US"

echo "==> Signing client cert with CA"
openssl x509 -req -days "${DAYS}" \
  -in  "${OUT}/client.csr" \
  -CA  "${OUT}/ca.pem" \
  -CAkey "${OUT}/ca.key" \
  -CAcreateserial \
  -out "${OUT}/client.pem"

# Cleanup intermediates
rm -f "${OUT}/server.csr" "${OUT}/client.csr" "${OUT}/server_ext.cnf" "${OUT}/ca.srl"

echo ""
echo "==> Done! Certificates written to ${OUT}/"
echo ""
echo "    SBC/PBX server cert:  ${OUT}/server.pem  +  ${OUT}/server.key"
echo "    Client cert for k6:   ${OUT}/client.pem  +  ${OUT}/client.key"
echo "    CA cert:              ${OUT}/ca.pem"
echo ""
echo "    k6 run command:"
echo "      SIP_TARGET='sip:ivr@${DOMAIN}' \\"
echo "      TLS_CERT=${OUT}/client.pem \\"
echo "      TLS_KEY=${OUT}/client.key \\"
echo "      TLS_CA=${OUT}/ca.pem \\"
echo "      TLS_SERVER_NAME=${DOMAIN} \\"
echo "      ./k6 run examples/k6/scenarios/12_tls_transport.js"
