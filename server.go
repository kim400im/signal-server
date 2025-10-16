// go mod init server
// go mod tidy
// 이거 실행하면 go.od, go.sum 파일 생김

package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// 클라이언트들을 관리하기 위한 맵
var clients = make(map[*websocket.Conn]bool)

// 들어오는 메시지를 모든 클라이언트에게 전달하기 위한 채널
var broadcast = make(chan Message)

// HTTP 연결을 WebSocket으로 업그레이드하기 위한 설정
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 모든 Origin 허용 (테스트 목적)
	},
}

// 주고받을 메시지 구조체 정의
type Message struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func main() {
	// "/ws" 엔드포인트로 오는 HTTP 요청을 처리하는 핸들러 등록
	http.HandleFunc("/ws", handleConnections)

	// 메시지 브로드캐스트를 처리하는 고루틴 시작
	go handleMessages()

	log.Println("시그널링 서버 시작 (포트: 8080)")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// 각 클라이언트의 WebSocket 연결을 처리
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// HTTP -> WebSocket으로 업그레이드
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// 새로운 클라이언트를 맵에 등록
	clients[ws] = true
	log.Println("새로운 클라이언트 접속")

	for {
		var msg Message
		// 클라이언트로부터 메시지(JSON)를 읽음
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// 읽은 메시지를 broadcast 채널로 보냄
		broadcast <- msg
	}
}

// broadcast 채널에 들어온 메시지를 모든 클라이언트에게 전송
func handleMessages() {
	for {
		// 채널에서 메시지를 기다림
		msg := <-broadcast

		// 연결된 모든 클라이언트에게 메시지를 전송
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
