// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENCE file.

package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

// BenchmarkGenerateKey performs a benchmark for the key generation
// function signature.GenerateKey.
func BenchmarkGenerateKey(b *testing.B) {
	curve := weierstrass.Stark()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := GenerateKey(curve, rand.Reader); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSign performs a benchmark for signing messages using a
// generated private key.
func BenchmarkSign(b *testing.B) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	hashed := []byte("testing")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig, err := SignASN1(rand.Reader, pvt, hashed)
		if err != nil {
			b.Fatal(err)
		}
		// Prevent the compiler from optimizing out the operation.
		hashed[0] = sig[0]
	}
}

// BenchmarkVerify performs a benchmark on verifying signatures on
// signed messages.
func BenchmarkVerify(b *testing.B) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	hashed := []byte("testing")
	r, s, err := Sign(rand.Reader, pvt, hashed)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !Verify(&pvt.PublicKey, hashed, r, s) {
			b.Fatal("verify failed")
		}
	}
}

// Example showcases the use of the signature package.
func Example() {
	curve := weierstrass.Stark()
	privateKey, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		panic(err)
	}

	msg := "Hello, World!"
	hash := sha256.Sum256([]byte(msg))

	sig, err := SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		panic(err)
	}
	fmt.Printf("signature: %x\n", sig)

	valid := VerifyASN1(&privateKey.PublicKey, hash[:], sig)
	fmt.Println("signature verified:", valid)
}

// TestASN1 tests the signing and verification process for ASN1 encoded
// messages.
func TestASN1(t *testing.T) {
	curve := weierstrass.Stark()
	privateKey, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	msg := "Hello, World!"
	hash := sha256.Sum256([]byte(msg))

	sig, err := SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		t.Fatal("failed to sign message")
	}

	valid := VerifyASN1(&privateKey.PublicKey, hash[:], sig)
	if !valid {
		t.Error("signature is not valid")
	}
}

// TestKeyGeneration tests the validity of the public keys generated
// from the key generation process.
func TestKeyGeneration(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}
	if !curve.IsOnCurve(pvt.PublicKey.X, pvt.PublicKey.Y) {
		t.Errorf("public key invalid: %s", err)
	}
}

// TestSignAndVerify tests the ability to sign and verify messages.
func TestSignAndVerify(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	hashed := []byte("testing")
	r, s, err := Sign(rand.Reader, pvt, hashed)
	if err != nil {
		t.Errorf("error signing: %s", err)
		return
	}

	if !Verify(&pvt.PublicKey, hashed, r, s) {
		t.Error("failed to verify signature")
	}

	hashed[0] ^= 0xff // Scramble message.
	if Verify(&pvt.PublicKey, hashed, r, s) {
		t.Error("Verify always returns true")
	}
}

// TestNonceSafety checks for critical security vulnerabilities around
// how the nonce is used in ECDSA.
func TestNonceSafety(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	hashed := []byte("testing")
	r0, s0, err := Sign(zeroReader, pvt, hashed)
	if err != nil {
		t.Fatalf("error signing: %s", err)
	}

	hashed = []byte("testing...")
	r1, s1, err := Sign(zeroReader, pvt, hashed)
	if err != nil {
		t.Fatalf("error signing: %s", err)
	}

	if s0.Cmp(s1) == 0 {
		t.Error("produced the same signatures on two distinct messages")
	}

	if r0.Cmp(r1) == 0 {
		t.Error("nonce reuse detected")
	}
}

// TestIndcca tests for the IND-CCA (indistinguishability under chosen
// ciphertext attack). See linked [discussion] for details.
//
// [discussion]: https://crypto.stackexchange.com/questions/26689/easy-explanation-of-ind-security-notions
func TestIndcca(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	hashed := []byte("testing")
	r0, s0, err := Sign(rand.Reader, pvt, hashed)
	if err != nil {
		t.Fatalf("error signing: %s", err)
	}

	hashed = []byte("testing...")
	r1, s1, err := Sign(rand.Reader, pvt, hashed)
	if err != nil {
		t.Fatalf("error signing: %s", err)
	}

	if s0.Cmp(s1) == 0 {
		t.Error("produced the same signatures on two distinct messages")
	}

	if r0.Cmp(r1) == 0 {
		t.Error("nonce reuse detected")
	}
}

// TestNegativeInputs checks whether bogus inputs are invalidated i.e. a
// value for r larger than any defined by any of the curves in the
// weierstrass and elliptic packages.
func TestNegativeInputs(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	var hash [32]byte
	r := new(big.Int).SetInt64(1)
	r.Lsh(r, 550 /* larger than any supported curve */)
	r.Neg(r)

	if Verify(&pvt.PublicKey, hash[:], r, r) {
		t.Error("bogus signature accepted")
	}
}

// TestZeroHashSignature tests whether signing and verification can be
// performed on a message that consists of an array of all zero bytes.
func TestZeroHashSignature(t *testing.T) {
	curve := weierstrass.Stark()
	pvt, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}

	hash := make([]byte, 64)
	r, s, err := Sign(rand.Reader, pvt, hash)
	if err != nil {
		t.Fatalf("error signing: %s", err)
	}

	if !Verify(&pvt.PublicKey, hash, r, s) {
		t.Fatalf("failed to verify message with a zeroed byte array for %T", curve)
	}
}

// TestEqual runs a range of equality tests on the curve(s) in this
// package.
func TestEqual(t *testing.T) {
	curve := weierstrass.Stark()
	private, err := GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal("failed to generate key")
	}
	public := &private.PublicKey

	if !public.Equal(public) {
		t.Errorf("public key is not equal to itself: %v", public)
	}
	if !public.Equal(crypto.Signer(private).Public().(*PublicKey)) {
		t.Errorf("private.Public() is not Equal to public: %q", public)
	}
	if !private.Equal(private) {
		t.Errorf("private key is not equal to itself: %v", private)
	}

	// XXX: The following only supports private keys defined in the
	// standard library rsa, ecdsa, and ed25519 packages.
	/*
		enc, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := x509.ParsePKCS8PrivateKey(enc)
		if err != nil {
			t.Fatal(err)
		}
		if !public.Equal(decoded.(crypto.Signer).Public()) {
			t.Errorf("public key is not equal to itself after decoding: %v", public)
		}
		if !private.Equal(decoded) {
			t.Errorf("private key is not equal to itself after decoding: %v", private)
		}
	*/

	other, _ := GenerateKey(curve, rand.Reader)
	if public.Equal(other.Public()) {
		t.Errorf("different public keys are Equal")
	}
	if private.Equal(other) {
		t.Errorf("different private keys are Equal")
	}

	// XXX: Porting the following requires that there be at least two
	// curves specified in this package in order to run a comparison.
	/*
		// Ensure that keys with the same coordinates but on different curves
		// aren't considered Equal.
		differentCurve := &PublicKey{}
		*differentCurve = *public // make a copy of the public key
		if differentCurve.Curve == elliptic.P256() {
			differentCurve.Curve = elliptic.P224()
		} else {
			differentCurve.Curve = elliptic.P256()
		}
		if public.Equal(differentCurve) {
			t.Errorf("public keys with different curves are Equal")
		}
	*/
}
