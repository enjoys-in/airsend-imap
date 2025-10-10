# IMAP TLS Certificates

Generated on: Sat, Oct 11, 2025  2:19:15 AM

## Certificate Information
- **Domain**: localhost
- **Organization**: ENJOYS
- **Validity**: 90 days
- **Key Size**: 4096 bits

## Files Generated

### 1. Private Key
- **File**: `imap-key.pem`
- **Description**: RSA private key (Keep this secure!)
- **Permissions**: 600 (read/write owner only)

### 2. Certificate
- **File**: `imap-cert.pem`
- **Description**: X.509 certificate
- **Permissions**: 644 (readable by all)

### 3. Combined PEM
- **File**: `imap-combined.pem`
- **Description**: Certificate + Private Key in one file
- **Use**: Some servers prefer this format
- **Permissions**: 600

### 4. PKCS12 Format
- **File**: `imap-cert.p12`
- **Description**: Certificate bundle for Windows/Mac
- **Permissions**: 600

### 5. OpenSSL Config
- **File**: `openssl.cnf`
- **Description**: Configuration used to generate certificates

## Usage in Go IMAP Server

```go
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
```

## Testing the Certificate

### Test with OpenSSL (IMAPS - port 993)
```bash
openssl s_client -connect localhost:993 -showcerts
```

### Test with OpenSSL (IMAP STARTTLS - port 143)
```bash
openssl s_client -connect localhost:143 -starttls imap
```

### Verify certificate locally
```bash
openssl x509 -in imap-cert.pem -text -noout
```

## Security Notes

⚠️ **IMPORTANT**: This is a self-signed certificate!

- Email clients will show security warnings
- For production, use certificates from a trusted CA (Let's Encrypt, etc.)
- Keep `imap-key.pem` secure - never share it!
- Set proper file permissions (already done by script)

## Certificate Renewal

This certificate is valid for 90 days.

**Expiration Date**: Jan  8 20:49:10 2026 GMT

To renew, run this script again.

## Trusting the Certificate (For Testing)

### Linux
```bash
sudo cp imap-cert.pem /usr/local/share/ca-certificates/imap-cert.crt
sudo update-ca-certificates
```

### macOS
```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain imap-cert.pem
```

### Windows
Import `imap-cert.p12` via Certificate Manager (certmgr.msc)

## Environment Variables for Your App

```bash
export TLS_CERT_FILE="/f/private/ENJOYS/airsend/go-imap/certs/imap-cert.pem"
export TLS_KEY_FILE="/f/private/ENJOYS/airsend/go-imap/certs/imap-key.pem"
```
