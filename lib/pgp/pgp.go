// Pitchfork's PGP functions
package pfpgp

import (
	"bytes"
	"crypto"
	"errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/crypto/openpgp/s2k"
	"strconv"
	"strings"
	"time"
)

// PubKey returns the public key from a entity.
func PubKey(ent *openpgp.Entity) (pubkey string, err error) {
	buf := new(bytes.Buffer)
	armor, err := armor.Encode(buf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", err
	}

	ent.Serialize(armor)
	armor.Close()

	pubkey = buf.String()

	return
}

// PubKey returns the private key from a entity.
func SecKey(ent *openpgp.Entity, cfg *packet.Config) (seckey string, err error) {
	buf := new(bytes.Buffer)
	armor, err := armor.Encode(buf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", err
	}

	ent.SerializePrivate(armor, cfg)
	armor.Close()

	seckey = buf.String()

	return
}

// CreateKey creates a new PGP key.
func CreateKey(email string, name string, descr string) (seckey string, pubkey string, err error) {
	var cfg *packet.Config = nil

	/* TODO nil == config, configure it ;) */
	ent, err := openpgp.NewEntity(name, descr, email, cfg)
	if err != nil {
		return
	}

	/* Figure out the IDs of the Hashes we prefer */
	hashes := []crypto.Hash{
		crypto.SHA512,
		crypto.SHA384,
		crypto.SHA256,
		crypto.SHA224,
	}

	phashes := []uint8{}
	for _, h := range hashes {
		id, ok := s2k.HashToHashId(h)
		if !ok {
			err = errors.New("Unsupported Hash")
			return
		}
		phashes = append(phashes, id)
	}

	/* Self sign the identities with our preferences */
	for _, id := range ent.Identities {
		id.SelfSignature.PreferredSymmetric = []uint8{
			uint8(packet.CipherAES256),
			uint8(packet.CipherAES192),
			uint8(packet.CipherAES128),
			uint8(packet.CipherCAST5),
			uint8(packet.Cipher3DES),
		}

		id.SelfSignature.PreferredHash = phashes

		id.SelfSignature.PreferredCompression = []uint8{
			uint8(packet.CompressionZLIB),
			uint8(packet.CompressionZIP),
		}

		err = id.SelfSignature.SignUserId(id.UserId.Id, ent.PrimaryKey, ent.PrivateKey, cfg)
		if err != nil {
			return
		}
	}

	/* Self-sign the Subkeys */
	for _, subkey := range ent.Subkeys {
		err = subkey.Sig.SignKey(subkey.PublicKey, ent.PrivateKey, cfg)
		if err != nil {
			return
		}
	}

	/* Get the Public Armored key */
	pubkey, err = PubKey(ent)
	if err != nil {
		return
	}

	/* Get the Secret Armored key */
	seckey, err = SecKey(ent, cfg)
	if err != nil {
		return
	}

	return
}

// GetKeyInfo retrieves information about a key.
func GetKeyInfo(keyring string, email string) (key_id string, key_exp time.Time, err error) {
	/* Parse the Keyring */
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(keyring))
	if err != nil {
		return
	}

	/* Find the right entity */
	for _, e := range entities {
		for _, i := range e.Identities {
			/* Correct identity? */
			if i.UserId.Email == email {
				/* Format the Key ID */
				key_id = strings.ToUpper(strconv.FormatUint(e.PrimaryKey.KeyId, 16))

				/* TODO Get real expiration time, if any */
				key_exp = time.Unix(0, 0)
				return
			}
		}
	}

	err = errors.New("Key for " + email + " not found")

	return
}
