package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/paperview/webtransport-go"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

func main() {
	domain := flag.String("domain", "localhost", "Domain")
	certPath := flag.String("cert-path", "", "Cert path")
	keyPath := flag.String("key-path", "", "Key path")
	flag.Parse()

	fmt.Println("Starting server...")
	go runServer(*certPath, *keyPath)

	// Give the server a moment to start
	time.Sleep(1 * time.Second)

	fmt.Println("Starting client...")
	runClient(*domain)
}

func runServer(certPath, keyPath string) {
	fmt.Println("Server: Setting up...")
	ctx := context.Background()

	wtServer := webtransport.Server{
		H3: http3.Server{
			Addr:       ":8443",
			QUICConfig: &quic.Config{},
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Server: Received a request")
		wtSession, err := wtServer.Upgrade(w, r)
		if err != nil {
			fmt.Printf("Server: Upgrade error: %v\n", err)
			return
		}
		fmt.Println("Server: Upgraded to WebTransport")

		stream, err := wtSession.AcceptStream(ctx)
		if err != nil {
			fmt.Printf("Server: AcceptStream error: %v\n", err)
			return
		}
		fmt.Println("Server: Accepted stream")

		bytes, err := io.ReadAll(stream)
		if err != nil {
			fmt.Printf("Server: ReadAll error: %v\n", err)
			return
		}

		fmt.Println("From client: " + string(bytes))

		_, err = stream.Write(bytes)
		if err != nil {
			fmt.Printf("Server: Write error: %v\n", err)
			return
		}
		fmt.Println("Server: Wrote response")

		err = stream.Close()
		if err != nil {
			fmt.Printf("Server: Close error: %v\n", err)
			return
		}
		fmt.Println("Server: Closed stream")
	})

	fmt.Printf("Server: Listening on port 8443 with cert %s and key %s\n", certPath, keyPath)
	err := wtServer.ListenAndServeTLS(certPath, keyPath)
	checkErr(err)
}

func runClient(domain string) {
	fmt.Println("Client: Setting up...")
	ctx := context.Background()

	var d webtransport.Dialer
	d.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // Skip certificate verification for testing
	}

	url := "https://" + domain + ":8443"
	fmt.Printf("Client: Connecting to %s\n", url)

	_, wtSession, err := d.Dial(ctx, url, nil)
	if err != nil {
		fmt.Printf("Client: Dial error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Client: Connected")

	stream, err := wtSession.OpenStreamSync(ctx)
	if err != nil {
		fmt.Printf("Client: OpenStreamSync error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Client: Opened stream")

	message := "Hi there"
	fmt.Printf("Client: Sending message: %s\n", message)
	_, err = stream.Write([]byte(message))
	if err != nil {
		fmt.Printf("Client: Write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Client: Sent message")

	// Note that stream.Close() only closes the send side. This allows the
	// stream to receive the reply from the server.
	err = stream.Close()
	if err != nil {
		fmt.Printf("Client: Close error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Client: Closed stream (send side)")

	bytes, err := io.ReadAll(stream)
	if err != nil {
		fmt.Printf("Client: ReadAll error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Client: Received response")

	fmt.Println("From server: " + string(bytes))
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
