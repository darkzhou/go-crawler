package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/lizongying/go-crawler/static"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	rootCert     *x509.Certificate
	rootKey      *rsa.PrivateKey
	privateKey   *rsa.PrivateKey
	listener     *Listener
	srv          *http.Server
	serialNumber int64
	replace      bool
	proxy        *url.URL
}

func (p *Proxy) getCertificate(domain string) (cert *tls.Certificate, err error) {
	atomic.AddInt64(&p.serialNumber, 1)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(p.serialNumber),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, p.rootCert, &p.privateKey.PublicKey, p.rootKey)
	if err != nil {
		return
	}

	cert = &tls.Certificate{
		PrivateKey:  p.privateKey,
		Certificate: [][]byte{certBytes},
	}
	return
}

func (p *Proxy) doReplace(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>%s</title>
</head>
<body>
  %s
</body>
</html>
`, r.Host, r.URL.String())))

}

func (p *Proxy) doRequest(w http.ResponseWriter, r *http.Request) {
	if p.proxy != nil {
		r.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.proxy.String())))
	}

	if r.URL.Host == "" {
		r.URL.Host = r.Host
	}
	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}

	fmt.Println(strings.Repeat("#", 100))
	fmt.Println("Request:")
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println("Error dumping request:", err)
		return
	}
	fmt.Println(string(bytes.TrimSpace(requestDump)))
	fmt.Println(strings.Repeat("#", 100))
	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	copyHeader(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)

	fmt.Println("Response:")
	responseDump, err := httputil.DumpResponse(response, true)
	if err != nil {
		fmt.Println("Error dumping response:", err)
		return
	}
	fmt.Println(string(bytes.TrimSpace(responseDump)))
	fmt.Println(strings.Repeat("#", 100))

	_, _ = io.Copy(w, response.Body)
}

func (p *Proxy) handleHttps(w http.ResponseWriter, _ *http.Request) {
	client, server := net.Pipe()
	defer func() {
		_ = client.Close()
	}()

	p.listener.AddConn(server)
	_, _ = w.Write([]byte("HTTP/1.1 200 Connection Established\n\n"))

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	hijack, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	defer func() {
		_ = hijack.Close()
	}()

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		defer g.Done()
		transfer(client, hijack)
	}()
	go func() {
		defer g.Done()
		transfer(hijack, client)
	}()
	g.Wait()
}

func (p *Proxy) start() error {
	return p.srv.ServeTLS(p.listener, "", "")
}

func (p *Proxy) close() (err error) {
	err = p.srv.Close()
	if err != nil {
		return
	}
	err = p.listener.Close()
	if err != nil {
		return
	}
	return
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleHttps(w, r)
	} else {
		if p.replace {
			p.doReplace(w, r)
		} else {
			p.doRequest(w, r)
		}
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer func() {
		_ = destination.Close()
	}()
	defer func() {
		_ = source.Close()
	}()
	_, _ = io.Copy(destination, source)
}

func NewProxy(proxy string, replace bool) (p *Proxy, err error) {
	p = new(Proxy)
	if P, err := url.Parse(proxy); err != nil {
		p.proxy = P
	}
	p.replace = replace

	// ca.cert
	block, _ := pem.Decode(static.CaCert)
	if block == nil {
		return
	}
	p.rootCert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return
	}

	// ca.key
	block, _ = pem.Decode(static.CaKey)
	if block == nil {
		return
	}
	if err != nil {
		return
	}
	p.rootKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return
	}

	// server.key
	p.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}

	p.listener, _ = NewListener()
	p.srv = &http.Server{
		Handler: p,
		TLSConfig: &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return p.getCertificate(info.ServerName)
			},
		},
	}
	go func() {
		_ = p.start()
	}()
	return
}
