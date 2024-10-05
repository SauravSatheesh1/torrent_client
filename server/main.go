package main

import (
	"bit_torrent/p2p"
	"bit_torrent/torrent"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var mu sync.Mutex

// Clients map to store all WebSocket connections
var clients = make(map[*websocket.Conn]bool)

// File uploads directory
const uploadsDir = "./uploads"

const outputDir = "./output"

// Handle WebSocket connections and register clients
func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	// Register the client connection
	clients[ws] = true
	log.Println("WebSocket client connected")

	// Ping the client every 30 seconds to keep the connection alive
	go func() {
		for {
			err := ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Println("Ping error:", err)
				delete(clients, ws)
				return
			}
			time.Sleep(30 * time.Second) // Ping every 30 seconds
		}
	}()

	// Wait for the client to disconnect or an error to occur
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			delete(clients, ws) // Remove the client on error
			break
		}
	}
}

// Torrent initialization function
func initializeTorrent(filePath string, torrentMap *torrent.TorrentMap, outputFilePath string, uploadFolderPath string, progressFilePath string) {
	fmt.Printf("Initializing torrent from file: %s\n", filePath)

	// Create a progress channel for the current torrent file
	progressChan := make(chan p2p.ProgressData)

	go func() {
		tf, err := torrent.Open(filePath)
		if err != nil {
			log.Println("Torrent download failed:", err)
			close(progressChan)
			return
		}

		// Start downloading and send progress updates
		err = tf.DownloadToFile(outputFilePath, progressChan, torrentMap, uploadFolderPath, progressFilePath) 
		if err != nil {
			log.Println("Torrent download failed:", err)
			close(progressChan)
			return
		}

		// Close progress channel when done
		close(progressChan)
	}()

	// Listen for progress updates and broadcast to WebSocket clients
	for progress := range progressChan {
		// Include the torrent file name as part of the progress data
		broadcastProgress(progress, filepath.Base(filePath))
	}
}

// Broadcast progress to all connected WebSocket clients, with torrent identifier
func broadcastProgress(progress p2p.ProgressData, torrentFile string) {
	for client := range clients {
		// Include the torrentFile in the progress update
		message := map[string]interface{}{
			"torrentFile": torrentFile,
			"progress":    progress,
		}

		mu.Lock() // Lock the mutex before writing
		err := client.WriteJSON(message)
		mu.Unlock() // Unlock the mutex after writing
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// UploadHandler - handles uploading .torrent files via HTTP POST
func UploadHandler(w http.ResponseWriter, r *http.Request, torrentMap *torrent.TorrentMap) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form data (limit file size to 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form data", http.StatusBadRequest)
		return
	}

	// Retrieve the uploaded file
	file, handler, err := r.FormFile("torrentFile")
	if err != nil {
		http.Error(w, "Unable to retrieve file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileNameWithoutExt := strings.TrimSuffix(handler.Filename, filepath.Ext(handler.Filename))

	uploadFolderPath := filepath.Join(uploadsDir, fileNameWithoutExt)

	// Create the uploads directory if it doesn't exist
	err = os.MkdirAll(uploadFolderPath, os.ModePerm)
	if err != nil {
		http.Error(w, "Unable to create folder for the uploaded file", http.StatusInternalServerError)
		return
	}
	// Save the uploaded file to the created folder
	filePath := filepath.Join(uploadFolderPath, handler.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Unable to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}

	progressInfoFilePath := filepath.Join(uploadFolderPath, "torrent_progress_info.json")

	outputFilePath := filepath.Join(outputDir, fileNameWithoutExt)

	// Start torrent download in the background
	go initializeTorrent(filePath, torrentMap, outputFilePath, uploadFolderPath, progressInfoFilePath)

	fmt.Fprintf(w, "File uploaded successfully: %s\n", handler.Filename)
}

// DownloadHandler - handles the torrent download and triggers WebSocket for progress
func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Get the file path of the uploaded torrent from query params or request body
	filePath := r.URL.Query().Get("filepath")
	fmt.Println(filePath)
	if filePath == "" {
		http.Error(w, "Filepath is required", http.StatusBadRequest)
		return
	}

	// Start the torrent download
	// go initializeTorrent(filePath,)

	fmt.Fprintf(w, "Download started for torrent file: %s", filePath)
}

// Handler to pause the torrent download
func PauseDownloadHandler(w http.ResponseWriter, r *http.Request, torrentMap *torrent.TorrentMap) {
	fmt.Println("Reacheddddddddddd")
	filePath := r.URL.Query().Get("filepath")
	if filePath == "" {
		http.Error(w, "Filepath is required", http.StatusBadRequest)
		return
	}

	valid := torrent.PauseTorrentDownload(filePath, torrentMap, w)
	if !valid {
		return
	}

	fmt.Fprintf(w, "Torrent paused: %s", filePath)
}

func ResumeDownloadHandler(w http.ResponseWriter, r *http.Request, torrentMap *torrent.TorrentMap) {
	fmt.Println("Reacheddddddddddd")
	f := r.URL.Query().Get("filepath")
	if f == "" {
		http.Error(w, "Filepath is required", http.StatusBadRequest)
		return
	}
	outputFilePath := filepath.Join(outputDir, f)
	uploadFolderPath := filepath.Join(uploadsDir, f)
	uploadFilePath := filepath.Join(uploadFolderPath, f+".torrent")
	progressInfoFilePath := filepath.Join(uploadFolderPath, "torrent_progress_info.json")

	initializeTorrent(uploadFilePath, torrentMap, outputFilePath, uploadFolderPath, progressInfoFilePath)

	fmt.Fprintf(w, "Torrent resumed: %s", f)
}

func GetAllActiveTorrents(w http.ResponseWriter, r *http.Request, torrentMap *torrent.TorrentMap) {

	type ActiveTorrents struct {
		Pieces      map[int]bool `json:"pieces"` // Map of index to download status
		TotalPieces int          `json:"total_pieces"`
	}
	fmt.Println("ReachedddddddddddActive")
	files, err := os.ReadDir(uploadsDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read uploads folder: %v", err), http.StatusInternalServerError)
		return
	}
	progress := make(map[string]p2p.ProgressData)
	for _, file := range files {
		path := filepath.Join(uploadsDir, file.Name(), "torrent_progress_info.json")
		progressFile, err := os.Open(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("error opening file: %v", err), http.StatusInternalServerError)
		}
		data, err := io.ReadAll(progressFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("error reading file: %v", err), http.StatusInternalServerError)
			return
		}

		// Unmarshal JSON data into the struct
		var status ActiveTorrents

		err = json.Unmarshal(data, &status)
		if err != nil {
			http.Error(w, fmt.Sprintf("error unmarshaling json: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Println(status.TotalPieces)
		totalProgressPercent := float64(len(status.Pieces)) / float64(status.TotalPieces) * 100
		progress[file.Name()] = p2p.ProgressData{Progress: float64(totalProgressPercent), Name: file.Name(), Speed: 0, RemainingTime: 0, Paused: true}
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(progress); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// Response structure for the API
type SizeResponse struct {
	Directory string `json:"directory"`
	TotalSize int64  `json:"total_size"`
	Message   string `json:"message,omitempty"`
}

// calculateDirectorySize walks through the directory and calculates total size of files
func calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Add file size (ignore directories)
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize, nil
}

// Handler for API that returns total size of files in a directory
func GetTotalDownloadedFilesSize(w http.ResponseWriter, r *http.Request) {
	// Calculate directory size
	totalSize, err := calculateDirectorySize(outputDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to calculate directory size: %v", err), http.StatusInternalServerError)
		return
	}

	// Create the response
	response := SizeResponse{
		Directory: outputDir,
		TotalSize: totalSize,
	}

	// Set header and encode response to JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	torrentMap := torrent.NewTorrentMap()
	r := mux.NewRouter()

	// Ensure uploads and output directories are created
	if err := os.MkdirAll(uploadsDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Define the routes
	r.HandleFunc("/download", DownloadHandler).Methods("GET")
	r.HandleFunc("/progress", wsHandler)

	// Wrap the handlers to pass the `torrentMap`
	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		UploadHandler(w, r, torrentMap)
	}).Methods("POST")

	r.HandleFunc("/pause", func(w http.ResponseWriter, r *http.Request) {
		PauseDownloadHandler(w, r, torrentMap)
	}).Methods("POST")

	r.HandleFunc("/resume", func(w http.ResponseWriter, r *http.Request) {
		ResumeDownloadHandler(w, r, torrentMap)
	}).Methods("POST")

	r.HandleFunc("/active-torrents", func(w http.ResponseWriter, r *http.Request) {
		GetAllActiveTorrents(w, r, torrentMap)
	}).Methods("GET")

	r.HandleFunc("/total-downloaded", func(w http.ResponseWriter, r *http.Request) {
		GetTotalDownloadedFilesSize(w, r)
	}).Methods("GET")

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // Frontend origin
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type"},
	})

	handler := corsHandler.Handler(r)

	// Start the server
	log.Println("Server starting at :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal("Server failed:", err)
	}
}
