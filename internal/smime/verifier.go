package smime

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"time"

	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
	"go.mozilla.org/pkcs7"
)

// Verifier handles S/MIME signature verification
type Verifier struct {
	store *Store
	log   zerolog.Logger
}

// NewVerifier creates a new S/MIME verifier
func NewVerifier(store *Store, log zerolog.Logger) *Verifier {
	return &Verifier{
		store: store,
		log:   log,
	}
}

// VerifyAndUnwrap detects S/MIME signed content, verifies the signature,
// caches the sender cert, and returns the verification result plus the
// unwrapped inner body (if any). If the message is not S/MIME signed or
// verification encounters a fatal parse error, it returns (nil, nil).
func (v *Verifier) VerifyAndUnwrap(raw []byte) (*SignatureResult, []byte) {
	// Parse the message to find Content-Type
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		if headerEnd == -1 {
			return nil, nil
		}
	}

	headers := raw[:headerEnd]
	ct := extractHeaderValue(headers, "Content-Type")
	if ct == "" {
		return nil, nil
	}

	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, nil
	}

	// Handle multipart/signed with pkcs7-signature protocol
	if strings.EqualFold(mediaType, "multipart/signed") {
		protocol := params["protocol"]
		if !strings.EqualFold(protocol, "application/pkcs7-signature") &&
			!strings.EqualFold(protocol, "application/x-pkcs7-signature") {
			return nil, nil
		}
		return v.verifyMultipartSigned(raw, params)
	}

	// Handle application/pkcs7-mime (opaque signed)
	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		if strings.EqualFold(smimeType, "signed-data") {
			return v.verifyOpaqueSigned(raw)
		}
	}

	return nil, nil
}

// verifyMultipartSigned handles clear-signed messages (multipart/signed)
func (v *Verifier) verifyMultipartSigned(raw []byte, params map[string]string) (*SignatureResult, []byte) {
	boundary := params["boundary"]
	if boundary == "" {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "missing boundary parameter",
		}, nil
	}

	// Find the body after headers
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find header/body boundary",
		}, nil
	}

	body := raw[bodyStart:]

	// RFC 2046 §5.1: Extract the raw bytes of the first body part.
	// For detached signature verification the signed content MUST be the
	// exact bytes between the opening boundary's trailing CRLF and the
	// CRLF that introduces the next boundary delimiter. We must NOT
	// re-parse the part headers because any re-serialization (e.g.
	// header reordering) would invalidate the signature.
	boundaryLine := []byte("--" + boundary)

	// Locate the opening boundary delimiter
	firstIdx := bytes.Index(body, boundaryLine)
	if firstIdx == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find opening boundary",
		}, nil
	}

	// Content starts right after the boundary line's CRLF
	contentStart := firstIdx + len(boundaryLine)
	if contentStart+2 <= len(body) && body[contentStart] == '\r' && body[contentStart+1] == '\n' {
		contentStart += 2
	} else if contentStart < len(body) && body[contentStart] == '\n' {
		contentStart++
	}

	// Find the next boundary delimiter. Per RFC 2046, the CRLF
	// preceding the delimiter line belongs to the boundary, not to the
	// encapsulated part.
	rest := body[contentStart:]
	delim := []byte("\r\n--" + boundary)
	endIdx := bytes.Index(rest, delim)
	if endIdx == -1 {
		// Try with bare LF
		delim = []byte("\n--" + boundary)
		endIdx = bytes.Index(rest, delim)
		if endIdx == -1 {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: "cannot find closing boundary for signed part",
			}, nil
		}
	}

	signedContent := rest[:endIdx]

	// Extract the signature from the second part. We use
	// multipart.Reader here because the exact bytes of the signature
	// part are irrelevant — we only need the decoded PKCS#7 data.
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	// Skip the first part (NextPart consumes the previous part internally)
	if p, err := reader.NextPart(); err == nil {
		_, _ = io.Copy(io.Discard, p)
	}

	// Second part: the PKCS#7 detached signature
	sigPart, err := reader.NextPart()
	if err != nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: fmt.Sprintf("failed to read signature part: %v", err),
		}, nil
	}
	sigBytes, err := io.ReadAll(sigPart)
	if err != nil {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: fmt.Sprintf("failed to read signature bytes: %v", err),
		}, nil
	}

	// The signature part is typically base64-encoded per its
	// Content-Transfer-Encoding header.  Try parsing as raw DER first;
	// if that fails, base64-decode and retry.
	p7, err := pkcs7.Parse(sigBytes)
	if err != nil {
		// Strip whitespace/line breaks from base64 data
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, sigBytes)
		decoded, decErr := base64.StdEncoding.DecodeString(string(cleaned))
		if decErr != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 signature: %v", err),
			}, nil
		}
		p7, err = pkcs7.Parse(decoded)
		if err != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 signature after base64 decode: %v", err),
			}, nil
		}
	}

	// Attach the raw signed content for detached signature verification
	p7.Content = signedContent

	// Verify the signature
	result := v.verifyPKCS7(p7)

	return result, signedContent
}

// verifyOpaqueSigned handles opaque signed messages (application/pkcs7-mime)
func (v *Verifier) verifyOpaqueSigned(raw []byte) (*SignatureResult, []byte) {
	// Find body after headers
	headerEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyStart := headerEnd + 4
	if headerEnd == -1 {
		headerEnd = bytes.Index(raw, []byte("\n\n"))
		bodyStart = headerEnd + 2
	}
	if headerEnd == -1 {
		return &SignatureResult{
			Status:       StatusInvalid,
			ErrorMessage: "cannot find header/body boundary",
		}, nil
	}

	body := raw[bodyStart:]

	// Try parsing as DER first; if that fails, base64-decode and retry
	p7, err := pkcs7.Parse(body)
	if err != nil {
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, body)
		decoded, decErr := base64.StdEncoding.DecodeString(string(cleaned))
		if decErr != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 data: %v", err),
			}, nil
		}
		p7, err = pkcs7.Parse(decoded)
		if err != nil {
			return &SignatureResult{
				Status:       StatusInvalid,
				ErrorMessage: fmt.Sprintf("failed to parse PKCS#7 data after base64 decode: %v", err),
			}, nil
		}
	}

	result := v.verifyPKCS7(p7)

	// For opaque signed, the inner content is embedded in p7.Content
	return result, p7.Content
}

// verifyPKCS7 verifies a parsed PKCS#7 object and caches the signer cert
func (v *Verifier) verifyPKCS7(p7 *pkcs7.PKCS7) *SignatureResult {
	// Try verification against system trust roots
	err := p7.Verify()

	// Extract signer information regardless of verification result
	signerEmail, signerName := v.extractSignerInfo(p7)

	if err != nil {
		// Distinguish between "untrusted CA" and "truly invalid signature"
		// If we can verify without trust check, the signature is valid but signer is unknown
		errStr := err.Error()

		// Common certificate verification errors indicate untrusted/expired certs
		if strings.Contains(errStr, "certificate signed by unknown authority") ||
			strings.Contains(errStr, "x509: certificate") {
			// Try to determine if cert is expired
			if v.isSignerCertExpired(p7) {
				v.cacheSenderCert(p7, signerEmail)
				return &SignatureResult{
					Status:       StatusExpiredCert,
					SignerEmail:  signerEmail,
					SignerName:   signerName,
					ErrorMessage: "signer certificate has expired",
				}
			}
			// Check if the leaf cert is self-signed (Issuer == Subject)
			v.cacheSenderCert(p7, signerEmail)
			if v.isSignerCertSelfSigned(p7) {
				return &SignatureResult{
					Status:       StatusSelfSigned,
					SignerEmail:  signerEmail,
					SignerName:   signerName,
					ErrorMessage: "self-signed certificate",
				}
			}
			return &SignatureResult{
				Status:       StatusUnknownSigner,
				SignerEmail:  signerEmail,
				SignerName:   signerName,
				ErrorMessage: fmt.Sprintf("unverified signer: %v", err),
			}
		}

		// Truly invalid signature
		return &SignatureResult{
			Status:       StatusInvalid,
			SignerEmail:  signerEmail,
			SignerName:   signerName,
			ErrorMessage: fmt.Sprintf("signature verification failed: %v", err),
		}
	}

	// Signature verified successfully — but pkcs7.Verify() trusts certs
	// embedded in the PKCS7 structure, so a self-signed cert will pass.
	// Check for self-signed before reporting as fully trusted.
	v.cacheSenderCert(p7, signerEmail)
	if v.isSignerCertSelfSigned(p7) {
		return &SignatureResult{
			Status:       StatusSelfSigned,
			SignerEmail:  signerEmail,
			SignerName:   signerName,
			ErrorMessage: "self-signed certificate",
		}
	}
	return &SignatureResult{
		Status:      StatusSigned,
		SignerEmail: signerEmail,
		SignerName:  signerName,
	}
}

// extractSignerInfo gets the email and common name from the first signer certificate
func (v *Verifier) extractSignerInfo(p7 *pkcs7.PKCS7) (email, name string) {
	if len(p7.Certificates) == 0 {
		return "", ""
	}

	// Find the actual signer cert (first cert with EmailAddresses typically)
	for _, cert := range p7.Certificates {
		if len(cert.EmailAddresses) > 0 {
			return cert.EmailAddresses[0], cert.Subject.CommonName
		}
	}

	// Fall back to first certificate's CN
	return "", p7.Certificates[0].Subject.CommonName
}

// isSignerCertExpired checks if any signer certificate in the PKCS#7 is expired
func (v *Verifier) isSignerCertExpired(p7 *pkcs7.PKCS7) bool {
	now := time.Now()
	for _, cert := range p7.Certificates {
		if !cert.IsCA && now.After(cert.NotAfter) {
			return true
		}
	}
	return false
}

// isSignerCertSelfSigned checks if the leaf signer certificate is self-signed
func (v *Verifier) isSignerCertSelfSigned(p7 *pkcs7.PKCS7) bool {
	for _, cert := range p7.Certificates {
		if !cert.IsCA {
			return bytes.Equal(cert.RawIssuer, cert.RawSubject)
		}
	}
	if len(p7.Certificates) > 0 {
		c := p7.Certificates[0]
		return bytes.Equal(c.RawIssuer, c.RawSubject)
	}
	return false
}

// cacheSenderCert stores the signer's leaf certificate for future reference
func (v *Verifier) cacheSenderCert(p7 *pkcs7.PKCS7, email string) {
	if email == "" || len(p7.Certificates) == 0 {
		return
	}

	// Find the leaf (non-CA) certificate
	var leafCert *x509.Certificate
	for _, cert := range p7.Certificates {
		if !cert.IsCA {
			leafCert = cert
			break
		}
	}
	if leafCert == nil {
		leafCert = p7.Certificates[0]
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: leafCert.Raw,
	})

	if err := v.store.CacheSenderCert(email, string(certPEM)); err != nil {
		v.log.Warn().Err(err).Str("email", logging.RedactEmail(email)).Msg("Failed to cache sender certificate")
	}
}

// extractHeaderValue extracts a header value from raw headers (case-insensitive)
func extractHeaderValue(headers []byte, name string) string {
	lines := strings.Split(string(headers), "\n")
	lowerName := strings.ToLower(name)

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		headerName := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		if headerName != lowerName {
			continue
		}

		value := strings.TrimSpace(line[colonIdx+1:])

		// Handle multi-line headers (continuation lines start with whitespace)
		for j := i + 1; j < len(lines); j++ {
			nextLine := strings.TrimRight(lines[j], "\r")
			if len(nextLine) == 0 {
				break
			}
			if nextLine[0] == ' ' || nextLine[0] == '\t' {
				value += " " + strings.TrimSpace(nextLine)
			} else {
				break
			}
		}

		return value
	}
	return ""
}

// IsSMIMEEncrypted checks if a Content-Type header indicates S/MIME encrypted content
func IsSMIMEEncrypted(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		return strings.EqualFold(smimeType, "enveloped-data")
	}

	return false
}

// IsSMIMESigned checks if a Content-Type header indicates S/MIME signed content
func IsSMIMESigned(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	// multipart/signed with pkcs7 protocol
	if strings.EqualFold(mediaType, "multipart/signed") {
		protocol := params["protocol"]
		return strings.EqualFold(protocol, "application/pkcs7-signature") ||
			strings.EqualFold(protocol, "application/x-pkcs7-signature")
	}

	// application/pkcs7-mime with signed-data type
	if strings.EqualFold(mediaType, "application/pkcs7-mime") ||
		strings.EqualFold(mediaType, "application/x-pkcs7-mime") {
		smimeType := params["smime-type"]
		return strings.EqualFold(smimeType, "signed-data")
	}

	return false
}
