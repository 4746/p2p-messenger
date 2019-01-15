package proto

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/crypto/ed25519"
	"log"
	"net"
	"os"
	"reflect"
)

//Proto Ядро протокола
type Proto struct {
	Name    string
	Peers   *Peers
	PubKey  ed25519.PublicKey
	privKey ed25519.PrivateKey
	Broker  chan *Envelope
}

func (p Proto) String() string {
	return "proto: " + hex.EncodeToString(p.PubKey) + ": " + p.Name
}

func getSeed() []byte {
	seed := getRandomSeed(32)

	fName := "seed.dat"
	file, err := os.Open(fName)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(fName)
			if err != nil {
				panic(err)
			}
		}
	}

	_, err = file.Read(seed)
	if err != nil {
		panic(err)
	}
	return seed
}

//NewProto - создание экземпляра ядра протокола
func NewProto(name string) *Proto {
	//privateKey := ed25519.NewKeyFromSeed(getSeed())
	publicKey, privateKey := LoadKey(name)
	return &Proto{
		Name:    name,
		Peers:   NewPeers(),
		PubKey:  publicKey,
		privKey: privateKey,
		Broker:  make(chan *Envelope),
	}
}

//SendName Отправка своего имени в сокет
func (p Proto) SendName(peer *Peer) {

	exchPubKey, exchPrivKey := CreateKeyExchangePair()

	handShake, err := json.Marshal(HandShake{
		Name:   p.Name,
		PubKey: hex.EncodeToString(p.PubKey),
		ExKey:  hex.EncodeToString(exchPubKey[:]),
	})

	peer.SharedKey.Update(nil, exchPrivKey[:])

	if err != nil {
		panic(err)
	}
	sign := ed25519.Sign(p.privKey, handShake)

	envelope := NewSignedEnvelope("HAND", p.PubKey[:], make([]byte, 32), sign, handShake)

	envelope.Send(peer)
}

//RequestPeers Запрос списка пиров
func (p Proto) RequestPeers(peer *Peer) {
	envelope := NewEnvelope("LIST", []byte("TODO"))
	envelope.Send(peer)
}

//SendPeers Отправка списка пиров
func (p Proto) SendPeers(peer *Peer) {
	envelope := NewEnvelope("PEER", []byte("TODO"))
	envelope.Send(peer)
}

//SendMessage Отправка сообщения
func (p Proto) SendMessage(peer *Peer, msg string) {
	envelope := NewEnvelope("MESS", []byte(msg))
	envelope.Send(peer)
}

//RegisterPeer Регистрация пира в списках пиров
func (p Proto) RegisterPeer(peer *Peer) *Peer {
	// TODO: сравнение через equal
	if reflect.DeepEqual(peer.PubKey, p.PubKey) {
		return nil
	}

	p.Peers.Put(peer)

	log.Printf("Register new peer: %s (%v)", peer.Name, len(p.Peers.peers))

	return peer
}

//UnregisterPeer Удаление пира из списка
func (p Proto) UnregisterPeer(peer *Peer) {
	p.Peers.Remove(peer)
	log.Printf("UnRegister peer: %s", peer.Name)
}

//ListenPeer Старт прослушивания соединения с пиром
func (p Proto) ListenPeer(peer *Peer) {
	readWriter := bufio.NewReadWriter(bufio.NewReader(*peer.Conn), bufio.NewWriter(*peer.Conn))
	p.HandleProto(readWriter, *peer.Conn)
}

//HandleProto Обработка входящих сообщений
func (p Proto) HandleProto(rw *bufio.ReadWriter, conn net.Conn) {
	var peer *Peer
	for {
		envelope, err := ReadEnvelope(rw.Reader)
		if err != nil {
			log.Printf("Error on read Envelope: %v", err)
			break
		}

		if ed25519.Verify(envelope.From, envelope.Content, envelope.Sign) {
			log.Printf("Signed envelope!")
		}

		log.Printf("LISTENER: recieve envelope from %s", conn.RemoteAddr())

		switch string(envelope.Cmd) {
		case "HAND":
			{
				newPeer := NewPeer(conn)

				err := newPeer.UpdatePeer(envelope)
				if err != nil {
					log.Printf("Update peer error: %s", err)
				} else {
					if peer != nil {
						p.UnregisterPeer(peer)
					}
					p.RegisterPeer(newPeer)
					peer = newPeer
				}
				p.SendName(peer)

			}
		case "MESS":
			{
				p.Broker <- envelope
				log.Printf("NEW MESSAGE %s", envelope.Content)
			}
		default:
			log.Printf("PROTO MESSAGE %v %v %v", envelope.Cmd, envelope.Id, envelope.Content)
		}

	}

	if peer != nil {
		p.UnregisterPeer(peer)
	}

}
