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

var clients = make(map[*websocket.Conn]string) // Value를 string(주소)으로 변경
var broadcast = make(chan map[*websocket.Conn]string)

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

	// *** 수정된 부분 시작 ***
	// 클라이언트가 보내주는 UDP 포트 번호를 기다림
	var msg Message
	err = ws.ReadJSON(&msg)
	if err != nil {
		log.Println("클라이언트 UDP 포트 수신 실패:", err)
		return
	}

	// WebSocket 연결에서 클라이언트의 공인 IP 주소를 가져옴
	publicIP := strings.Split(ws.RemoteAddr().String(), ":")[0]
	// 클라이언트가 알려준 UDP 포트와 조합하여 최종 P2P 주소를 완성
	peerAddr := publicIP + ":" + msg.Body

	clients[ws] = peerAddr
	log.Printf("새로운 클라이언트 접속: %s", peerAddr)

	// 현재 접속한 모든 클라이언트 정보를 broadcast 채널로 보냄
	broadcast <- clients
	// *** 수정된 부분 끝 ***

	// 클라이언트 연결이 끊어지면 맵에서 제거하고 다시 브로드캐스트
	for {
		if _, _, err := ws.NextReader(); err != nil {
			delete(clients, ws)
			log.Printf("클라이언트 접속 끊어짐: %s", peerAddr)
			broadcast <- clients
			break
		}
	}
}

func handleMessages() {
	for {
		clientMap := <-broadcast
		// 모든 클라이언트 목록을 각 클라이언트에게 전송
		for client := range clientMap {
			// 주소 목록 생성 (자기 자신 제외)
			var addrs []string
			for otherClient, addr := range clientMap {
				if client != otherClient {
					addrs = append(addrs, addr)
				}
			}
			// 다른 피어의 주소를 JSON 형태로 전송
			err := client.WriteJSON(addrs)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
