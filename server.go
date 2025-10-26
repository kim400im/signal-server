// go mod init server
// go mod tidy
// 이거 실행하면 go.od, go.sum 파일 생김

package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// *** 수정: 사설 IP 정보를 포함한 구조체 ***
type ClientAddr struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
	Port      string `json:"port"`
}

var clients = make(map[*websocket.Conn]ClientAddr)
var broadcast = make(chan map[*websocket.Conn]ClientAddr)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message 구조체는 이제 서버에서만 사용
type Message struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	go handleMessages()

	log.Println("시그널링 서버 시작 (포트: 8080)")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// *** 수정: 클라이언트로부터 UDP 정보 수신 ***
	var addrInfo ClientAddr
	err = ws.ReadJSON(&addrInfo)
	if err != nil {
		log.Println("클라이언트 UDP 정보 수신 실패:", err)
		return
	}

	// WebSocket 연결에서 클라이언트의 공인 IP 주소 추출
	publicIP := strings.Split(ws.RemoteAddr().String(), ":")[0]
	addrInfo.PublicIP = publicIP

	clients[ws] = addrInfo
	log.Printf("새로운 클라이언트 접속 - 공인IP: %s, 사설IP: %s, 포트: %s",
		addrInfo.PublicIP, addrInfo.PrivateIP, addrInfo.Port)

	// 현재 접속한 모든 클라이언트 정보를 broadcast 채널로 보냄
	broadcast <- clients
	// *** 수정된 부분 끝 ***

	// 클라이언트 연결이 끊어지면 맵에서 제거하고 다시 브로드캐스트
	for {
		if _, _, err := ws.NextReader(); err != nil {
			delete(clients, ws)
			log.Printf("클라이언트 접속 끊어짐: %s")
			broadcast <- clients
			break
		}
	}
}

func handleMessages() {
	for {
		clientMap := <-broadcast
		// 모든 클라이언트에게 다른 피어의 주소 정보 전송
		for client := range clientMap {
			var peerAddrs []map[string]string
			for otherClient, addr := range clientMap {
				if client != otherClient {
					peerInfo := map[string]string{
						"public_ip":  addr.PublicIP,
						"private_ip": addr.PrivateIP,
						"port":       addr.Port,
					}
					peerAddrs = append(peerAddrs, peerInfo)
				}
			}
			err := client.WriteJSON(peerAddrs)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
