package network

import (
	"context"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"snake_game/protobuf"
	"strconv"
	"sync"
	"time"
)

const (
	MulticastAddress = "239.192.0.4"
	MulticastPort    = 9192

	AnnouncementDelay    = 1000
	AnnouncementWaitTime = 5000
)

type Announcement struct {
	announcement *protobuf.GameAnnouncement
	serverAddr   net.UDPAddr
	lastReceived time.Time
}

func (a *Announcement) Announce() *protobuf.GameAnnouncement {
	return a.announcement
}

func (a *Announcement) ServerAddr() net.UDPAddr {
	return a.serverAddr
}

func (a *Announcement) LastReceived() time.Time {
	return a.lastReceived
}

func ListenForAnnouncements(ctx context.Context, announcements *[]*Announcement, lock *sync.Mutex) {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(MulticastAddress, strconv.Itoa(MulticastPort)))
	if err != nil {
		log.Fatalf("Ошибка при резолвинге адреса: %v", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Fatalf("Ошибка при создании мультикаст-соединения: %v", err)
	}
	defer conn.Close()

	conn.SetReadBuffer(4 * 1024)
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("Ошибка при чтении пакета: %v", err)
				continue
			}

			message := &protobuf.GameMessage{}
			if err := proto.Unmarshal(buf[:n], message); err != nil {
				log.Printf("Ошибка при декодировании протобаф-сообщения: %v", err)
				continue
			}

			announcementMsg := message.GetAnnouncement()
			if announcementMsg == nil {
				log.Println("[client] message doesnt contains announcement")
				return
			}

			anns := announcementMsg.Games
			if len(anns) == 0 {
				log.Println("Список игр пуст")
				return
			}

			announcement := anns[0]

			log.Printf("[Announcements] Got announs from %s: Game name: %s, Number of players: %d, CanJoin: %v",
				src.String(),
				announcement.GetGameName(),
				len(announcement.GetPlayers().GetPlayers()),
				announcement.GetCanJoin(),
			)

			newAnn := &Announcement{
				announcement: announcement,
				serverAddr:   *src,
			}
			updateAnnouncements(announcements, lock, newAnn)
		}
	}
}

func updateAnnouncements(announcements *[]*Announcement, lock *sync.Mutex, newAnnouncement *Announcement) {
	lock.Lock()
	defer lock.Unlock()

	now := time.Now()

	var filtered []*Announcement
	for _, ann := range *announcements {
		if now.Sub(ann.lastReceived) <= AnnouncementWaitTime*time.Millisecond {
			filtered = append(filtered, ann)
		}
	}
	*announcements = filtered

	if newAnnouncement == nil {
		return
	}

	isExists := false
	for _, ann := range *announcements {
		if ann.serverAddr.String() == newAnnouncement.serverAddr.String() &&
			ann.announcement.GetGameName() == newAnnouncement.announcement.GetGameName() {
			ann.lastReceived = time.Now()
			isExists = true
			break
		}
	}

	if !isExists {
		newAnnouncement.lastReceived = time.Now()
		*announcements = append(*announcements, newAnnouncement)

		log.Printf("[Announcements] Updated: %d games", len(*announcements))
	}
}
