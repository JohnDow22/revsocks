package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/kost/revsocks/internal/common"
)

// genPair генерирует пару CA сертификата и ключа + пользовательский сертификат
func genPair(keysize int) (cacert []byte, cakey []byte, cert []byte, certkey []byte) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	ca := &x509.Certificate{
		SerialNumber: common.RandBigInt(serialNumberLimit),
		Subject: pkix.Name{
			Country:            []string{common.RandString(16)},
			Organization:       []string{common.RandString(16)},
			OrganizationalUnit: []string{common.RandString(16)},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          common.RandBytes(5),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	priv, _ := rsa.GenerateKey(rand.Reader, keysize)
	pub := &priv.PublicKey
	caBin, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		log.Println("create ca failed", err)
		return
	}

	cert2 := &x509.Certificate{
		SerialNumber: common.RandBigInt(serialNumberLimit),
		Subject: pkix.Name{
			Country:            []string{common.RandString(16)},
			Organization:       []string{common.RandString(16)},
			OrganizationalUnit: []string{common.RandString(16)},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: common.RandBytes(6),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	priv2, _ := rsa.GenerateKey(rand.Reader, keysize)
	pub2 := &priv2.PublicKey
	cert2Bin, err2 := x509.CreateCertificate(rand.Reader, cert2, ca, pub2, priv)
	if err2 != nil {
		log.Println("create cert2 failed", err2)
		return
	}

	privBin := x509.MarshalPKCS1PrivateKey(priv)
	priv2Bin := x509.MarshalPKCS1PrivateKey(priv2)

	return caBin, privBin, cert2Bin, priv2Bin
}

// GetPEMs конвертирует сертификат и ключ в PEM формат
func GetPEMs(cert []byte, key []byte) (pemcert []byte, pemkey []byte) {
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})

	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: key,
	})

	return certPem, keyPem
}

// GetTLSPair создаёт tls.Certificate из PEM данных
func GetTLSPair(certPem []byte, keyPem []byte) (tls.Certificate, error) {
	tlspair, errt := tls.X509KeyPair(certPem, keyPem)
	if errt != nil {
		return tlspair, errt
	}
	return tlspair, nil
}

// GetRandomTLS генерирует случайный TLS сертификат
func GetRandomTLS(keysize int) (tls.Certificate, error) {
	_, _, cert, certkey := genPair(keysize)
	certPem, keyPem := GetPEMs(cert, certkey)
	tlspair, err := GetTLSPair(certPem, keyPem)
	return tlspair, err
}

// ========================================
// Lazy TLS: Кеширование сертификата
// ========================================

// tlsCacheDir возвращает путь к директории кеша сертификатов
func tlsCacheDir() string {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dirname, ".revsocks-tls-cache")
}

// GetCachedTLS загружает сертификат из кеша или генерирует новый
// Кеш сохраняется в ~/.revsocks-tls-cache/
// Ускоряет повторные запуски сервера (RSA 2048 генерация ~100-500ms)
func GetCachedTLS(keysize int) (tls.Certificate, error) {
	cacheDir := tlsCacheDir()
	if cacheDir == "" {
		// Не удалось определить home dir - генерируем без кеша
		log.Println("Cannot determine home directory, generating TLS without cache")
		return GetRandomTLS(keysize)
	}

	certFile := filepath.Join(cacheDir, "server.crt")
	keyFile := filepath.Join(cacheDir, "server.key")

	// Пробуем загрузить из кеша
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		log.Printf("Loaded cached TLS certificate from %s", cacheDir)
		return cert, nil
	}

	// Кеша нет - генерируем новый сертификат
	log.Println("Generating new TLS certificate...")
	_, _, certBytes, keyBytes := genPair(keysize)
	certPem, keyPem := GetPEMs(certBytes, keyBytes)

	// Сохраняем в кеш
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		log.Printf("Cannot create TLS cache directory: %v", err)
		// Продолжаем без кеширования
	} else {
		if err := os.WriteFile(certFile, certPem, 0600); err != nil {
			log.Printf("Cannot write cert to cache: %v", err)
		}
		if err := os.WriteFile(keyFile, keyPem, 0600); err != nil {
			log.Printf("Cannot write key to cache: %v", err)
		} else {
			log.Printf("TLS certificate cached to %s", cacheDir)
		}
	}

	return GetTLSPair(certPem, keyPem)
}
