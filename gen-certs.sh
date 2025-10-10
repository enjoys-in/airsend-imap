#!/bin/bash

#########################################################
# IMAP TLS Certificate Generator
# Creates self-signed SSL/TLS certificates for IMAP server
# Author: Your Name
# Version: 1.0
#########################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
CERT_DIR="./certs"
CERT_VALIDITY_DAYS=365
KEY_SIZE=4096

# Print colored message
print_message() {
    local color=$1
    shift
    echo -e "${color}$@${NC}"
}

print_header() {
    echo ""
    print_message "$BLUE" "=========================================="
    print_message "$BLUE" "$1"
    print_message "$BLUE" "=========================================="
    echo ""
}

print_success() {
    print_message "$GREEN" "✓ $1"
}

print_error() {
    print_message "$RED" "✗ $1"
}

print_warning() {
    print_message "$YELLOW" "⚠ $1"
}

print_info() {
    print_message "$BLUE" "ℹ $1"
}

# Check if OpenSSL is installed
check_openssl() {
    if ! command -v openssl &> /dev/null; then
        print_error "OpenSSL is not installed!"
        print_info "Install it with: sudo apt-get install openssl (Ubuntu/Debian)"
        print_info "                  sudo yum install openssl (CentOS/RHEL)"
        exit 1
    fi
    print_success "OpenSSL found: $(openssl version)"
}

# Prompt for certificate details
get_certificate_info() {
    print_header "Certificate Configuration"
    
    # Domain/Common Name
    read -p "Enter domain name (e.g., mail.example.com): " DOMAIN
    if [ -z "$DOMAIN" ]; then
        print_error "Domain name is required!"
        exit 1
    fi
    
    # Organization
    read -p "Enter organization name (e.g., MyCompany Inc): " ORG
    if [ -z "$ORG" ]; then
        ORG="Self-Signed Certificate"
    fi
    
    # Organizational Unit
    read -p "Enter organizational unit (e.g., IT Department) [Optional]: " OU
    if [ -z "$OU" ]; then
        OU="Mail Services"
    fi
    
    # Country
    read -p "Enter country code (e.g., US, UK, IN): " COUNTRY
    if [ -z "$COUNTRY" ]; then
        COUNTRY="US"
    fi
    
    # State/Province
    read -p "Enter state/province (e.g., California, Maharashtra): " STATE
    if [ -z "$STATE" ]; then
        STATE="State"
    fi
    
    # City/Locality
    read -p "Enter city/locality (e.g., San Francisco, Mumbai): " CITY
    if [ -z "$CITY" ]; then
        CITY="City"
    fi
    
    # Email
    read -p "Enter email address [Optional]: " EMAIL
    
    # Validity
    read -p "Enter certificate validity in days [$CERT_VALIDITY_DAYS]: " VALIDITY_INPUT
    if [ ! -z "$VALIDITY_INPUT" ]; then
        CERT_VALIDITY_DAYS=$VALIDITY_INPUT
    fi
    
    # Subject Alternative Names
    print_info "Do you want to add Subject Alternative Names (SANs)?"
    print_info "This allows the certificate to work with multiple domains/IPs"
    read -p "Add SANs? (y/n) [n]: " ADD_SAN
    
    if [[ "$ADD_SAN" == "y" || "$ADD_SAN" == "Y" ]]; then
        SAN_DOMAINS=()
        SAN_IPS=()
        
        # Add primary domain
        SAN_DOMAINS+=("$DOMAIN")
        
        # Additional domains
        while true; do
            read -p "Add additional domain (or press Enter to skip): " ADDITIONAL_DOMAIN
            if [ -z "$ADDITIONAL_DOMAIN" ]; then
                break
            fi
            SAN_DOMAINS+=("$ADDITIONAL_DOMAIN")
        done
        
        # IP addresses
        print_info "Add IP addresses (for local testing)"
        read -p "Add localhost (127.0.0.1)? (y/n) [y]: " ADD_LOCALHOST
        if [[ "$ADD_LOCALHOST" != "n" && "$ADD_LOCALHOST" != "N" ]]; then
            SAN_IPS+=("127.0.0.1")
        fi
        
        while true; do
            read -p "Add additional IP address (or press Enter to skip): " ADDITIONAL_IP
            if [ -z "$ADDITIONAL_IP" ]; then
                break
            fi
            SAN_IPS+=("$ADDITIONAL_IP")
        done
    fi
}

# Create OpenSSL configuration file
create_openssl_config() {
    print_info "Creating OpenSSL configuration..."
    
    CONFIG_FILE="$CERT_DIR/openssl.cnf"
    
    cat > "$CONFIG_FILE" <<EOF
[req]
default_bits = $KEY_SIZE
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = v3_req
x509_extensions = v3_ca

[dn]
C = $COUNTRY
ST = $STATE
L = $CITY
O = $ORG
OU = $OU
CN = $DOMAIN
EOF

    if [ ! -z "$EMAIL" ]; then
        echo "emailAddress = $EMAIL" >> "$CONFIG_FILE"
    fi

    # Add SAN if requested
    if [[ "$ADD_SAN" == "y" || "$ADD_SAN" == "Y" ]]; then
        cat >> "$CONFIG_FILE" <<EOF

[v3_req]
subjectAltName = @alt_names
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth

[v3_ca]
subjectAltName = @alt_names
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth

[alt_names]
EOF
        
        # Add DNS entries
        local dns_index=1
        for domain in "${SAN_DOMAINS[@]}"; do
            echo "DNS.$dns_index = $domain" >> "$CONFIG_FILE"
            ((dns_index++))
        done
        
        # Add IP entries
        local ip_index=1
        for ip in "${SAN_IPS[@]}"; do
            echo "IP.$ip_index = $ip" >> "$CONFIG_FILE"
            ((ip_index++))
        done
    else
        cat >> "$CONFIG_FILE" <<EOF

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth

[v3_ca]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
EOF
    fi
    
    print_success "OpenSSL configuration created: $CONFIG_FILE"
}

# Generate private key
generate_private_key() {
    print_info "Generating $KEY_SIZE-bit RSA private key..."
    
    KEY_FILE="$CERT_DIR/imap-key.pem"
    
    openssl genrsa -out "$KEY_FILE" $KEY_SIZE 2>/dev/null
    
    # Set secure permissions
    chmod 600 "$KEY_FILE"
    
    print_success "Private key generated: $KEY_FILE"
}

# Generate self-signed certificate
generate_certificate() {
    print_info "Generating self-signed certificate..."
    
    CERT_FILE="$CERT_DIR/imap-cert.pem"
    CONFIG_FILE="$CERT_DIR/openssl.cnf"
    
    openssl req -new -x509 \
        -key "$KEY_FILE" \
        -out "$CERT_FILE" \
        -days $CERT_VALIDITY_DAYS \
        -config "$CONFIG_FILE" \
        -extensions v3_ca 2>/dev/null
    
    # Set permissions
    chmod 644 "$CERT_FILE"
    
    print_success "Certificate generated: $CERT_FILE"
}

# Create combined PEM file
create_combined_pem() {
    print_info "Creating combined PEM file..."
    
    COMBINED_FILE="$CERT_DIR/imap-combined.pem"
    
    cat "$CERT_FILE" "$KEY_FILE" > "$COMBINED_FILE"
    chmod 600 "$COMBINED_FILE"
    
    print_success "Combined PEM created: $COMBINED_FILE"
}

# Create PKCS12 format (for some clients)
create_pkcs12() {
    print_info "Creating PKCS12 format (.p12)..."
    
    P12_FILE="$CERT_DIR/imap-cert.p12"
    
    read -sp "Enter export password for PKCS12 file (press Enter for no password): " P12_PASSWORD
    echo ""
    
    if [ -z "$P12_PASSWORD" ]; then
        openssl pkcs12 -export \
            -in "$CERT_FILE" \
            -inkey "$KEY_FILE" \
            -out "$P12_FILE" \
            -passout pass: 2>/dev/null
    else
        openssl pkcs12 -export \
            -in "$CERT_FILE" \
            -inkey "$KEY_FILE" \
            -out "$P12_FILE" \
            -passout pass:"$P12_PASSWORD" 2>/dev/null
    fi
    
    chmod 600 "$P12_FILE"
    print_success "PKCS12 file created: $P12_FILE"
}

# Verify certificates
verify_certificates() {
    print_header "Certificate Verification"
    
    print_info "Certificate details:"
    openssl x509 -in "$CERT_FILE" -noout -subject -issuer -dates
    
    echo ""
    print_info "Certificate fingerprints:"
    echo "  MD5:    $(openssl x509 -in "$CERT_FILE" -noout -fingerprint -md5 | cut -d'=' -f2)"
    echo "  SHA1:   $(openssl x509 -in "$CERT_FILE" -noout -fingerprint -sha1 | cut -d'=' -f2)"
    echo "  SHA256: $(openssl x509 -in "$CERT_FILE" -noout -fingerprint -sha256 | cut -d'=' -f2)"
    
    echo ""
    print_info "Verifying certificate and key match..."
    CERT_MODULUS=$(openssl x509 -noout -modulus -in "$CERT_FILE" | openssl md5)
    KEY_MODULUS=$(openssl rsa -noout -modulus -in "$KEY_FILE" 2>/dev/null | openssl md5)
    
    if [ "$CERT_MODULUS" == "$KEY_MODULUS" ]; then
        print_success "Certificate and key match!"
    else
        print_error "Certificate and key DO NOT match!"
        exit 1
    fi
    
    # Show SANs if present
    if openssl x509 -in "$CERT_FILE" -noout -text | grep -q "Subject Alternative Name"; then
        echo ""
        print_info "Subject Alternative Names:"
        openssl x509 -in "$CERT_FILE" -noout -text | grep -A1 "Subject Alternative Name" | tail -1 | sed 's/^[[:space:]]*/  /'
    fi
}

# Create README file
create_readme() {
    README_FILE="$CERT_DIR/README.md"
    
    cat > "$README_FILE" <<EOF
# IMAP TLS Certificates

Generated on: $(date)

## Certificate Information
- **Domain**: $DOMAIN
- **Organization**: $ORG
- **Validity**: $CERT_VALIDITY_DAYS days
- **Key Size**: $KEY_SIZE bits

## Files Generated

### 1. Private Key
- **File**: \`imap-key.pem\`
- **Description**: RSA private key (Keep this secure!)
- **Permissions**: 600 (read/write owner only)

### 2. Certificate
- **File**: \`imap-cert.pem\`
- **Description**: X.509 certificate
- **Permissions**: 644 (readable by all)

### 3. Combined PEM
- **File**: \`imap-combined.pem\`
- **Description**: Certificate + Private Key in one file
- **Use**: Some servers prefer this format
- **Permissions**: 600

### 4. PKCS12 Format
- **File**: \`imap-cert.p12\`
- **Description**: Certificate bundle for Windows/Mac
- **Permissions**: 600

### 5. OpenSSL Config
- **File**: \`openssl.cnf\`
- **Description**: Configuration used to generate certificates

## Usage in Go IMAP Server

\`\`\`go
import (
    "crypto/tls"
    "log"
)

func main() {
    cert, err := tls.LoadX509KeyPair("./certs/imap-cert.pem", "./certs/imap-key.pem")
    if err != nil {
        log.Fatal(err)
    }
    
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    }
    
    // Use with your IMAP server
}
\`\`\`

## Testing the Certificate

### Test with OpenSSL (IMAPS - port 993)
\`\`\`bash
openssl s_client -connect $DOMAIN:993 -showcerts
\`\`\`

### Test with OpenSSL (IMAP STARTTLS - port 143)
\`\`\`bash
openssl s_client -connect $DOMAIN:143 -starttls imap
\`\`\`

### Verify certificate locally
\`\`\`bash
openssl x509 -in imap-cert.pem -text -noout
\`\`\`

## Security Notes

⚠️ **IMPORTANT**: This is a self-signed certificate!

- Email clients will show security warnings
- For production, use certificates from a trusted CA (Let's Encrypt, etc.)
- Keep \`imap-key.pem\` secure - never share it!
- Set proper file permissions (already done by script)

## Certificate Renewal

This certificate is valid for $CERT_VALIDITY_DAYS days.

**Expiration Date**: $(openssl x509 -in "$CERT_FILE" -noout -enddate | cut -d'=' -f2)

To renew, run this script again.

## Trusting the Certificate (For Testing)

### Linux
\`\`\`bash
sudo cp imap-cert.pem /usr/local/share/ca-certificates/imap-cert.crt
sudo update-ca-certificates
\`\`\`

### macOS
\`\`\`bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain imap-cert.pem
\`\`\`

### Windows
Import \`imap-cert.p12\` via Certificate Manager (certmgr.msc)

## Environment Variables for Your App

\`\`\`bash
export TLS_CERT_FILE="$PWD/certs/imap-cert.pem"
export TLS_KEY_FILE="$PWD/certs/imap-key.pem"
\`\`\`
EOF
    
    print_success "README created: $README_FILE"
}

# Print summary
print_summary() {
    print_header "Certificate Generation Complete!"
    
    echo "📁 Certificate directory: $CERT_DIR"
    echo ""
    echo "📄 Generated files:"
    echo "   - imap-key.pem       (Private Key)"
    echo "   - imap-cert.pem      (Certificate)"
    echo "   - imap-combined.pem  (Combined)"
    echo "   - imap-cert.p12      (PKCS12)"
    echo "   - openssl.cnf        (Config)"
    echo "   - README.md          (Documentation)"
    echo ""
    
    print_success "All certificates generated successfully!"
    echo ""
    
    print_warning "IMPORTANT NOTES:"
    echo "  1. This is a SELF-SIGNED certificate"
    echo "  2. Email clients will show security warnings"
    echo "  3. For production, use Let's Encrypt or commercial CA"
    echo "  4. Keep imap-key.pem secure - never share it!"
    echo ""
    
    print_info "Certificate valid until:"
    openssl x509 -in "$CERT_FILE" -noout -enddate | cut -d'=' -f2
    echo ""
    
    print_info "Next steps:"
    echo "  1. Read $CERT_DIR/README.md for usage instructions"
    echo "  2. Configure your IMAP server to use these certificates"
    echo "  3. Test with: openssl s_client -connect $DOMAIN:993"
    echo ""
}

# Main execution
main() {
    print_header "IMAP TLS Certificate Generator"
    
    # Check prerequisites
    check_openssl
    
    # Create certificate directory
    if [ ! -d "$CERT_DIR" ]; then
        mkdir -p "$CERT_DIR"
        print_success "Created directory: $CERT_DIR"
    else
        print_warning "Directory already exists: $CERT_DIR"
        read -p "Overwrite existing certificates? (y/n): " OVERWRITE
        if [[ "$OVERWRITE" != "y" && "$OVERWRITE" != "Y" ]]; then
            print_info "Aborted by user"
            exit 0
        fi
    fi
    
    # Get certificate information
    get_certificate_info
    
    # Generate certificates
    create_openssl_config
    generate_private_key
    generate_certificate
    create_combined_pem
    create_pkcs12
    
    # Verify
    verify_certificates
    
    # Create documentation
    create_readme
    
    # Show summary
    print_summary
}

# Run main function
main