package client

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

func RunAsClient(port string, domain string, verbose bool) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:8888")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}

	// conf := &tls.Config{
	// 	InsecureSkipVerify: true, // Set this to false in production!
	// }

	// conn, err := tls.Dial("tcp", "app.passthru.fun:443", conf)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	defer conn.Close()

	// will request the domain here
	_, err = conn.Write([]byte("domain " + domain))
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Println("connection ended ", err)
		} else {
			log.Fatal(err)
		}
	}

	response := make([]byte, 10024)
	n, err := conn.Read(response)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Println("Connection rejected ", err)
		} else {
			log.Fatal(err)
		}
	}

	// log.Println("Response: ", string(response))
	// log.Println("Byte representation of response: ", []byte(string(response)))
	if string(response[:n]) == "false" {
		log.Println("domain not available")
		return
	} else if string(response[:n]) == "true" {
		log.Println("\033[1m\033[31mlocalhost:" + port + "\033[0m <===> \033[1m\033[32m" + domain + ".passthru.fun\033[0m\033[0m")
	}

	buf := make([]byte, 100024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("Server disconnected 2", err)
				break
			} else {
				log.Fatal(err)
			}
		}
		if verbose {
			log.Println(string(buf[:n]))
		}

		// #############################################################
		// new approach: Instead of making tcp connections with local server
		// we will make http requests to the local server

		// converting the buffer to a http request
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf[:n])))
		if err != nil {
			log.Println("Error reading request: ", err)
			if errors.Is(err, io.EOF) {
				log.Println("Server disconnected ", err)
				break
			}
		}

		// Check if the request is a WebSocket request
		if req.Header.Get("Upgrade") == "websocket" {
			log.Println("WebSocket request received, rejecting")
			// Create a response string indicating that WebSocket upgrades are not supported
			response := fmt.Sprintf("HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n%s", "WebSocket upgrade not supported")

			// Write the response string to the TCP connection
			_, err := conn.Write([]byte(response))
			if err != nil {
				log.Println("Error writing response to connection: ", err)
			}
			continue

		}

		// changing the URL to the local server
		req.URL.Scheme = "http"
		req.URL.Host = "localhost:" + port

		// clearing the requestURI field
		req.RequestURI = ""

		// writing the request to the local server
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Error making request to local server: ", err)
		}
		defer resp.Body.Close()

		// Write the status line
		statusLine := fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status)

		// Format the headers
		headers := new(bytes.Buffer)
		err = resp.Header.Write(headers)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Response headers ", headers.String())

		// Read the body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		// Combine the status line, headers, and body into one string
		response := statusLine + headers.String() + "\r\n" + string(body)

		// Write the response to the client
		_, err = io.WriteString(conn, response)
		if err != nil {
			log.Fatal(err)
		}

	}
}
