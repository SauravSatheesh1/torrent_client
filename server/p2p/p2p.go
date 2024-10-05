package p2p

import (
	"bit_torrent/client"
	"bit_torrent/message"
	"bit_torrent/peers"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

// MaxBlockSize is the largest number of bytes a request can ask for
const MaxBlockSize = 16384

// MaxBacklog is the number of unfulfilled requests a client can have in its pipeline
const MaxBacklog = 5

type Torrent struct {
	Peers       []peers.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
	Status      map[int]bool
	Paused      bool // Tracks if paused
	PauseChan   chan struct{}
}

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (t *Torrent) Pause() {
	t.Paused = true
	close(t.PauseChan) // Signal to pause all download workers
}

func (t *Torrent) calculateBoundsForPiece(index int) (begin int, end int) {
	begin = index * t.PieceLength
	end = begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}
	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	if msg == nil {
		return nil
	}
	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

func attemptDownloadPiece(c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{}) // Disable the deadline

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				// Last block might be shorter than the typical block
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}
				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}
		err := state.readMessage()
		if err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.hash[:]) {
		return fmt.Errorf("Index %d failed integrity check", pw.index)
	}
	return nil
}

func (t *Torrent) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, result chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		fmt.Println(err.Error())
		log.Printf("Could not handshake with %s. Disconnecting\n", peer.IP)
		return
	}
	defer c.Conn.Close()
	log.Printf("Completed handshake with %s\n", peer.IP)

	c.SendUnChoke()
	c.SendInterested()

	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}

		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw // Put piece back on the queue
			return
		}

		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("Piece #%d failed integrity check\n", pw.index)
			workQueue <- pw // Put piece back on the queue
			continue
		}

		c.SendHave(pw.index)
		result <- &pieceResult{pw.index, buf}

	}
}

type ProgressData struct {
	Name          string  `json:"name"`
	Progress      float64 `json:"progress"`
	Speed         float64 `json:"speed"` // Download speed in KB/s
	RemainingTime float64 `json:"remaining_time"`
	Paused        bool    `json:"paused"`
}

func (t *Torrent) Download(progressChan chan<- ProgressData, outputPath string, progressFilePath string) ([]byte, error) {
	log.Println("Starting download for", t.Name)

	// Open the file for writing (or create if it doesn't exist)
	existingIndex := []int{}
	for index, _ := range t.PieceHashes {
		if _, ok := t.Status[index]; ok {
			existingIndex = append(existingIndex, index)
		}

	}
	if len(existingIndex) == len(t.PieceHashes) {
		progressChan <- ProgressData{
			Name:          t.Name,
			Progress:      100,
			Speed:         0,
			RemainingTime: 0,
			Paused:        true,
		}
		return []byte{}, errors.New("file Already Downloaded")
	}
	outFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return []byte{}, err
	}

	defer outFile.Close()
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult)
	t.PauseChan = make(chan struct{})
	for index, hash := range t.PieceHashes {
		if !t.Status[index] {
			length := t.calculatePieceSize(index)
			workQueue <- &pieceWork{index, hash, length}
		}
	}

	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}

	buf := make([]byte, t.Length)
	donePieces := len(existingIndex)
	totalPieces := len(t.PieceHashes)
	startTime := time.Now()
	totalDownloaded := 0
	for donePieces < len(t.PieceHashes) {
		select {
		// Pause download when signal received
		case <-t.PauseChan:
			log.Println("Download paused.")
			progressChan <- ProgressData{
				Name:          t.Name,
				Progress:      0,
				Speed:         0,
				RemainingTime: 0,
				Paused:        true,
			}
			return nil, errors.New("download paused")
		// Process download results
		case res := <-results:
			begin, _ := t.calculateBoundsForPiece(res.index)

			// Write downloaded piece to file
			_, err := outFile.WriteAt(res.buf, int64(begin))
			if err != nil {
				log.Printf("Error writing piece #%d to file: %v", res.index, err)
				return nil, err
			}

			// Mark piece as downloaded and update status map
			donePieces++
			totalDownloaded += len(res.buf)
			t.Status[res.index] = true

			// Save the download status map
			var status DownloadStatus
			status.Pieces = t.Status
			status.TotalPieces = totalPieces
			if err := saveDownloadStatus(progressFilePath, status); err != nil {
				log.Printf("Error saving download map: %v", err)
				return nil, err
			}

			// Report download progress
			percent := float64(donePieces) / float64(totalPieces) * 100
			elapsedTime := time.Since(startTime).Seconds()
			speed := float64(totalDownloaded) / 1024.0 / elapsedTime
			remainingBytes := t.Length - totalDownloaded
			remainingTime := float64(remainingBytes) / 1024.0 / speed

			numWorkers := runtime.NumGoroutine() - 1 // subtract 1 for main thread
			log.Printf("(%0.2f%%) Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
			progressChan <- ProgressData{
				Name:          t.Name,
				Progress:      percent,
				Speed:         speed,
				RemainingTime: remainingTime,
				Paused:        false,
			}
		}
	}

	close(workQueue)
	return buf, nil
}

type DownloadStatus struct {
	Pieces      map[int]bool `json:"pieces"` // Map of index to download status
	TotalPieces int          `json:"total_pieces"`
}

func saveDownloadStatus(filePath string, status DownloadStatus) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create download status file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(status); err != nil {
		return fmt.Errorf("failed to encode download status: %v", err)
	}

	return nil
}
