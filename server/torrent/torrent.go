package torrent

import (
	"bit_torrent/p2p"
	"bit_torrent/peers"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackpal/bencode-go"
)

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type FileDetails struct {
	Length int      `bencode:"length"` // Length of each file in bytes
	Path   []string `bencode:"path"`   // Subdirectory names ending with the actual file name
}

type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

const Port uint16 = 6881

const udpPort uint16 = 6882

// TorrentMap manages a thread-safe map of torrents
type TorrentMap struct {
	sync.Mutex
	m map[string]*p2p.Torrent
}

// NewTorrentMap initializes and returns a new TorrentMap
func NewTorrentMap() *TorrentMap {
	return &TorrentMap{
		m: make(map[string]*p2p.Torrent),
	}
}

func Open(path string) (TorrentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return TorrentFile{}, err
	}
	bto := bencodeTorrent{}
	err1 := bencode.Unmarshal(file, &bto)
	if err1 != nil {
		return TorrentFile{}, err
	}
	return bto.toTorrentFile()
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	hashLen := 20
	buf := []byte(i.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("received malformed pieces of length %d", len(buf))
		return nil, err
	}
	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}
	return hashes, nil
}

func (bto *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	infoHash, err := bto.Info.hash()
	if err != nil {
		return TorrentFile{}, err
	}
	pieceHashes, err := bto.Info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, err
	}

	t := TorrentFile{
		Announce:    bto.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}

	return t, nil
}

func (t *TorrentFile) buildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

func (t *TorrentFile) requestPeers(peerID [20]byte, port uint16) ([]peers.Peer, error) {
	url, err := t.buildTrackerURL(peerID, port)
	fmt.Println(url)
	if err != nil {
		return nil, err
	}

	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var trackerResp bencodeTrackerResp
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	fmt.Println(resp.Body)
	if err != nil {
		return nil, err
	}
	return peers.Unmarshal([]byte(trackerResp.Peers))
}

func loadOrCreateDownloadStatus(filePath string) (p2p.DownloadStatus, error) {
	var status p2p.DownloadStatus
	status.Pieces = make(map[int]bool)

	// Check if the file exists
	if _, err := os.Stat(filePath); err == nil {
		// File exists, load the status
		file, err := os.Open(filePath)
		if err != nil {
			return status, fmt.Errorf("failed to open download status file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&status); err != nil {
			return status, fmt.Errorf("failed to decode download status: %v", err)
		}
	} else if os.IsNotExist(err) {
		// File does not exist, return empty status
		return status, nil
	} else {
		return status, fmt.Errorf("error checking download status file: %v", err)
	}

	return status, nil
}

func (t *TorrentFile) DownloadToFile(path string, progressChan chan<- p2p.ProgressData, torrentMap *TorrentMap, uploadFolderPath string, progressFilePath string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}
	p := []peers.Peer{}
	if strings.HasPrefix(t.Announce, "udp") {
		GetPeers(*t, func(peers []peers.Peer) {
			p = peers
		})

	} else {
		p, err = t.requestPeers(peerID, Port)
		if err != nil {
			return err
		}
	}

	// Load or create the download status map
	status, err := loadOrCreateDownloadStatus(progressFilePath)
	if err != nil {
		return err
	}

	torrent := p2p.Torrent{
		Peers:       p,
		PeerID:      peerID,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		Name:        t.Name,
		Status:      status.Pieces,
	}
	torrentMap.Lock()
	torrentMap.m[t.Name] = &torrent
	torrentMap.Unlock()

	_, err = torrent.Download(progressChan, path, progressFilePath)
	if err != nil {
		return err
	}

	return nil

}

func PauseTorrentDownload(filePath string, torrentMap *TorrentMap, w http.ResponseWriter) bool {
	torrentMap.Lock()
	torrent, ok := torrentMap.m[filePath]
	torrentMap.Unlock()
	fmt.Println(torrent)
	if !ok {
		http.Error(w, "Torrent not found", http.StatusNotFound)
		return false
	}

	// Assuming the Torrent struct has a method to pause
	torrent.Pause()

	return true
}

func SaveToJSONFile(torrent *p2p.Torrent, filename string) error {
	// Convert the struct to JSON
	data, err := json.MarshalIndent(torrent, "", "  ") // MarshalIndent for pretty printing
	if err != nil {
		return fmt.Errorf("failed to marshal struct to JSON: %v", err)
	}

	// Create or open the file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Write the JSON data to the file
	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON to file: %v", err)
	}

	return nil
}

// GetPeers initiates the connection to the UDP tracker and retrieves peers.
func GetPeers(torrent TorrentFile, callback func([]peers.Peer)) {
	// Parse the announce URL
	announceUrl, err := url.Parse(torrent.Announce)
	if err != nil {
		log.Fatalf("Failed to parse URL: %v", err)
	}

	// Create a UDP connection
	conn, err := net.Dial("udp", announceUrl.Host)
	if err != nil {
		log.Fatalf("Failed to dial UDP connection: %v", err)
	}
	defer conn.Close()

	// Step 1: Send connect request
	connReq := buildConnReq()
	udpSend(conn, connReq)

	// Buffer to read UDP responses
	resp := make([]byte, 1024)
	for {
		// Step 2: Receive and parse connect response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(resp)
		if err != nil {
			log.Fatalf("Failed to read UDP response: %v", err)
		}

		if respType(resp[:n]) == "connect" {
			connResp := parseConnResp(resp[:n])

			// Step 3: Send announce request
			announceReq := buildAnnounceReq(connResp.ConnectionID, torrent, udpPort)
			udpSend(conn, announceReq)
		} else if respType(resp[:n]) == "announce" {
			// Step 4: Parse announce response
			announceResp := parseAnnounceResp(resp[:n])
			fmt.Println(announceResp.Peers)

			// Step 5: Return peers to callback
			callback(announceResp.Peers)
			break
		}
	}
}

// // udpSend sends the message to the UDP tracker.
func udpSend(conn net.Conn, message []byte) {
	_, err := conn.Write(message)
	if err != nil {
		log.Fatalf("Failed to send UDP message: %v", err)
	}
}

// respType determines the type of response (connect or announce).
func respType(resp []byte) string {
	action := binary.BigEndian.Uint32(resp[:4])
	if action == 0 {
		return "connect"
	}
	if action == 1 {
		return "announce"
	}
	return ""
}

// // buildConnReq builds a connect request to send to the tracker.
func buildConnReq() []byte {
	buf := new(bytes.Buffer)

	// Connection ID
	binary.Write(buf, binary.BigEndian, uint64(0x41727101980))
	// Action (connect = 0)
	binary.Write(buf, binary.BigEndian, uint32(0))
	// Transaction ID
	transactionID := make([]byte, 4)
	rand.Read(transactionID)
	buf.Write(transactionID)

	return buf.Bytes()
}

// // parseConnResp parses the connect response from the tracker.
func parseConnResp(resp []byte) (connResp ConnResp) {
	connResp.Action = binary.BigEndian.Uint32(resp[:4])
	connResp.TransactionID = binary.BigEndian.Uint32(resp[4:8])
	connResp.ConnectionID = resp[8:16]
	return
}

// // buildAnnounceReq builds an announce request for peers.
func buildAnnounceReq(connID []byte, torrent TorrentFile, port uint16) []byte {
	fmt.Println(torrent.Length)
	fmt.Println(torrent.PieceLength)
	buf := new(bytes.Buffer)

	// Connection ID
	buf.Write(connID)
	// Action (announce = 1)
	binary.Write(buf, binary.BigEndian, uint32(1))
	// Transaction ID
	transactionID := make([]byte, 4)
	rand.Read(transactionID)
	buf.Write(transactionID)
	// Info hash
	buf.Write(torrent.InfoHash[:])
	// Peer ID
	peerID := generatePeerID()
	buf.Write(peerID)
	// Downloaded
	binary.Write(buf, binary.BigEndian, uint64(0))
	// Left (torrent size)
	binary.Write(buf, binary.BigEndian, uint64(torrent.Length))
	// Uploaded
	binary.Write(buf, binary.BigEndian, uint64(0))
	// Event (none = 0)
	binary.Write(buf, binary.BigEndian, uint32(0))
	// IP address (default = 0)
	binary.Write(buf, binary.BigEndian, uint32(0))
	// Key
	key := make([]byte, 4)
	rand.Read(key)
	buf.Write(key)
	// Num want (-1 means default)
	binary.Write(buf, binary.BigEndian, int32(-1))
	// Port
	binary.Write(buf, binary.BigEndian, port)

	return buf.Bytes()
}

// // parseAnnounceResp parses the announce response from the tracker.
func parseAnnounceResp(resp []byte) AnnounceResp {
	var announceResp AnnounceResp
	announceResp.Action = binary.BigEndian.Uint32(resp[:4])
	announceResp.TransactionID = binary.BigEndian.Uint32(resp[4:8])
	announceResp.Leechers = binary.BigEndian.Uint32(resp[8:12])
	announceResp.Seeders = binary.BigEndian.Uint32(resp[12:16])

	// Parse peers
	peerBytes := resp[20:]
	for i := 0; i < len(peerBytes); i += 6 {
		ip := net.IP(peerBytes[i : i+4])
		port := binary.BigEndian.Uint16(peerBytes[i+4 : i+6])
		announceResp.Peers = append(announceResp.Peers, peers.Peer{IP: ip, Port: port})
	}

	return announceResp
}

// // generatePeerID generates a random peer ID.
func generatePeerID() []byte {
	peerID := make([]byte, 20)
	copy(peerID[:], "-GO0001-")
	rand.Read(peerID[8:])
	return peerID
}

// // TorrentMeta and Peer are placeholder structures for torrent metadata and peers.
type TorrentMeta struct {
	Announce string
	InfoHash [20]byte
	Size     uint64
}

// // ConnResp represents a UDP connect response.
type ConnResp struct {
	Action        uint32
	TransactionID uint32
	ConnectionID  []byte
}

// // AnnounceResp represents a UDP announce response.
type AnnounceResp struct {
	Action        uint32
	TransactionID uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []peers.Peer
}
