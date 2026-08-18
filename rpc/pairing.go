// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package rpc

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/devproje/mininaru/util"
	"github.com/google/uuid"
)

type ClientDevice struct {
	Id                string
	Name              string
	Fingerprint       string
	CertificateSerial string
	PairedAt          int64
	LastSeenAt        int64
	RevokedAt         int64
}

type PairingRequest struct {
	Id             string
	Code           string
	Name           string
	Fingerprint    string
	PublicKey      []byte
	CertificatePEM []byte
	CreatedAt      int64
	ExpiresAt      int64
	Status         string
}

const pairingLifetime = 5 * time.Minute

const pairingRetention = 24 * time.Hour

const (
	pairingWaiting  = "waiting"
	pairingApproved = "approved"
	pairingDenied   = "denied"
	pairingExpired  = "expired"
)

const maxDeviceNameBytes = 80

const maxPendingPairings = 128

func pairingCode() (string, error) {
	var upper *big.Int
	var value *big.Int

	var err error

	upper = big.NewInt(1000000)
	value, err = rand.Int(rand.Reader, upper)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", value.Int64()), nil
}

func pairingPublicKey(encoded []byte) (ed25519.PublicKey, string, error) {
	var parsed any
	var publicKey ed25519.PublicKey
	var fingerprint string
	var ok bool

	var err error

	parsed, err = x509.ParsePKIXPublicKey(encoded)
	if err != nil {
		return nil, "", fmt.Errorf("parse client public key: %w", err)
	}

	publicKey, ok = parsed.(ed25519.PublicKey)
	if !ok {
		return nil, "", fmt.Errorf("client key is not ed25519")
	}

	fingerprint, err = publicKeyFingerprint(publicKey)
	if err != nil {
		return nil, "", err
	}

	return publicKey, fingerprint, nil
}

func pairingInsert(name, fingerprint string, publicKey []byte, now time.Time) (*PairingRequest, error) {
	var request PairingRequest
	var attempts int

	var err error

	request = PairingRequest{
		Id:          uuid.NewString(),
		Name:        name,
		Fingerprint: fingerprint,
		PublicKey:   publicKey,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(pairingLifetime).Unix(),
		Status:      pairingWaiting,
	}

	for attempts = 0; attempts < 10; attempts++ {
		request.Code, err = pairingCode()
		if err != nil {
			return nil, err
		}

		_, err = util.DB.Exec(`INSERT INTO rpc_pairings
			(id, code, name, fingerprint, public_key, created_at, expires_at, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
			request.Id, request.Code, request.Name, request.Fingerprint, request.PublicKey,
			request.CreatedAt, request.ExpiresAt, request.Status)
		if err == nil {
			return &request, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, err
		}
	}

	return nil, fmt.Errorf("could not allocate pairing code")
}

func PairingBegin(name string, encodedPublicKey []byte) (*PairingRequest, error) {
	var fingerprint string
	var now time.Time
	var pending int

	var err error

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("device name is required")
	}
	if len(name) > maxDeviceNameBytes {
		return nil, fmt.Errorf("device name exceeds %d bytes", maxDeviceNameBytes)
	}

	_, fingerprint, err = pairingPublicKey(encodedPublicKey)
	if err != nil {
		return nil, err
	}

	now = time.Now()
	_, err = util.DB.Exec("UPDATE rpc_pairings SET status = ? WHERE status = ? AND expires_at <= ?;", pairingExpired, pairingWaiting, now.Unix())
	if err != nil {
		return nil, err
	}
	_, err = util.DB.Exec("DELETE FROM rpc_pairings WHERE status != ? AND expires_at <= ?;", pairingWaiting, now.Add(-pairingRetention).Unix())
	if err != nil {
		return nil, err
	}
	err = util.DB.QueryRow("SELECT COUNT(*) FROM rpc_pairings WHERE status = ?;", pairingWaiting).Scan(&pending)
	if err != nil {
		return nil, err
	}
	if pending >= maxPendingPairings {
		return nil, fmt.Errorf("too many pairing requests are waiting")
	}

	return pairingInsert(name, fingerprint, append([]byte(nil), encodedPublicKey...), now)
}

func pairingScan(row *sql.Row) (*PairingRequest, error) {
	var request PairingRequest

	var err error

	err = row.Scan(&request.Id, &request.Code, &request.Name, &request.Fingerprint, &request.PublicKey,
		&request.CertificatePEM, &request.CreatedAt, &request.ExpiresAt, &request.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("pairing request not found")
		}

		return nil, err
	}

	if request.Status == pairingWaiting && request.ExpiresAt <= time.Now().Unix() {
		request.Status = pairingExpired
		_, err = util.DB.Exec("UPDATE rpc_pairings SET status = ? WHERE id = ? AND status = ?;", pairingExpired, request.Id, pairingWaiting)
		if err != nil {
			return nil, err
		}
	}

	return &request, nil
}

func PairingGet(id string) (*PairingRequest, error) {
	return pairingScan(util.DB.QueryRow(`SELECT id, code, name, fingerprint, public_key, certificate_pem,
		created_at, expires_at, status FROM rpc_pairings WHERE id = ?;`, id))
}

func PairingPending() ([]*PairingRequest, error) {
	var rows *sql.Rows
	var request PairingRequest
	var requests []*PairingRequest
	var now int64

	var err error

	now = time.Now().Unix()
	_, err = util.DB.Exec("UPDATE rpc_pairings SET status = ? WHERE status = ? AND expires_at <= ?;", pairingExpired, pairingWaiting, now)
	if err != nil {
		return nil, err
	}

	rows, err = util.DB.Query(`SELECT id, code, name, fingerprint, public_key, certificate_pem,
		created_at, expires_at, status FROM rpc_pairings WHERE status = ? ORDER BY created_at ASC;`, pairingWaiting)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&request.Id, &request.Code, &request.Name, &request.Fingerprint, &request.PublicKey,
			&request.CertificatePEM, &request.CreatedAt, &request.ExpiresAt, &request.Status)
		if err != nil {
			return nil, err
		}

		requests = append(requests, &PairingRequest{Id: request.Id, Code: request.Code, Name: request.Name,
			Fingerprint: request.Fingerprint, PublicKey: request.PublicKey, CreatedAt: request.CreatedAt,
			ExpiresAt: request.ExpiresAt, Status: request.Status})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return requests, nil
}

func PairingApprove(code string) (*ClientDevice, error) {
	var request *PairingRequest
	var certificate []byte
	var serial string
	var client ClientDevice
	var now int64
	var tx *sql.Tx

	var err error

	request, err = pairingScan(util.DB.QueryRow(`SELECT id, code, name, fingerprint, public_key, certificate_pem,
		created_at, expires_at, status FROM rpc_pairings WHERE code = ?;`, code))
	if err != nil {
		return nil, err
	}
	if request.Status != pairingWaiting {
		return nil, fmt.Errorf("pairing request is %s", request.Status)
	}

	client = ClientDevice{Id: uuid.NewString(), Name: request.Name, Fingerprint: request.Fingerprint}
	certificate, serial, err = IssueClientCertificate(request.PublicKey, client.Id)
	if err != nil {
		return nil, err
	}

	now = time.Now().Unix()
	client.CertificateSerial = serial
	client.PairedAt = now

	tx, err = util.DB.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`INSERT INTO rpc_clients
		(id, name, fingerprint, public_key, certificate_serial, paired_at, last_seen_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
		ON CONFLICT(fingerprint) DO UPDATE SET id = excluded.id, name = excluded.name, public_key = excluded.public_key,
			certificate_serial = excluded.certificate_serial, paired_at = excluded.paired_at, revoked_at = 0;`,
		client.Id, client.Name, client.Fingerprint, request.PublicKey, client.CertificateSerial, client.PairedAt)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	_, err = tx.Exec("UPDATE rpc_pairings SET status = ?, certificate_pem = ? WHERE id = ? AND status = ?;",
		pairingApproved, certificate, request.Id, pairingWaiting)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func PairingDeny(code string) error {
	var result sql.Result
	var affected int64

	var err error

	result, err = util.DB.Exec("UPDATE rpc_pairings SET status = ? WHERE code = ? AND status = ?;", pairingDenied, code, pairingWaiting)
	if err != nil {
		return err
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("waiting pairing request not found")
	}

	return nil
}

func ClientList() ([]*ClientDevice, error) {
	var rows *sql.Rows
	var client ClientDevice
	var clients []*ClientDevice

	var err error

	rows, err = util.DB.Query(`SELECT id, name, fingerprint, certificate_serial, paired_at, last_seen_at, revoked_at
		FROM rpc_clients ORDER BY paired_at ASC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&client.Id, &client.Name, &client.Fingerprint, &client.CertificateSerial,
			&client.PairedAt, &client.LastSeenAt, &client.RevokedAt)
		if err != nil {
			return nil, err
		}

		clients = append(clients, &ClientDevice{Id: client.Id, Name: client.Name, Fingerprint: client.Fingerprint,
			CertificateSerial: client.CertificateSerial, PairedAt: client.PairedAt,
			LastSeenAt: client.LastSeenAt, RevokedAt: client.RevokedAt})
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return clients, nil
}

func ClientRevoke(identifier string) error {
	var result sql.Result
	var affected int64

	var err error

	result, err = util.DB.Exec("UPDATE rpc_clients SET revoked_at = ? WHERE id = ? OR fingerprint = ?;", time.Now().Unix(), identifier, identifier)
	if err != nil {
		return err
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

func ClientAuthenticate(certificate *x509.Certificate) (*ClientDevice, error) {
	var fingerprint string
	var client ClientDevice

	var err error

	fingerprint, err = certificateFingerprint(certificate)
	if err != nil {
		return nil, err
	}

	err = util.DB.QueryRow(`SELECT id, name, fingerprint, certificate_serial, paired_at, last_seen_at, revoked_at
		FROM rpc_clients WHERE fingerprint = ?;`, fingerprint).Scan(&client.Id, &client.Name, &client.Fingerprint,
		&client.CertificateSerial, &client.PairedAt, &client.LastSeenAt, &client.RevokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("client is not paired")
		}

		return nil, err
	}
	if client.RevokedAt != 0 {
		return nil, fmt.Errorf("client is revoked")
	}
	if certificate.SerialNumber.String() != client.CertificateSerial {
		return nil, fmt.Errorf("client certificate has been replaced")
	}

	client.LastSeenAt = time.Now().Unix()
	_, err = util.DB.Exec("UPDATE rpc_clients SET last_seen_at = ? WHERE id = ?;", client.LastSeenAt, client.Id)
	if err != nil {
		return nil, err
	}

	return &client, nil
}
