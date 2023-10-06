package jwkutil

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

const symmetricKeyLength = 2048

func NewKeyPair(keyID string, alg jwa.SignatureAlgorithm) (jwk.Key, jwk.Set, error) {
	switch alg {
	case jwa.HS256, jwa.HS384, jwa.HS512:
		key := make([]byte, symmetricKeyLength)
		_, err := rand.Read(key)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate symmetric key: reading from crypto/rand: %w", err)
		}

		return newSymmetricKeyPair(keyID, key, alg)

	case jwa.ES256, jwa.ES384, jwa.ES512:
		// There's a helper function for this in jwx, jws.CurveForAlgorithm, but it requires a bunch of type asserting back and forth
		// Not really worth the trouble for a single switch statement
		var crv elliptic.Curve
		switch alg {
		case jwa.ES256:
			crv = elliptic.P256()
		case jwa.ES384:
			crv = elliptic.P384()
		case jwa.ES512:
			crv = elliptic.P521()
		default:
			panic("unreachable")
		}

		return newECKeyPair(keyID, alg, crv)

	case jwa.RS256, jwa.RS384, jwa.RS512:
		return newRSAKeyPair(keyID, alg)

	case jwa.EdDSA:
		return newEdwardsKeyPair(keyID, alg)

	default:
		return nil, nil, fmt.Errorf("unsupported algorithm: %s", alg)
	}
}

func newSymmetricKeyPair(id string, key []byte, alg jwa.SignatureAlgorithm) (jwk.Key, jwk.Set, error) {
	skey, err := jwk.FromRaw(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create symmetric key: %s", err)
	}

	setAll(skey, map[string]interface{}{
		jwk.AlgorithmKey: alg,
		jwk.KeyIDKey:     id,
	})

	set := jwk.NewSet()
	if err := set.AddKey(skey); err != nil {
		return nil, nil, fmt.Errorf("failed to add key to set: %s", err)
	}

	return skey, set, err
}

func newRSAKeyPair(id string, alg jwa.SignatureAlgorithm) (jwk.Key, jwk.Set, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA private key: %s", err)
	}

	return newKeyPair(id, alg, priv)
}

func newECKeyPair(id string, alg jwa.SignatureAlgorithm, crv elliptic.Curve) (jwk.Key, jwk.Set, error) {

	priv, err := ecdsa.GenerateKey(crv, rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate EC private key: %s", err)
	}

	return newKeyPair(id, alg, priv)
}

func newEdwardsKeyPair(id string, alg jwa.SignatureAlgorithm) (jwk.Key, jwk.Set, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Edwards private key: %s", err)
	}

	return newKeyPair(id, alg, priv)
}

func newKeyPair(id string, alg jwa.SignatureAlgorithm, privKey any) (jwk.Key, jwk.Set, error) {
	privJWK, err := jwk.FromRaw(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("jwk.FromRaw(%v) error = %v", privKey, err)
	}

	setAll(privJWK, map[string]interface{}{
		jwk.AlgorithmKey: alg,
		jwk.KeyIDKey:     id,
		jwk.KeyUsageKey:  jwk.ForSignature,
	})

	pubJWK, err := jwk.PublicKeyOf(privJWK)
	if err != nil {
		return nil, nil, fmt.Errorf("jwk.PublicKeyOf(%v) error = %v", privJWK, err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(pubJWK); err != nil {
		return nil, nil, fmt.Errorf("failed to add key to set: %s", err)
	}

	return privJWK, set, nil
}

func setAll(key jwk.Key, values map[string]interface{}) error {
	for k, v := range values {
		if err := key.Set(k, v); err != nil {
			return fmt.Errorf("failed to set %s: %s", k, err)
		}
	}

	return nil
}
