package crypto

import "github.com/ProtonMail/go-crypto/openpgp/packet"

type EncryptionProfile interface {
	EncryptionConfig() *packet.Config
	CompressionConfig() *packet.Config
}

// PGPEncryption is an interface for encrypting messages with GopenPGP.
// Use an EncryptionHandleBuilder to create a PGPEncryption handle.
type PGPEncryption interface {
	// EncryptingWriter returns a wrapper around underlying output Writer,
	// such that any write-operation via the wrapper results in a write to an encrypted pgp message.
	// If the output Writer is of type PGPSplitWriter, the output can be split to multiple writers
	// for different parts of the message. For example to write key packets and encrypted data packets
	// to different writers or to write a detached signature separately.
	// Metadata contains additional metadata about the plaintext, if nil defaults are used.
	// The returned pgp message WriteCloser must be closed after the plaintext has been written.
	EncryptingWriter(output Writer, meta *LiteralMetadata) (WriteCloser, error)
	// Encrypt encrypts a plaintext message.
	// Metadata contains additional metadata about the plaintext, if nil defaults are used.
	Encrypt(message []byte, meta *LiteralMetadata) (*PGPMessage, error)
	// EncryptSessionKey encrypts a session key with the encryption handle.
	// To encrypt a session key, the handle must contain either recipients or a password.
	EncryptSessionKey(sessionKey *SessionKey) ([]byte, error)
	// ClearPrivateParams clears all private key material contained in EncryptionHandle from memory,
	ClearPrivateParams()
}

type Writer interface {
	Write(b []byte) (n int, err error)
}

type WriteCloser interface {
	Write(b []byte) (n int, err error)
	Close() (err error)
}

type PGPSplitWriter interface {
	Writer
	Keys() Writer
	Signature() Writer
}