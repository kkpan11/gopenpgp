package crypto

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/gopenpgp/v3/armor"
	"github.com/ProtonMail/gopenpgp/v3/constants"
	"github.com/ProtonMail/gopenpgp/v3/internal"
	"github.com/stretchr/testify/assert"
)

const signedPlainText = "Signed message\n"

var textSignature, binSignature []byte
var signatureTest = regexp.MustCompile("(?s)^-----BEGIN PGP SIGNATURE-----.*-----END PGP SIGNATURE-----$")

func getSignatureType(sig []byte) (packet.SignatureType, error) {
	sigPacket, err := getSignaturePacket(sig)
	if err != nil {
		return 0, err
	}
	return sigPacket.SigType, nil
}

func testSignerText() PGPSign {
	signer, _ := testPGP.Sign().
		SigningKeys(keyRingTestPrivate).
		Utf8().
		Detached().
		New()
	return signer
}

func testSigner() PGPSign {
	signer, _ := testPGP.Sign().
		SigningKeys(keyRingTestPrivate).
		Detached().
		New()
	return signer
}

func testVerifier() PGPVerify {
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerifyTime(testTime).
		New()
	return verifier
}

func TestSignTextDetached(t *testing.T) {
	var err error

	textSignature, err = testSignerText().Sign([]byte(signedPlainText), Bytes)
	if err != nil {
		t.Fatal("Cannot generate signature:", err)
	}

	armoredSignature, err := armor.ArmorPGPSignatureBinary(textSignature)
	if err != nil {
		t.Fatal("Cannot armor signature:", err)
	}

	sigType, err := getSignatureType(textSignature)

	if err != nil {
		t.Fatal("Cannot get signature type:", err)
	}

	if sigType != packet.SigTypeText {
		t.Fatal("Signature type was not text")
	}

	assert.Regexp(t, signatureTest, string(armoredSignature))

	verificationError, err := testVerifier().VerifyDetached([]byte(signedPlainText), textSignature, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err = verificationError.SignatureError(); err != nil {
		t.Fatal("Cannot verify plaintext signature:", err)
	}

	fakeMessage := []byte("wrong text")
	verificationError, err = testVerifier().VerifyDetached(fakeMessage, textSignature, Bytes)
	if err != nil {
		t.Fatal(err)
	}

	checkVerificationError(t, verificationError.SignatureError(), constants.SIGNATURE_FAILED)
}

func TestSignNonUtf8Text(t *testing.T) {
	var err error

	var nonUft8, _ = hex.DecodeString("fc80808080af")

	textSignature, err = testSignerText().Sign(nonUft8, Bytes)
	if !errors.Is(err, internal.ErrIncorrectUtf8) {
		t.Fatal("Expected not valid utf8 error")
	}
}

func checkVerificationError(t *testing.T, err error, expectedStatus int) {
	if err == nil {
		t.Fatalf("Expected a verification error")
	}
	castedErr := &SignatureVerificationError{}
	isType := errors.As(err, castedErr)
	if !isType {
		t.Fatalf("Error was not a verification error: %v", err)
	}
	if castedErr.Status != expectedStatus {
		t.Fatalf("Expected status to be %d got %d", expectedStatus, castedErr.Status)
	}
}

func TestSignBinDetached(t *testing.T) {
	var err error

	binSignature, err = testSigner().Sign([]byte(signedPlainText), Bytes)
	if err != nil {
		t.Fatal("Cannot generate signature:", err)
	}

	armoredSignature, err := armor.ArmorPGPSignatureBinary(binSignature)
	if err != nil {
		t.Fatal("Cannot armor signature:", err)
	}

	sigType, err := getSignatureType(binSignature)

	if err != nil {
		t.Fatal("Cannot get signature type:", err)
	}

	if sigType != packet.SigTypeBinary {
		t.Fatal("Signature type was not binary")
	}

	assert.Regexp(t, signatureTest, string(armoredSignature))

	verificationError, err := testVerifier().VerifyDetached([]byte(signedPlainText), binSignature, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err = verificationError.SignatureError(); err != nil {
		t.Fatal("Cannot verify binary signature:", err)
	}
}

func Test_KeyRing_GetVerifiedSignatureTimestampSuccess(t *testing.T) {
	message := []byte(testMessage)
	var timeLocal int64 = 1600000000
	signer, _ := testPGP.Sign().
		SigningKeys(keyRingTestPrivate).
		SignTime(timeLocal).
		Detached().
		New()
	signature, err := signer.Sign(message, Bytes)
	if err != nil {
		t.Errorf("Got an error while generating the signature: %v", err)
	}
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerifyTime(timeLocal).
		New()
	verificationResult, err := verifier.VerifyDetached(message, signature, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	actualTime := verificationResult.SignatureCreationTime()
	if err != nil {
		t.Errorf("Got an error while parsing the signature creation time: %v", err)
	}
	if timeLocal != actualTime {
		t.Errorf("Expected creation time to be %d, got %d", timeLocal, actualTime)
	}
}

func Test_KeyRing_GetVerifiedSignatureWithTwoKeysTimestampSuccess(t *testing.T) {
	publicKey1Armored, err := os.ReadFile("testdata/signature/publicKey1")
	if err != nil {
		t.Errorf("Couldn't read the public key file: %v", err)
	}
	publicKey1 := parseKey(t, string(publicKey1Armored))
	publicKey2Armored, err := os.ReadFile("testdata/signature/publicKey2")
	if err != nil {
		t.Errorf("Couldn't read the public key file: %v", err)
	}
	publicKey2 := parseKey(t, string(publicKey2Armored))
	message := []byte("hello world")
	signatureArmored, err := os.ReadFile("testdata/signature/detachedSigSignedTwice")
	if err != nil {
		t.Errorf("Couldn't read the signature file: %v", err)
	}
	signature, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Errorf("Got an error while parsing the signature: %v", err)
	}
	time1 := getTimestampOfIssuer(signature, publicKey1.GetKeyID())
	time2 := getTimestampOfIssuer(signature, publicKey2.GetKeyID())
	keyRing, err := NewKeyRing(publicKey1)
	if err != nil {
		t.Errorf("Got an error while building the key ring: %v", err)
	}
	err = keyRing.AddKey(publicKey2)
	if err != nil {
		t.Errorf("Got an error while adding key 2 to the key ring: %v", err)
	}

	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRing).
		DisableVerifyTimeCheck().
		New()

	verificationResult, err := verifier.VerifyDetached(message, signature, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	actualTime := verificationResult.SignatureCreationTime()
	otherTime := verificationResult.Signatures[1].Signature.CreationTime.Unix()
	if err != nil {
		t.Errorf("Got an error while parsing the signature creation time: %v", err)
	}

	if time2 != otherTime {
		t.Errorf("Expected creation time to be %d, got %d", otherTime, time2)
	}
	if time1 != actualTime {
		t.Errorf("Expected creation time to be %d, got %d", actualTime, time1)
	}
}

func parseKey(t *testing.T, keyArmored string) *Key {
	key, err := NewKeyFromArmored(keyArmored)
	if err != nil {
		t.Errorf("Couldn't parse key: %v", err)
		return nil
	}
	return key
}

func getTimestampOfIssuer(signature []byte, keyID uint64) int64 {
	packets := packet.NewReader(bytes.NewReader(signature))
	var err error
	var p packet.Packet
	for {
		p, err = packets.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}
		sigPacket, ok := p.(*packet.Signature)
		if !ok {
			continue
		}
		var outBuf bytes.Buffer
		err = sigPacket.Serialize(&outBuf)
		if err != nil {
			continue
		}
		if *sigPacket.IssuerKeyId == keyID {
			return sigPacket.CreationTime.Unix()
		}
	}
	return -1
}

func Test_KeyRing_GetVerifiedSignatureTimestampError(t *testing.T) {
	message := []byte(testMessage)
	var timeLocal int64 = 1600000000
	signer, _ := testPGP.Sign().
		SignTime(timeLocal).
		SigningKeys(keyRingTestPrivate).
		Detached().
		New()
	signature, err := signer.Sign(message, Bytes)
	if err != nil {
		t.Errorf("Got an error while generating the signature: %v", err)
	}
	messageCorrupted := []byte("Ciao world!")
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerifyTime(timeLocal).
		New()
	verificationResult, err := verifier.VerifyDetached(messageCorrupted, signature, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if verificationResult.SignatureError() == nil {
		t.Errorf("Expected an error while parsing the creation time of a wrong signature, got nil")
	}
}

func Test_SignDetachedWithNonCriticalContext(t *testing.T) {
	// given

	context := NewSigningContext(
		"test-context",
		false,
	)
	signer, _ := testPGP.Sign().
		SigningKeys(keyRingTestPrivate).
		SigningContext(context).
		Detached().
		New()
	// when
	signature, err := signer.Sign([]byte(testMessage), Bytes)
	// then
	if err != nil {
		t.Fatal(err)
	}
	p, err := packet.Read(bytes.NewReader(signature))
	if err != nil {
		t.Fatal(err)
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		t.Fatal("Packet was not a signature")
	}
	notations := sig.Notations
	if len(notations) != 2 {
		t.Fatal("Wrong number of notations")
	}
	notation := notations[0]
	if notation.Name != constants.SignatureContextName {
		t.Fatalf("Expected notation name to be %s, got %s", constants.SignatureContextName, notation.Name)
	}
	if string(notation.Value) != context.Value {
		t.Fatalf("Expected notation value to be %s, got %s", context.Value, notation.Value)
	}
	if notation.IsCritical {
		t.Fatal("Expected notation to be non critical")
	}
	if !notation.IsHumanReadable {
		t.Fatal("Expected notation to be human readable")
	}
}

func Test_SignDetachedWithCriticalContext(t *testing.T) {
	// given

	context := NewSigningContext(
		"test-context",
		true,
	)
	signer, _ := testPGP.Sign().
		SigningKeys(keyRingTestPrivate).
		SigningContext(context).
		Detached().
		New()
	// when
	signature, err := signer.Sign([]byte(testMessage), Bytes)
	// then
	if err != nil {
		t.Fatal(err)
	}
	p, err := packet.Read(bytes.NewReader(signature))
	if err != nil {
		t.Fatal(err)
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		t.Fatal("Packet was not a signature")
	}
	notations := sig.Notations
	if len(notations) != 2 {
		t.Fatal("Wrong number of notations")
	}
	notation := notations[0]
	if notation.Name != constants.SignatureContextName {
		t.Fatalf("Expected notation name to be %s, got %s", constants.SignatureContextName, notation.Name)
	}
	if string(notation.Value) != context.Value {
		t.Fatalf("Expected notation value to be %s, got %s", context.Value, notation.Value)
	}
	if !notation.IsCritical {
		t.Fatal("Expected notation to be critical")
	}
	if !notation.IsHumanReadable {
		t.Fatal("Expected notation to be human readable")
	}
}

func Test_VerifyWithUnknownCriticalContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/critical_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}

	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	checkVerificationError(t, result.SignatureError(), constants.SIGNATURE_FAILED)
}

func Test_VerifyWithUnKnownNonCriticalContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/non_critical_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	if err = result.SignatureError(); err != nil {
		t.Fatalf("Expected no verification error, got %v", err)
	}
}

func Test_VerifyWithKnownCriticalContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/critical_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	verificationContext := NewVerificationContext(
		"test-context",
		false,
		0,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	if err = result.SignatureError(); err != nil {
		t.Fatalf("Expected no verification error, got %v", err)
	}
}

func Test_VerifyWithWrongContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/critical_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	verificationContext := NewVerificationContext(
		"another-test-context",
		false,
		0,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	checkVerificationError(t, result.SignatureError(), constants.SIGNATURE_BAD_CONTEXT)
}

func Test_VerifyWithMissingNonRequiredContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/no_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	verificationContext := NewVerificationContext(
		"test-context",
		false,
		0,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	if err = result.SignatureError(); err != nil {
		t.Fatalf("Expected no verification error, got %v", err)
	}
}

func Test_VerifyWithMissingRequiredContext(t *testing.T) {
	// given

	signatureArmored, err := os.ReadFile("testdata/signature/no_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	verificationContext := NewVerificationContext(
		"test-context",
		true,
		0,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	checkVerificationError(t, result.SignatureError(), constants.SIGNATURE_BAD_CONTEXT)
}

func Test_VerifyWithMissingRequiredContextBeforeCutoff(t *testing.T) {
	// given
	signatureArmored, err := os.ReadFile("testdata/signature/no_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	p, err := packet.Read(bytes.NewReader(sig))
	if err != nil {
		t.Fatal(err)
	}
	sigPacket, ok := p.(*packet.Signature)
	if !ok {
		t.Fatal("Packet was not a signature")
	}
	verificationContext := NewVerificationContext(
		"test-context",
		true,
		sigPacket.CreationTime.Unix()+10000,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	if err = result.SignatureError(); err != nil {
		t.Fatalf("Expected no verification error, got %v", err)
	}
}

func Test_VerifyWithMissingRequiredContextAfterCutoff(t *testing.T) {
	// given
	signatureArmored, err := os.ReadFile("testdata/signature/no_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	p, err := packet.Read(bytes.NewReader(sig))
	if err != nil {
		t.Fatal(err)
	}
	sigPacket, ok := p.(*packet.Signature)
	if !ok {
		t.Fatal("Packet was not a signature")
	}
	verificationContext := NewVerificationContext(
		"test-context",
		true,
		sigPacket.CreationTime.Unix()-10000,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	checkVerificationError(t, result.SignatureError(), constants.SIGNATURE_BAD_CONTEXT)
}

func Test_VerifyWithDoubleContext(t *testing.T) {
	// given
	signatureArmored, err := os.ReadFile("testdata/signature/double_critical_context_detached_sig")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := armor.UnarmorBytes(signatureArmored)
	if err != nil {
		t.Fatal(err)
	}
	verificationContext := NewVerificationContext(
		"test-context",
		true,
		0,
	)
	// when
	verifier, _ := testPGP.Verify().
		VerificationKeys(keyRingTestPublic).
		VerificationContext(verificationContext).
		DisableVerifyTimeCheck().
		New()
	result, err := verifier.VerifyDetached([]byte(testMessage), sig, Bytes)
	if err != nil {
		t.Fatal(err)
	}
	// then
	checkVerificationError(t, result.SignatureError(), constants.SIGNATURE_BAD_CONTEXT)
}
