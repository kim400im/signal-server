// go mod init server
// go mod tidy
// 이거 실행하면 go.od, go.sum 파일 생김

package main

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// 클라이언트의 주소 정보를 담는 구조체 (기존과 동일)
type ClientInfo struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
	Port      string `json:"port"`
}

// *** 핵심 변경: 방(Room)을 관리하는 새로운 자료구조 ***
// map[방 이름] -> map[클라이언트 연결]클라이언트 정보
var rooms = make(map[string]map[*websocket.Conn]ClientInfo)

// 여러 클라이언트가 동시에 rooms 맵에 접근하는 것을 막기 위한 잠금장치
var roomsMux = &sync.Mutex{}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	log.Println("시그널링 서버 시작 (포트: 8080)")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// *** 핵심 변경: URL에서 방 이름(room name)을 가져옴 ***
	// 예: ws://.../ws?room=my-room-123
	roomName := r.URL.Query().Get("room")
	if roomName == "" {
		log.Println("에러: 방 이름이 지정되지 않았습니다.")
		return // 방 이름이 없으면 연결 거부
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// 함수가 끝나면(클라이언트 접속 종료 시) 반드시 실행됨
	defer func() {
		removeClient(roomName, ws)
		broadcastPeerList(roomName) // 변경된 목록을 해당 방에만 알림
	}()

	var info ClientInfo
	if err := ws.ReadJSON(&info); err != nil {
		log.Println("클라이언트 정보 수신 실패:", err)
		return
	}

	// 서버가 직접 파악한 클라이언트의 공인 IP를 사용 (클라이언트가 보낸 값보다 신뢰성 높음)
	info.PublicIP = strings.Split(ws.RemoteAddr().String(), ":")[0]

	addClient(roomName, ws, info)
	log.Printf("'%s' 방에 새로운 클라이언트 접속: %+v", roomName, info)

	// 새로운 클라이언트가 왔으므로, 해당 방에만 최신 목록 전송
	broadcastPeerList(roomName)

	// 클라이언트가 연결을 유지하도록 메시지를 계속 읽음 (내용은 무시)
	// 에러가 발생하면(연결 끊김) 루프가 종료되고 위의 defer 함수가 실행됨
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

// 클라이언트를 특정 방에 추가하는 함수
func addClient(roomName string, conn *websocket.Conn, info ClientInfo) {
	roomsMux.Lock()         // 다른 작업이 동시에 접근하지 못하도록 잠금
	defer roomsMux.Unlock() // 함수가 끝나면 잠금 해제

	// 해당 이름의 방이 없으면, 새로 생성
	if rooms[roomName] == nil {
		rooms[roomName] = make(map[*websocket.Conn]ClientInfo)
		log.Printf("새로운 방 생성: %s", roomName)
	}
	// 방에 클라이언트 추가
	rooms[roomName][conn] = info
}

// 특정 방에서 클라이언트를 제거하는 함수
func removeClient(roomName string, conn *websocket.Conn) {
	roomsMux.Lock()
	defer roomsMux.Unlock()

	if room, ok := rooms[roomName]; ok {
		log.Printf("'%s' 방에서 클라이언트 접속 끊어짐: %+v", roomName, room[conn])
		delete(room, conn)
		// 방에 아무도 남지 않으면, 방 자체를 삭제
		if len(room) == 0 {
			delete(rooms, roomName)
			log.Printf("방이 비어서 삭제됨: %s", roomName)
		}
	}
}

// *** 핵심 변경: 특정 방에만 참여자 목록을 전송 ***
func broadcastPeerList(roomName string) {
	roomsMux.Lock()
	defer roomsMux.Unlock()

	room, ok := rooms[roomName]
	if !ok {
		return // 방이 이미 삭제되었으면 아무것도 안 함
	}

	// 해당 방에 있는 모든 클라이언트에게 반복
	for client := range room {
		// 보낼 목록 (자기 자신은 제외)
		var peerList []ClientInfo
		for otherClient, info := range room {
			if client != otherClient {
				peerList = append(peerList, info)
			}
		}

		// 해당 클라이언트에게만 다른 피어들의 목록을 전송
		if err := client.WriteJSON(peerList); err != nil {
			log.Printf("브로드캐스트 에러: %v", err)
		}
	}
}
