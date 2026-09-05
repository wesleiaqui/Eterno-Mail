package pgp

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

// Verifier handles PGP/MIME signature verification
type Verifier struct {
	store *Store
	log   zerolog.Logger
}

// NewVerifier creates a new PGP verifier
func NewVerifier(store *Store, log zerolog.Logger) *Verifier {
	return &Verifier{
		store: store,
		log:   log,
	}
}

// VerifyAndUnwrap detects PGP/MIME signed content, verifies the signature,
// caches the sender key, and returns the verification result plus the
// unwrapped inner body. If the message is not PGP signed, returns (nil, nil).
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

	// Handle multipart/signed with pgp-signature protocol
	if !strings.EqualFold(mediaType, "multipart/signed") {
		return nil, nil
	}

	protocol := params["protocol"]
	if !strings.EqualFold(protocol, "application/pgp-signature") {
		return nil, nil
	}

	return v.verifyMultipartSigned(raw, params)
}

// verifyMultipartSigned handles PGP/MIME signed messages (multipart/signed)
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
	// CRLF that introduces the next boundary delimiter.
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

	// Find the next boundary delimiter
	rest := body[contentStart:]
	delim := []byte("\r\n--" + boundary)
	endIdx := bytes.Index(rest, delim)
	if endIdx == -1 {
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

	// Extract the signature from the second part using multipart.Reader
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	// Skip the first part
	if p, err := reader.NextPart(); err == nil {
		_, _ = io.Copy(io.Discard, p)
	}

	// Second part: the PGP signature
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

	// Build a keyring from all known keys (own keys + sender keys)
	keyring, err := v.buildKeyring()
	if err != nil {
		v.log.Warn().Err(err).Msg("Failed to build keyring for verification")
		return &SignatureResult{
			Status:       StatusUnknownKey,
			ErrorMessage: "failed to build keyring",
		}, signedContent
	}

	// Verify the detached signature
	signer, err := openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewReader(signedContent), bytes.NewReader(sigBytes), nil)
	if err != nil {
		// Try to extract key ID from the signature for diagnostics
		keyID := extractKeyIDFromSignature(sigBytes)

		// Check if the error is because we don't have the key
		if strings.Contains(err.Error(), "signature made by unknown entity") ||
			strings.Contains(err.Error(), "key not found") {
			return &SignatureResult{
				Status:       StatusUnknownKey,
				SignerKeyID:  keyID,
				ErrorMessage: "signing key not found",
			}, signedContent
		}

		return &SignatureResult{
			Status:       StatusInvalid,
			SignerKeyID:  keyID,
			ErrorMessage: fmt.Sprintf("signature verification failed: %v", err),
		}, signedContent
	}

	// Signature verified — extract signer info
	signerEmail := ExtractEmailFromKey(signer)
	signerKeyID := fmt.Sprintf("%016X", signer.PrimaryKey.KeyId)

	// Cache the sender's public key
	v.cacheSenderKey(signer, signerEmail)

	// Check if the key is expired
	if IsKeyExpired(signer) {
		return &SignatureResult{
			Status:       StatusExpiredKey,
			SignerEmail:  signerEmail,
			SignerKeyID:  signerKeyID,
			ErrorMessage: "signing key has expired",
		}, signedContent
	}

	return &SignatureResult{
		Status:      StatusSigned,
		SignerEmail: signerEmail,
		SignerKeyID: signerKeyID,
	}, signedContent
}

// buildKeyring creates an openpgp.EntityList from all known keys
func (v *Verifier) buildKeyring() (openpgp.EntityList, error) {
	var keyring openpgp.EntityList

	// Add all sender keys
	senderKeys, err := v.store.ListAllSenderKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to list sender keys: %w", err)
	}

	for _, sk := range senderKeys {
		armored, getErr := v.store.GetSenderKeyArmored(sk.ID)
		if getErr != nil {
			continue
		}
		entities, parseErr := ParseArmoredKey(armored)
		if parseErr != nil {
			continue
		}
		keyring = append(keyring, entities...)
	}

	return keyring, nil
}

// cacheSenderKey stores the signer's public key for future reference
func (v *Verifier) cacheSenderKey(entity *openpgp.Entity, email string) {
	if email == "" {
		return
	}

	armored, err := ArmorPublicKey(entity)
	if err != nil {
		v.log.Warn().Err(err).Str("email", logging.RedactEmail(email)).Msg("Failed to armor sender key for caching")
		return
	}

	if err := v.store.CacheSenderKey(email, armored, "message"); err != nil {
		v.log.Warn().Err(err).Str("email", logging.RedactEmail(email)).Msg("Failed to cache sender key")
	}
}

// extractKeyIDFromSignature attempts to extract a key ID from a PGP signature
func extractKeyIDFromSignature(sigData []byte) string {
	// This is a best-effort extraction; parsing may fail
	return ""
}

// IsPGPSigned checks if a Content-Type header indicates PGP/MIME signed content
func IsPGPSigned(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	if strings.EqualFold(mediaType, "multipart/signed") {
		protocol := params["protocol"]
		return strings.EqualFold(protocol, "application/pgp-signature")
	}

	return false
}

// IsPGPEncrypted checks if a Content-Type header indicates PGP/MIME encrypted content
func IsPGPEncrypted(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	if strings.EqualFold(mediaType, "multipart/encrypted") {
		protocol := params["protocol"]
		return strings.EqualFold(protocol, "application/pgp-encrypted")
	}

	return false
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

// extractHeader extracts a header value from raw headers
func extractHeader(headers []byte, name string) string {
	return extractHeaderValue(headers, name)
}

// writeFilteredHeaders writes headers from the original message, excluding
// Content-Type and Content-Transfer-Encoding (which will be replaced)
func writeFilteredHeaders(buf *bytes.Buffer, headers []byte) {
	lines := strings.Split(string(headers), "\n")
	skipHeaders := map[string]bool{
		"content-type":              true,
		"content-transfer-encoding": true,
		"mime-version":              true,
	}

	skipContinuation := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}

		// Check if this is a continuation line
		if line[0] == ' ' || line[0] == '\t' {
			if skipContinuation {
				continue
			}
			buf.WriteString(line + "\r\n")
			continue
		}

		// Check if this header should be skipped
		colonIdx := strings.Index(line, ":")
		if colonIdx != -1 {
			headerName := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
			if skipHeaders[headerName] {
				skipContinuation = true
				continue
			}
		}

		skipContinuation = false
		buf.WriteString(line + "\r\n")
	}

	// Always add MIME-Version for signed messages
	buf.WriteString("MIME-Version: 1.0\r\n")
}
