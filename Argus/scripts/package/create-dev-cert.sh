#!/bin/bash
# Create a self-signed code signing certificate for development.
# This gives a stable identity so macOS TCC permissions persist across rebuilds.
# Run once: ./scripts/package/create-dev-cert.sh

set -e

CERT_NAME="Argus Dev"
KEYCHAIN="login.keychain-db"

# Check if cert already exists
if security find-identity -p codesigning -v 2>&1 | grep -q "$CERT_NAME"; then
    echo "✅ Certificate '$CERT_NAME' already exists"
    security find-identity -p codesigning -v
    exit 0
fi

echo "🔐 Creating self-signed code signing certificate: '$CERT_NAME'"
echo "   You may be prompted for your macOS login password."
echo ""

# Create certificate config
CERT_CONFIG=$(mktemp /tmp/argus-cert-XXXXXX.conf)
cat > "$CERT_CONFIG" <<EOF
[ req ]
default_bits       = 2048
distinguished_name = req_dn
prompt             = no
x509_extensions    = codesign

[ req_dn ]
CN = $CERT_NAME

[ codesign ]
keyUsage               = digitalSignature
extendedKeyUsage       = codeSigning
basicConstraints       = CA:false
EOF

# Generate key + cert
CERT_PEM=$(mktemp /tmp/argus-cert-XXXXXX.pem)
KEY_PEM=$(mktemp /tmp/argus-key-XXXXXX.pem)
P12_FILE=$(mktemp /tmp/argus-cert-XXXXXX.p12)

openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$KEY_PEM" -out "$CERT_PEM" \
    -days 3650 -config "$CERT_CONFIG" 2>/dev/null

# Convert to p12 for import
openssl pkcs12 -export -inkey "$KEY_PEM" -in "$CERT_PEM" \
    -out "$P12_FILE" -passout pass: 2>/dev/null

# Import into login keychain
security import "$P12_FILE" -k "$KEYCHAIN" -T /usr/bin/codesign -P "" 2>/dev/null

# Trust the certificate for code signing
# This requires the user to confirm via system dialog
security add-trusted-cert -d -r trustRoot -k "$KEYCHAIN" "$CERT_PEM" 2>/dev/null || true

# Cleanup
rm -f "$CERT_CONFIG" "$CERT_PEM" "$KEY_PEM" "$P12_FILE"

echo ""
echo "✅ Certificate '$CERT_NAME' created and imported to keychain."
echo "   Use: codesign -s '$CERT_NAME' YourApp.app"
echo ""
security find-identity -p codesigning -v
