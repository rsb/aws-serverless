// Package security is used to create key-pairs and tunnel database connections
// in the local aws env for development
package security

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rsb/failure"
	"github.com/rsb/sls"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	DefaultBitSize = 2048
	EnvSSHAuthSock = "SSH_AUTH_SOCK"
)

type KeyPairClient struct {
	BitSize int
}

func (c *KeyPairClient) LoadExistingKeyPair(filename string, name string) (sls.KeyPair, error) {
	var kp sls.KeyPair
	isPrivFile, err := isFile(filename)
	if err != nil {
		return kp, failure.Wrap(err, "isFile failed")
	}

	if !isPrivFile {
		return kp, failure.NotFound("private key file (%s) does not exist", filename)
	}

	privData, err := os.ReadFile(filename)
	if err != nil {
		return kp, failure.ToSystem(err, "os.ReadFile failed for (%s)", filename)
	}

	privKey, err := ParseRsaPrivateKeyFromPem(privData)
	if err != nil {
		return kp, failure.Wrap(err, "ParseRsaPrivateKeyFromPem failed")
	}

	key, err := PublicKeyToString(&privKey.PublicKey)
	if err != nil {
		return kp, failure.Wrap(err, "PublicKeyToString failed")
	}
	kp = sls.KeyPair{
		Name:      name,
		PublicKey: key,
	}

	return kp, nil
}

func (c *KeyPairClient) GenerateSaveKeyPair(privFile, pubFile, name string) (sls.KeyPair, error) {
	var kp sls.KeyPair
	privKey, pubKey, err := GenerateRsaKeyPair(c.BitSize)
	if err != nil {
		return kp, failure.Wrap(err, "GenerateRsaKeyPair failed")
	}

	privStr := PrivateKeyToString(privKey)

	if err = ioutil.WriteFile(privFile, []byte(privStr), 0600); err != nil {
		return kp, failure.ToSystem(err, "ioutil.WriteFile failed (%s)", privFile)
	}

	pubStr, err := PublicKeyToString(pubKey)
	if err != nil {
		return kp, failure.Wrap(err, "PublicKeyToString failed")
	}

	if err = ioutil.WriteFile(pubFile, []byte(pubStr), 0644); err != nil {
		return kp, failure.ToSystem(err, "ioutil.WriteFile failed (%s)", privFile)
	}

	kp = sls.KeyPair{
		Name:      name,
		PublicKey: pubStr,
	}

	return kp, nil
}

func (c *KeyPairClient) GeneratePrivateKey() (*rsa.PrivateKey, error) {
	if c.BitSize == 0 {
		return nil, failure.System("c.BitSize is not initialized")
	}

	privKey, err := GenerateRsaPrivateKey(c.BitSize)
	if err != nil {
		return nil, failure.Wrap(err, "GenerateRsaPrivateKey failed")
	}

	return privKey, nil
}

func (c *KeyPairClient) KeyPairFromPEM(name string, pem []byte) (sls.KeyPair, error) {
	var kp sls.KeyPair
	key, err := ParseRsaPrivateKeyFromPem(pem)
	if err != nil {
		return kp, failure.Wrap(err, "ParseRsaPublicKeyFromPem failed")
	}

	data, err := c.PublicKeyString(&key.PublicKey)
	if err != nil {
		return kp, failure.Wrap(err, "PublicKeyToString failed")
	}

	kp = sls.KeyPair{
		Name:      name,
		PublicKey: data,
	}

	return kp, nil
}

func (c *KeyPairClient) PrivateKeyString(p *rsa.PrivateKey) string {
	return PrivateKeyToString(p)
}

func (c *KeyPairClient) PublicKeyString(p *rsa.PublicKey) (string, error) {
	data, err := PublicKeyToString(p)
	if err != nil {
		return data, failure.Wrap(err, "PublicKeyToString failed")
	}

	return data, nil
}

func (c *KeyPairClient) PublicKeyFromPEM(pubPEM []byte) (*rsa.PublicKey, error) {
	key, err := ParseRsaPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, failure.Wrap(err, "ParseRsaPublicKeyFromPem failed")
	}

	return key, nil
}

func GenerateRsaKeyPair(size ...int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := GenerateRsaPrivateKey(size...)
	if err != nil {
		return nil, nil, failure.Wrap(err, "GenerateRsaPrivateKey failed")
	}
	return privKey, &privKey.PublicKey, nil
}

func GenerateRsaPrivateKey(size ...int) (*rsa.PrivateKey, error) {
	bitSize := DefaultBitSize
	if len(size) > 0 {
		bitSize = size[0]
	}

	privKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, failure.ToSystem(err, "rsa.GenerateKey failed (bitSize: %+v)", bitSize)
	}

	return privKey, nil
}

func PrivateKeyToString(privKey *rsa.PrivateKey) string {
	data := x509.MarshalPKCS1PrivateKey(privKey)
	key := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: data,
	})

	return string(key)
}

func PublicKeyToString(k *rsa.PublicKey) (string, error) {
	key, err := ssh.NewPublicKey(k)
	if err != nil {
		return "", failure.ToSystem(err, "ssh.NewPublicKey failed")
	}

	data := ssh.MarshalAuthorizedKey(key)

	return string(data), nil
}

// ParseRsaPrivateKeyFromPem converts bytes from a pem file, extracts the
// private key and returns it.
func ParseRsaPrivateKeyFromPem(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, failure.System("pem.Decode failed. invalid PEM block")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, failure.ToSystem(err, "x509.ParsePKCS1PrivateKey failed")
	}

	return priv, nil
}

// ParseRsaPublicKeyFromPEM converts bytes from a pem file, extracts the public key
// and returns it
func ParseRsaPublicKeyFromPEM(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, failure.System("pem.Decode failed. invalid PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, failure.ToSystem(err, "x509.ParsePKIXPublicKey failed")
	}

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		return pub, nil
	default:
		break // fall through
	}

	return nil, failure.System("Key type is not RSA")
}

// MarshalRSAPrivate allows you to convert your private key to the right format to be
// saved to a file or used in other tooling systems.
func MarshalRSAPrivate(p *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(p),
	})
}

// MarshalRSAPublic converts the ssh.PublicKey to bytes. Sometimes you want to
// save the public part in a format readable by OpenSSH to grant access to a
// user. It is usually the format you can find in the ~/.ssh/authorized_keys file.
func MarshalRSAPublic(pub ssh.PublicKey) []byte {
	return bytes.TrimSuffix(ssh.MarshalAuthorizedKey(pub), []byte{'\n'})
}

func UnmarshalRSAPublic(bytes []byte) (ssh.PublicKey, error) {
	var pub ssh.PublicKey
	pub, _, _, _, err := ssh.ParseAuthorizedKey(bytes)
	if err != nil {
		return pub, failure.ToSystem(err, "ssh.ParseAuthorizedKey failed")
	}

	return pub, nil
}

// isFile is a helper utility used to determine if the file exists on the filesystem
func isFile(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, failure.ToSystem(err, "os.Stat failed for (%s)", p)
}

type Endpoint struct {
	User string
	Host string
	Port int
}

func NewEndpoint(s string) (*Endpoint, error) {
	ep := Endpoint{Host: s}

	if parts := strings.Split(ep.Host, "@"); len(parts) > 1 {
		ep.User = parts[0]
		ep.Host = parts[1]
	}

	if parts := strings.Split(ep.Host, ":"); len(parts) > 1 {
		ep.Host = parts[0]
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, failure.ToSystem(err, "strconv.Atoi failed to cast port number (%s, %s)", parts[0], parts[1])
		}
		ep.Port = p
	}

	return &ep, nil
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("%s:%d", e.Host, e.Port)
}

type Printable interface {
	Printf(string, ...interface{})
}

type SSHTunnel struct {
	Local       *Endpoint
	Server      *Endpoint
	Remote      *Endpoint
	Config      *ssh.ClientConfig
	Log         Printable
	Conns       []net.Conn
	ServerConns []*ssh.Client
	isOpen      bool
	close       chan interface{}
}

func NewSSHTunnel(tunnel string, auth ssh.AuthMethod, dest string, localPort string) (*SSHTunnel, error) {
	local, err := NewEndpoint("localhost:" + localPort)
	if err != nil {
		return nil, failure.Wrap(err, "NewEndpoint failed for localhost")
	}

	server, err := NewEndpoint(tunnel)
	if err != nil {
		return nil, failure.Wrap(err, "NewEndpoint failed for bastion server")
	}
	if server.Port == 0 {
		server.Port = 22
	}

	remote, err := NewEndpoint(dest)
	if err != nil {
		return nil, failure.Wrap(err, "NewEndpoint failed for destination")
	}

	st := SSHTunnel{
		Config: &ssh.ClientConfig{
			User: server.User,
			Auth: []ssh.AuthMethod{auth},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				// Always accept key
				return nil
			},
		},
		Local:  local,
		Server: server,
		Remote: remote,
		close:  make(chan interface{}),
	}

	return &st, nil
}

func NewConnectionWaiter(listener net.Listener, c chan net.Conn) error {
	conn, err := listener.Accept()
	if err != nil {
		return failure.ToSystem(err, "listener.Accept failed")
	}
	c <- conn
	return nil
}

func (t *SSHTunnel) Close() {
	t.close <- struct{}{}
}

func (t *SSHTunnel) Logf(fmt string, args ...interface{}) {
	if t.Log == nil {
		return
	}

	t.Log.Printf(fmt, args...)
}

func (t *SSHTunnel) Start() {
	listener, err := net.Listen("tcp", t.Local.String())
	if err != nil {
		t.Logf("net.Listen failed: %v", err)
	}
	t.isOpen = true

	for {
		if !t.isOpen {
			break
		}

		c := make(chan net.Conn)
		go func() {
			if err := NewConnectionWaiter(listener, c); err != nil {
				t.Logf("NewConnectionWaiter failed for (%s)", t.Local)
			}
		}()
		t.Logf("listening for new connections...")

		select {
		case <-t.close:
			t.Logf("close signal received, closing...")
			t.isOpen = false
		case conn := <-c:
			t.Conns = append(t.Conns, conn)
			t.Logf("accepted connection")
			go t.forward(conn)
		}
	}
	var total int

	total = len(t.Conns)
	for i, conn := range t.Conns {
		t.Logf("closing the netConn (%d of %d)", i+1, total)
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close failed: %s", err.Error())
		}
	}

	total = len(t.ServerConns)
	for i, conn := range t.ServerConns {
		t.Logf("closing the serverConn (%d of %d)", i+1, total)
		if err := conn.Close(); err != nil {
			t.Logf("conn.Close failed: %s", err.Error())
		}
	}

	if err := listener.Close(); err != nil {
		t.Logf("listener.Close failed: %v", err)
	}
	t.Logf("tunnel closed")
}

func (t *SSHTunnel) forward(localConn net.Conn) {
	serverConn, err := ssh.Dial("tcp", t.Server.String(), t.Config)
	if err != nil {
		t.Logf("server dial error: %s", err)
		return
	}
	t.Logf("connected to %s (1 of 2)\n", t.Server.String())
	t.ServerConns = append(t.ServerConns, serverConn)

	remoteConn, err := serverConn.Dial("tcp", t.Remote.String())
	if err != nil {
		t.Logf("remote dial error: %s", err)
	}
	t.Logf("connect to %s (1 of 2)", t.Remote.String())

	copyConn := func(writer, reader net.Conn) {
		if _, err := io.Copy(writer, reader); err != nil {
			t.Logf("io.Copy error: %s", err)
		}
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)

	return
}

func AuthMethodFromSSHAgent(sshAuthSock string) (ssh.AuthMethod, error) {
	var method ssh.AuthMethod
	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return method, failure.ToSystem(err, "net.Dial failed for (%s)", sshAuthSock)
	}

	method = ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
	return method, nil
}

func AuthMethodFromPrivateKeyFile(file string) (ssh.AuthMethod, error) {
	var method ssh.AuthMethod
	isPrivFile, err := isFile(file)
	if err != nil {
		return method, failure.Wrap(err, "isFile failed")
	}

	if !isPrivFile {
		return method, failure.NotFound("private key file (%s) does not exist", file)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return method, failure.ToSystem(err, "os.ReadFile failed for (%s)", file)
	}

	key, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return method, failure.ToSystem(err, "ssh.ParsePrivateKey failed (%s)", file)
	}

	method = ssh.PublicKeys(key)
	return method, nil
}
